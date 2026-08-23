package boleto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/modulo"
)

// --- duplo do binding -------------------------------------------------------

type libFake struct {
	acbr.BoletoServico // nil: método não sobrescrito estoura, o que é bom num teste

	iniPDF, iniRemessa, cfgRetorno, arquivoRetorno string
	numArquivo                                     int
	online                                         acbr.BoletoOnline
	tenant                                         acbr.TenantConfig

	resPDF, resRemessa, resRetorno, resOnline acbr.Result
	err                                       error
}

func (f *libFake) GerarPDF(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniPDF, f.tenant = ini, t
	return f.resPDF, f.err
}

func (f *libFake) GerarRemessa(t acbr.TenantConfig, ini string, num int) (acbr.Result, error) {
	f.iniRemessa, f.numArquivo, f.tenant = ini, num, t
	return f.resRemessa, f.err
}

func (f *libFake) LerRetorno(t acbr.TenantConfig, cfg, conteudo string) (acbr.Result, error) {
	f.cfgRetorno, f.arquivoRetorno, f.tenant = cfg, conteudo, t
	return f.resRetorno, f.err
}

func (f *libFake) Registrar(t acbr.TenantConfig, op acbr.BoletoOnline) (acbr.Result, error) {
	f.online, f.tenant = op, t
	return f.resOnline, f.err
}

// --- apoio ------------------------------------------------------------------

func muxDe(f *libFake) *http.ServeMux {
	mux := http.NewServeMux()
	NovoModulo(f).Registrar(rotasEm{mux})
	return mux
}

type rotasEm struct{ mux *http.ServeMux }

func (r rotasEm) Handle(p string, h http.Handler)         { r.mux.Handle(comPrefixo(p), h) }
func (r rotasEm) HandleFunc(p string, h http.HandlerFunc) { r.mux.HandleFunc(comPrefixo(p), h) }

func comPrefixo(padrao string) string {
	metodo, caminho, _ := strings.Cut(padrao, " ")
	return metodo + " /boletos" + caminho
}

var _ modulo.Modulo = (*Modulo)(nil)

func post(t *testing.T, mux *http.ServeMux, caminho string, corpo any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(corpo)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, caminho, strings.NewReader(string(b))))
	return rec
}

func contaMinima() map[string]any {
	return map[string]any{
		"banco": "237", "tipoCobranca": 5, "CNAB": "1",
		"agencia": "1234", "conta": "67890",
		"nome": "ACME LTDA", "CNPJCPF": "12.345.678/0001-90",
	}
}

func tituloMinimo() map[string]any {
	return map[string]any{
		"numeroDocumento": "1", "valorDocumento": 150.5, "vencimento": "2026-09-10",
		"sacado": map[string]any{"nome": "FULANO", "CNPJCPF": "11122233344"},
	}
}

func pedidoMinimo() map[string]any {
	return map[string]any{"conta": contaMinima(), "titulos": []any{tituloMinimo()}}
}

// --- PDF --------------------------------------------------------------------

