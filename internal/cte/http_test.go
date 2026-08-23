package cte

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/modulo"
)

// --- duplo do binding -------------------------------------------------------

// libFake registra o que cada método recebeu e devolve o que o teste mandar.
type libFake struct {
	acbr.CTeServico // nil: qualquer método não sobrescrito estoura, o que é bom num teste

	iniMontar, iniValidar, iniEvento string
	xmlTransmitido                   string
	tenantMontar, tenantTransmitir   acbr.TenantConfig

	resMontar, resValidar, resTransmitir, resEvento acbr.Result
	errMontar, errValidar, errTransmitir            error
}

func (f *libFake) MontarXML(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniMontar, f.tenantMontar = ini, t
	return f.resMontar, f.errMontar
}

func (f *libFake) ValidarRegras(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniValidar = ini
	return f.resValidar, f.errValidar
}

func (f *libFake) Transmitir(t acbr.TenantConfig, xml string) (acbr.Result, error) {
	f.xmlTransmitido, f.tenantTransmitir = xml, t
	return f.resTransmitir, f.errTransmitir
}

func (f *libFake) Cancelar(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento = ini
	return f.resEvento, nil
}

func (f *libFake) CartaCorrecao(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento = ini
	return f.resEvento, nil
}

func (f *libFake) EnviarEvento(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento = ini
	return f.resEvento, nil
}

// --- apoio ------------------------------------------------------------------

const chaveFixture = "35240812345678000190570010000000011000000010"

// xmlFixture é um CT-e mínimo com o Id e o tpAmb que os extratores procuram.
func xmlFixture(tpAmb string) string {
	return `<?xml version="1.0"?><CTe xmlns="http://www.portalfiscal.inf.br/cte">` +
		`<infCte versao="4.00" Id="CTe` + chaveFixture + `">` +
		`<ide><tpAmb>` + tpAmb + `</tpAmb></ide></infCte></CTe>`
}

// muxDe monta o módulo sob /cte, como o servidor faria.
func muxDe(f *libFake) *http.ServeMux {
	mux := http.NewServeMux()
	NovoModulo(f).Registrar(rotasEm{mux})
	return mux
}

type rotasEm struct{ mux *http.ServeMux }

func (r rotasEm) Handle(p string, h http.Handler) { r.mux.Handle(comPrefixo(p), h) }
func (r rotasEm) HandleFunc(p string, h http.HandlerFunc) {
	r.mux.HandleFunc(comPrefixo(p), h)
}

func comPrefixo(padrao string) string {
	metodo, caminho, _ := strings.Cut(padrao, " ")
	return metodo + " /cte" + caminho
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

func postCru(t *testing.T, mux *http.ServeMux, caminho, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, caminho, strings.NewReader(corpo)))
	return rec
}

func certValido() map[string]string {
	return map[string]string{"pfx_b64": base64.StdEncoding.EncodeToString([]byte("pfx-falso")), "senha": "s3nh4"}
}

// --- gerar -----------------------------------------------------------------

func TestGerarNaoRecebeCertificado(t *testing.T) {
	// É a propriedade que sustenta o desenho: montar e validar não assinam nem
	// falam com a SEFAZ, então o certificado não tem por que existir aqui.
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	rec := post(t, muxDe(f), "/cte/xml", pedidoMinimo())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.tenantMontar.PFXBase64 != "" || f.tenantMontar.SenhaPFX != "" {
		t.Errorf("a geração montou tenant com certificado: %+v", f.tenantMontar)
	}

	var resp RespostaXML
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Chave != chaveFixture {
		t.Errorf("chave = %q, quero %q, sem ela o cliente não recupera transmissão perdida", resp.Chave, chaveFixture)
	}
	if resp.Assinado {
		t.Error("assinado = true; a ACBrLib assina no envio, não na montagem")
	}
	xml, err := base64.StdEncoding.DecodeString(resp.XMLBase64)
	if err != nil || !strings.Contains(string(xml), "infCte") {
		t.Errorf("xml_b64 não devolveu o XML montado: %v", err)
	}
	if !resp.Validacao.OK || !resp.Validacao.Suportada {
		t.Errorf("validação = %+v, quero ok e suportada", resp.Validacao)
	}
}