func TestPDFDevolveBase64(t *testing.T) {
	f := &libFake{resPDF: acbr.Result{PDF: []byte("%PDF-1.4 boleto")}}
	rec := post(t, muxDe(f), "/boletos/pdf", pedidoMinimo())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var corpo struct {
		PDF string `json:"pdf_b64"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &corpo)
	bruto, err := base64.StdEncoding.DecodeString(corpo.PDF)
	if err != nil || !bytes.HasPrefix(bruto, []byte("%PDF")) {
		t.Errorf("pdf_b64 não decodifica num PDF: %v", err)
	}
	// O boleto não é fiscal: nada de certificado A1 nem ambiente na sessão.
	if f.tenant.PFXBase64 != "" || len(f.tenant.Config) != 0 {
		t.Errorf("tenant do boleto não deveria ter certificado nem config fiscal: %+v", f.tenant)
	}
	if f.tenant.CNPJ != "12345678000190" {
		t.Errorf("CNPJ do cedente = %q, quero só os dígitos", f.tenant.CNPJ)
	}
	for _, quero := range []string{"[Banco]", "[Conta]", "[Cedente]", "[Titulo1]"} {
		if !strings.Contains(f.iniPDF, quero) {
			t.Errorf("INI não tem %q:\n%s", quero, f.iniPDF)
		}
	}
}

func TestPDFSemPDFEh422(t *testing.T) {
	f := &libFake{resPDF: acbr.Result{Resposta: "erro qualquer"}}
	rec := post(t, muxDe(f), "/boletos/pdf", pedidoMinimo())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
}

func TestExigeBancoETitulos(t *testing.T) {
	casos := map[string]map[string]any{
		"sem banco":   {"conta": map[string]any{"CNPJCPF": "1"}, "titulos": []any{tituloMinimo()}},
		"sem títulos": {"conta": contaMinima(), "titulos": []any{}},
	}
	for nome, corpo := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{}
			rec := post(t, muxDe(f), "/boletos/pdf", corpo)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.iniPDF != "" {
				t.Error("chamou a lib com pedido incompleto")
			}
		})
	}
}

// --- remessa ----------------------------------------------------------------

func TestRemessaUsaSequencialUmPorPadrao(t *testing.T) {
	f := &libFake{resRemessa: acbr.Result{Resposta: "0REMESSA..."}}
	rec := post(t, muxDe(f), "/boletos/remessa", pedidoMinimo())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.numArquivo != 1 {
		t.Errorf("numeroArquivo = %d, quero 1", f.numArquivo)
	}
	var corpo struct {
		Remessa string `json:"remessa"`
		Numero  int    `json:"numero_arquivo"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &corpo)
	if corpo.Remessa == "" || corpo.Numero != 1 {
		t.Errorf("corpo = %+v", corpo)
	}
	// JSON, não anexo: uma resposta por formato em toda a API.
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "json") {
		t.Errorf("Content-Type = %q, quero JSON", ct)
	}
}

func TestRemessaRespeitaOSequencialInformado(t *testing.T) {
	f := &libFake{resRemessa: acbr.Result{Resposta: "0REMESSA..."}}
	p := pedidoMinimo()
	p["numeroArquivo"] = 47
	post(t, muxDe(f), "/boletos/remessa", p)
	if f.numArquivo != 47 {
		t.Errorf("numeroArquivo = %d, quero 47: o sequencial é controle do cliente", f.numArquivo)
	}
}

func TestRemessaVaziaEh422(t *testing.T) {
	f := &libFake{resRemessa: acbr.Result{Resposta: "   "}}
	rec := post(t, muxDe(f), "/boletos/remessa", pedidoMinimo())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
}

// --- retorno ----------------------------------------------------------------

// O INI do retorno leva SÓ a config do banco. Mandar títulos junto confundiria
// a leitura do arquivo.
func TestRetornoMandaSoAConfigDoBanco(t *testing.T) {
	f := &libFake{resRetorno: acbr.Result{Resposta: `{"titulos":[]}`}}
	rec := post(t, muxDe(f), "/boletos/retorno", map[string]any{
		"conta": contaMinima(), "arquivo": "02RETORNO...",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(f.cfgRetorno, "[Titulo") {
		t.Errorf("a config do retorno levou títulos:\n%s", f.cfgRetorno)
	}
	if f.arquivoRetorno != "02RETORNO..." {
		t.Errorf("conteúdo do arquivo não chegou: %q", f.arquivoRetorno)
	}
}

// A lib responde JSON; devolver aninhado poupa o cliente de um segundo parse.
func TestRetornoJSONVaiAninhado(t *testing.T) {
	f := &libFake{resRetorno: acbr.Result{Resposta: `{"CEDENTE":{"Nome":"ACME"}}`}}
	rec := post(t, muxDe(f), "/boletos/retorno", map[string]any{
		"conta": contaMinima(), "arquivo": "x",
	})
	var corpo struct {
		Retorno struct {
			Cedente struct {
				Nome string
			} `json:"CEDENTE"`
		} `json:"retorno"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if corpo.Retorno.Cedente.Nome != "ACME" {
		t.Errorf("retorno não veio aninhado: %s", rec.Body)
	}
}

// Se a lib devolver algo que não é JSON, cair para string é melhor do que
// quebrar o cliente com um 500.
func TestRetornoNaoJSONViraString(t *testing.T) {
	f := &libFake{resRetorno: acbr.Result{Resposta: "erro cru da lib"}}
	rec := post(t, muxDe(f), "/boletos/retorno", map[string]any{
		"conta": contaMinima(), "arquivo": "x",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var corpo struct {
		Retorno string `json:"retorno"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil || corpo.Retorno != "erro cru da lib" {
		t.Errorf("resposta não-JSON não virou string: %s", rec.Body)
	}
}

func TestRetornoExigeBancoEArquivo(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/boletos/retorno", map[string]any{"conta": contaMinima()})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// --- registro online --------------------------------------------------------

func TestRegistroExigeCredenciaisDoBanco(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/boletos/registro", pedidoMinimo())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
	if f.online.INI != "" {
		t.Error("chamou o banco sem credenciais")
	}
}

func TestRegistroPassaOMTLSDecodificado(t *testing.T) {
	f := &libFake{resOnline: acbr.Result{Resposta: `{"ok":true}`}}
	p := pedidoMinimo()
	conta := p["conta"].(map[string]any)
	conta["ws"] = map[string]any{
		"clientID": "id", "clientSecret": "segredo",
		"certCRT": base64.StdEncoding.EncodeToString([]byte("---CRT---")),
		"certKEY": base64.StdEncoding.EncodeToString([]byte("---KEY---")),
	}
	rec := post(t, muxDe(f), "/boletos/registro", p)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if string(f.online.CertCRT) != "---CRT---" || string(f.online.CertKEY) != "---KEY---" {
		t.Errorf("mTLS não chegou decodificado: crt=%q key=%q", f.online.CertCRT, f.online.CertKEY)
	}
}

func TestRegistroComMTLSInvalidoEh400(t *testing.T) {
	f := &libFake{}
	p := pedidoMinimo()
	p["conta"].(map[string]any)["ws"] = map[string]any{"certCRT": "!!!nao-base64"}
	rec := post(t, muxDe(f), "/boletos/registro", p)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
	if f.online.INI != "" {
		t.Error("chamou o banco com certificado ilegível")
	}
}

// Baixa (operação 2) não exige títulos novos; inclusão exige.
func TestBaixaDispensaTitulos(t *testing.T) {
	f := &libFake{resOnline: acbr.Result{Resposta: "ok"}}
	rec := post(t, muxDe(f), "/boletos/registro", map[string]any{
		"conta":    comWS(contaMinima()),
		"titulos":  []any{},
		"operacao": 2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.online.Operacao != 2 {
		t.Errorf("operacao = %d, quero 2", f.online.Operacao)
	}
}

// Recusa do banco é 502 com o detalhe: a falha é do outro lado, e é o detalhe
// que o cliente precisa para agir.
func TestRecusaDoBancoEh502ComDetalhe(t *testing.T) {
	f := &libFake{resOnline: acbr.Result{Codigo: 7, Resposta: "titulo ja registrado"}}
	rec := post(t, muxDe(f), "/boletos/registro", map[string]any{
		"conta": comWS(contaMinima()), "titulos": []any{tituloMinimo()},
	})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, quero 502: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "titulo ja registrado") {
		t.Errorf("o detalhe do banco não chegou: %s", rec.Body)
	}
}

func comWS(c map[string]any) map[string]any {
	c["ws"] = map[string]any{"clientID": "id", "clientSecret": "segredo"}
	return c
}

// --- segredo não vaza -------------------------------------------------------

// ContaWS carrega ClientSecret e o mTLS. Num serviço sem estado, log é a única
// forma realista de um segredo do cliente escapar do processo.
func TestCredenciaisDoBancoSaoRedigidas(t *testing.T) {
	ws := ContaWS{ClientID: "id", ClientSecret: "SEGREDO-MUITO-SECRETO", CertKEY: "CHAVE-PRIVADA"}

	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("teste", "ws", ws)
	if strings.Contains(buf.String(), "SEGREDO") || strings.Contains(buf.String(), "CHAVE-PRIVADA") {
		t.Errorf("segredo vazou no slog: %s", buf.String())
	}
	if s := ws.String(); strings.Contains(s, "SEGREDO") {
		t.Errorf("segredo vazou no String(): %s", s)
	}
}

func TestNomeECapacidades(t *testing.T) {
	m := NovoModulo(&libFake{})
	if m.Nome() != "boletos" {
		t.Errorf("nome = %q", m.Nome())
	}
	caps := strings.Join(m.Capacidades(), ",")
	for _, quero := range []string{"pdf", "remessa", "retorno", "registro"} {
		if !strings.Contains(caps, quero) {
			t.Errorf("capacidade %q não anunciada", quero)
		}
	}
}