// A lib devolve SEMPRE código 0 em ValidarRegrasdeNegocios; quem denuncia a
// rejeição é a resposta vir preenchida. Conferir o código daria "válido" para
// qualquer documento: este teste tranca isso.
func TestValidacaoUsaAResposta_NaoOCodigo(t *testing.T) {
	f := &libFake{
		resMontar:  acbr.Result{XML: xmlFixture("2")},
		resValidar: acbr.Result{Codigo: 0, Resposta: "502-Rejeição: Erro na Chave de Acesso\n503-Rejeição: Série fora da faixa\n"},
	}
	rec := post(t, muxDe(f), "/cte/xml", pedidoMinimo())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422 (código 0 com rejeições não é sucesso): %s", rec.Code, rec.Body)
	}
	var env struct {
		Erro struct {
			Codigo   string      `json:"codigo"`
			Detalhes RespostaXML `json:"detalhes"`
		} `json:"erro"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Erro.Codigo != "regras_de_negocio" {
		t.Errorf("código do erro = %q", env.Erro.Codigo)
	}
	if len(env.Erro.Detalhes.Validacao.Mensagens) != 2 {
		t.Errorf("mensagens = %v, quero as 2 rejeições", env.Erro.Detalhes.Validacao.Mensagens)
	}
	// O XML acompanha a rejeição: é com ele que o cliente vê o que a lib entendeu.
	if env.Erro.Detalhes.XMLBase64 == "" {
		t.Error("a rejeição deveria devolver o XML montado junto")
	}
}

func TestValidacaoNaoSuportadaNaoFingeSucesso(t *testing.T) {
	f := &libFake{
		resMontar:  acbr.Result{XML: xmlFixture("2")},
		errValidar: acbr.ErrNaoSuportado,
	}
	rec := post(t, muxDe(f), "/cte/xml", pedidoMinimo())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var resp RespostaXML
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Validacao.Suportada {
		t.Error("suportada = true, mas a lib recusou a operação")
	}
}

func TestGerarExigeCNPJDoEmitente(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/cte/xml", map[string]any{"ambiente": "homologacao"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// Campo com nome errado que o servidor ignora vira documento transmitido com
// dado faltando: só descoberto depois de a SEFAZ autorizar.
func TestCampoDesconhecidoEhRecusado(t *testing.T) {
	rec := postCru(t, muxDe(&libFake{}), "/cte/xml", `{"ambienteX":"homologacao"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ambienteX") {
		t.Errorf("a mensagem deveria nomear o campo errado: %s", rec.Body)
	}
}

// --- transmitir -----------------------------------------------------------------

func TestTransmiteOXMLGerado(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[CTe]\ncStat=100\nxMotivo=Autorizado o uso do CT-e\nchCTe=" + chaveFixture + "\nnProt=135240000000001\n",
		XML:      "<cteProc/>",
	}}
	xml := xmlFixture("2")
	rec := post(t, muxDe(f), "/cte/transmissao", map[string]any{
		"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xml)),
		"certificado": certValido(),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.xmlTransmitido != xml {
		t.Errorf("a lib recebeu XML diferente do enviado")
	}
	if f.tenantTransmitir.PFXBase64 == "" {
		t.Error("o certificado não chegou ao binding")
	}
	if got := f.tenantTransmitir.CNPJ; got != "12345678000190" {
		t.Errorf("CNPJ da sessão = %q, quero o extraído da chave", got)
	}

	var resp RespostaTransmissao
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "autorizado" || resp.Protocolo != "135240000000001" || resp.Chave != chaveFixture {
		t.Errorf("resposta = %+v", resp)
	}
	if resp.XMLProcBase64 == "" {
		t.Error("xml_proc_b64 vazio: é o documento autorizado que o cliente precisa guardar")
	}
}

// O ambiente sai do tpAmb do XML, não do cliente: divergir manda documento de
// homologação para o webservice de produção.
func TestAmbienteVemDoXMLNaoDoCliente(t *testing.T) {
	casos := map[string]struct{ tpAmb, ordinalQuerido string }{
		"produção":    {"1", "0"},
		"homologação": {"2", "1"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: "[CTe]\ncStat=100\n"}}
			post(t, muxDe(f), "/cte/transmissao", map[string]any{
				"xml_b64": base64.StdEncoding.EncodeToString([]byte(xmlFixture(c.tpAmb))),
				// o cliente manda o ambiente OPOSTO de propósito: o XML manda.
				"ambiente":    map[string]string{"1": "homologacao", "2": "producao"}[c.tpAmb],
				"certificado": certValido(),
			})
			var ordinal string
			for _, kv := range f.tenantTransmitir.Config {
				if kv.Key == "Ambiente" {
					ordinal = kv.Value
				}
			}
			if ordinal != c.ordinalQuerido {
				t.Errorf("ordinal do ambiente = %q, quero %q (tpAmb=%s)", ordinal, c.ordinalQuerido, c.tpAmb)
			}
		})
	}
}

func TestTransmissaoExigeCertificado(t *testing.T) {
	f := &libFake{}
	casos := map[string]any{
		"sem certificado": nil,
		"pfx vazio":       map[string]string{"pfx_b64": "", "senha": "x"},
		"pfx não base64":  map[string]string{"pfx_b64": "!!!", "senha": "x"},
		"sem senha":       map[string]string{"pfx_b64": base64.StdEncoding.EncodeToString([]byte("p")), "senha": ""},
	}
	for nome, cert := range casos {
		t.Run(nome, func(t *testing.T) {
			corpo := map[string]any{"xml_b64": base64.StdEncoding.EncodeToString([]byte(xmlFixture("2")))}
			if cert != nil {
				corpo["certificado"] = cert
			}
			rec := post(t, muxDe(f), "/cte/transmissao", corpo)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.xmlTransmitido != "" {
				t.Error("transmitiu mesmo com certificado inválido")
			}
		})
	}
}

// Desfecho indeterminado NUNCA pode virar 503: 503 convida o cliente a repetir,
// e repetir uma transmissão que talvez tenha ido duplica documento fiscal.
func TestDesfechoIndeterminadoMandaConsultar(t *testing.T) {
	f := &libFake{errTransmitir: acbr.ErrIndeterminado}
	rec := post(t, muxDe(f), "/cte/transmissao", map[string]any{
		"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
		"certificado": certValido(),
	})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, quero 502: %s", rec.Code, rec.Body)
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "desfecho_indeterminado") {
		t.Errorf("código do erro não identifica o caso: %s", corpo)
	}
	if !strings.Contains(corpo, "consulte pela chave") {
		t.Errorf("a mensagem precisa dizer o que fazer em vez de repetir: %s", corpo)
	}
}

func TestRejeicaoDaSefazEh422ComMotivo(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[CTe]\ncStat=539\nxMotivo=Duplicidade de CT-e\n",
	}}
	rec := post(t, muxDe(f), "/cte/transmissao", map[string]any{
		"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
		"certificado": certValido(),
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	var resp RespostaTransmissao
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.CStat != "539" || resp.Motivo == "" {
		t.Errorf("a rejeição precisa carregar cStat e motivo: %+v", resp)
	}
	if resp.Chave != chaveFixture {
		t.Errorf("chave = %q; a rejeição deve identificar o documento", resp.Chave)
	}
}

// --- eventos ----------------------------------------------------------------

func TestEventoCancelamentoMontaINIIndexado(t *testing.T) {
	// A resposta da lib é INI seccionado; sem o cabeçalho de seção o ParseEnvio
	// trata o texto inteiro como uma mensagem de erro (e está certo).
	f := &libFake{resEvento: acbr.Result{
		Resposta: "[Evento]\ncStat=135\nxMotivo=Evento registrado e vinculado ao CT-e\nnProt=135240000000009\n",
		XML:      "<procEventoCTe/>",
	}}
	rec := post(t, muxDe(f), "/cte/eventos/cancelamento", map[string]any{
		"chave":       chaveFixture,
		"protocolo":   "135240000000001",
		"certificado": certValido(),
		"evento":      map[string]any{"justificativa": "erro de digitacao no destinatario"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	for _, quero := range []string{"[EVENTO]", "[EVENTO001]", "tpEvento=110111",
		"chCTe=" + chaveFixture, "CNPJ=12345678000190", "nProt=135240000000001"} {
		if !strings.Contains(f.iniEvento, quero) {
			t.Errorf("INI do evento não tem %q:\n%s", quero, f.iniEvento)
		}
	}
	var resp RespostaEvento
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "concluido" || resp.XMLEventoBase64 == "" {
		t.Errorf("resposta = %+v", resp)
	}
}

func TestEventoExigeCamposDoTipo(t *testing.T) {
	casos := map[string]struct {
		tipo   string
		evento map[string]any
	}{
		"cancelamento sem justificativa": {"cancelamento", map[string]any{}},
		"carta sem correções":            {"carta-correcao", map[string]any{}},
		"comprovante sem documentos":     {"comprovante-entrega", map[string]any{}},
		"insucesso sem documentos":       {"insucesso-entrega", map[string]any{}},
		"desacordo sem xObs":             {"prestacao-desacordo", map[string]any{}},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{}
			rec := post(t, muxDe(f), "/cte/eventos/"+c.tipo, map[string]any{
				"chave": chaveFixture, "certificado": certValido(), "evento": c.evento,
			})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.iniEvento != "" {
				t.Error("enviou o evento mesmo com o pedido incompleto")
			}
		})
	}
}

func TestEventoDesconhecidoEh404(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/cte/eventos/inventado", map[string]any{
		"chave": chaveFixture, "certificado": certValido(),
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404: %s", rec.Code, rec.Body)
	}
}

func TestEventoExigeChaveDe44Digitos(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/cte/eventos/cancelamento", map[string]any{
		"chave": "123", "certificado": certValido(),
		"evento": map[string]any{"justificativa": "x"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400", rec.Code)
	}
}

// --- capacidades ------------------------------------------------------------

func TestCapacidadesCobremAsRotas(t *testing.T) {
	m := NovoModulo(&libFake{})
	caps := map[string]bool{}
	for _, c := range m.Capacidades() {
		caps[c] = true
	}
	for _, quero := range []string{"xml", "transmissao", "eventos", "consulta", "pdf"} {
		if !caps[quero] {
			t.Errorf("capacidade %q não anunciada", quero)
		}
	}
	if m.Nome() != "cte" {
		t.Errorf("nome = %q", m.Nome())
	}
}

// pedidoMinimo é o menor CT-e que passa pela validação de entrada do handler.
func pedidoMinimo() map[string]any {
	return map[string]any{
		"ambiente": "homologacao",
		"infCte": map[string]any{
			"versao": "4.00",
			"ide":    map[string]any{"CFOP": "5353", "natOp": "Transporte", "serie": 1, "nCT": 1},
			"emit":   map[string]any{"CNPJ": "12345678000190", "xNome": "ACME"},
			"vPrest": map[string]any{},
			"imp":    map[string]any{},
		},
	}
}
