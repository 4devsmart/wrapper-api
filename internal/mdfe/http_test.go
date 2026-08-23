package mdfe

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

type libFake struct {
	acbr.MDFeServico // nil: método não sobrescrito estoura, o que é bom num teste

	iniMontar, iniEvento           string
	xmlTransmitido                 string
	tenantMontar, tenantTransmitir acbr.TenantConfig
	metodoEvento                   string

	resMontar, resValidar, resTransmitir, resEvento acbr.Result
	errValidar, errTransmitir                       error
}

func (f *libFake) MontarXML(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniMontar, f.tenantMontar = ini, t
	return f.resMontar, nil
}

func (f *libFake) ValidarRegras(_ acbr.TenantConfig, _ string) (acbr.Result, error) {
	return f.resValidar, f.errValidar
}

func (f *libFake) Transmitir(t acbr.TenantConfig, xml string) (acbr.Result, error) {
	f.xmlTransmitido, f.tenantTransmitir = xml, t
	return f.resTransmitir, f.errTransmitir
}

func (f *libFake) Encerrar(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento, f.metodoEvento = ini, "Encerrar"
	return f.resEvento, nil
}

func (f *libFake) Cancelar(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento, f.metodoEvento = ini, "Cancelar"
	return f.resEvento, nil
}

func (f *libFake) EnviarEvento(_ acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento, f.metodoEvento = ini, "EnviarEvento"
	return f.resEvento, nil
}

func (f *libFake) ConsultaNaoEncerrados(_ acbr.TenantConfig, cnpj string) (acbr.Result, error) {
	return acbr.Result{Resposta: "cnpj=" + cnpj}, nil
}

// --- apoio ------------------------------------------------------------------

const chaveFixture = "35240812345678000190580010000000011000000017"

func xmlFixture(tpAmb string) string {
	return `<?xml version="1.0"?><MDFe xmlns="http://www.portalfiscal.inf.br/mdfe">` +
		`<infMDFe versao="3.00" Id="MDFe` + chaveFixture + `">` +
		`<ide><tpAmb>` + tpAmb + `</tpAmb></ide></infMDFe></MDFe>`
}

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
	return metodo + " /mdfe" + caminho
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

func certValido() map[string]string {
	return map[string]string{"pfx_b64": base64.StdEncoding.EncodeToString([]byte("pfx-falso")), "senha": "s3nh4"}
}

func pedidoMinimo() map[string]any {
	return map[string]any{
		"ambiente": "homologacao",
		"infMDFe": map[string]any{
			"versao": "3.00",
			"ide":    map[string]any{"tpEmit": 1, "serie": 1, "nMDF": 1, "modal": 1},
			"emit":   map[string]any{"CNPJ": "12345678000190", "xNome": "ACME"},
			"tot":    map[string]any{},
		},
	}
}

// --- fase 1 -----------------------------------------------------------------

func TestFase1NaoRecebeCertificado(t *testing.T) {
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	rec := post(t, muxDe(f), "/mdfe/xml", pedidoMinimo())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.tenantMontar.PFXBase64 != "" || f.tenantMontar.SenhaPFX != "" {
		t.Errorf("a fase 1 montou tenant com certificado: %+v", f.tenantMontar)
	}
	var resp RespostaXML
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Chave != chaveFixture {
		t.Errorf("chave = %q, quero %q", resp.Chave, chaveFixture)
	}
	if resp.Assinado {
		t.Error("assinado = true; a ACBrLib assina no envio")
	}
}

// A mesma armadilha do CT-e vale aqui: código 0 com rejeições não é sucesso.
func TestValidacaoUsaAResposta_NaoOCodigo(t *testing.T) {
	f := &libFake{
		resMontar:  acbr.Result{XML: xmlFixture("2")},
		resValidar: acbr.Result{Codigo: 0, Resposta: "645-Rejeicao: MDF-e sem municipio de carregamento\n"},
	}
	rec := post(t, muxDe(f), "/mdfe/xml", pedidoMinimo())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "regras_de_negocio") {
		t.Errorf("código do erro inesperado: %s", rec.Body)
	}
}

func TestFase1ExigeCNPJDoEmitente(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/mdfe/xml", map[string]any{"ambiente": "homologacao"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// --- fase 2 -----------------------------------------------------------------

func TestFase2TransmiteOXMLDaFase1(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[MDFe]\ncStat=100\nxMotivo=Autorizado o uso do MDF-e\nchMDFe=" + chaveFixture + "\nnProt=135240000000002\n",
		XML:      "<mdfeProc/>",
	}}
	xml := xmlFixture("2")
	rec := post(t, muxDe(f), "/mdfe/transmissao", map[string]any{
		"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xml)),
		"certificado": certValido(),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.xmlTransmitido != xml {
		t.Error("a lib recebeu XML diferente do enviado")
	}
	if f.tenantTransmitir.CNPJ != "12345678000190" {
		t.Errorf("CNPJ da sessão = %q, quero o extraído da chave", f.tenantTransmitir.CNPJ)
	}
	var resp RespostaTransmissao
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "autorizado" || resp.Protocolo != "135240000000002" {
		t.Errorf("resposta = %+v", resp)
	}
	if resp.XMLProcBase64 == "" {
		t.Error("xml_proc_b64 vazio: é o documento autorizado que o cliente guarda")
	}
}

func TestAmbienteVemDoXMLNaoDoCliente(t *testing.T) {
	casos := map[string]struct{ tpAmb, ordinal string }{
		"produção":    {"1", "0"},
		"homologação": {"2", "1"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: "[MDFe]\ncStat=100\n"}}
			post(t, muxDe(f), "/mdfe/transmissao", map[string]any{
				"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xmlFixture(c.tpAmb))),
				"ambiente":    map[string]string{"1": "homologacao", "2": "producao"}[c.tpAmb],
				"certificado": certValido(),
			})
			var ordinal string
			for _, kv := range f.tenantTransmitir.Config {
				if kv.Key == "Ambiente" {
					ordinal = kv.Value
				}
			}
			if ordinal != c.ordinal {
				t.Errorf("ordinal = %q, quero %q (tpAmb=%s)", ordinal, c.ordinal, c.tpAmb)
			}
		})
	}
}

func TestDesfechoIndeterminadoMandaConsultar(t *testing.T) {
	f := &libFake{errTransmitir: acbr.ErrIndeterminado}
	rec := post(t, muxDe(f), "/mdfe/transmissao", map[string]any{
		"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
		"certificado": certValido(),
	})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, quero 502: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "consulte pela chave") {
		t.Errorf("a mensagem precisa dizer o que fazer em vez de repetir: %s", rec.Body)
	}
}

// --- eventos ----------------------------------------------------------------

// Encerramento e cancelamento têm método PRÓPRIO na lib; os demais vão pelo
// EnviarEvento genérico. Trocar isso quebra o evento em silêncio.
func TestEventoUsaOMetodoCertoDaLib(t *testing.T) {
	casos := map[string]struct {
		tipo, metodo string
		evento       map[string]any
	}{
		"encerramento":      {"encerramento", "Encerrar", map[string]any{"cMun": "3550308"}},
		"cancelamento":      {"cancelamento", "Cancelar", map[string]any{"justificativa": "erro na emissao do manifesto"}},
		"inclusao-condutor": {"inclusao-condutor", "EnviarEvento", map[string]any{"nome": "JOAO", "cpf": "12345678909"}},
		"inclusao-dfe": {"inclusao-dfe", "EnviarEvento", map[string]any{
			"documentos": []map[string]string{{"chNFe": strings.Repeat("1", 44), "cMunDescarga": "3550308", "xMunDescarga": "SAO PAULO"}},
		}},
		"pagamento-operacao": {"pagamento-operacao", "EnviarEvento", map[string]any{
			"pagamentos": []map[string]any{{"xNome": "TRANSP", "CNPJ": "12345678000190", "vContrato": 1000.0, "indPag": 0}},
		}},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resEvento: acbr.Result{
				Resposta: "[Evento]\ncStat=135\nxMotivo=Evento registrado\nnProt=135240000000009\n",
				XML:      "<procEventoMDFe/>",
			}}
			rec := post(t, muxDe(f), "/mdfe/eventos/"+c.tipo, map[string]any{
				"chave": chaveFixture, "protocolo": "135240000000002",
				"certificado": certValido(), "evento": c.evento,
			})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if f.metodoEvento != c.metodo {
				t.Errorf("chamou %s, quero %s", f.metodoEvento, c.metodo)
			}
			for _, quero := range []string{"[EVENTO]", "[EVENTO001]", "chMDFe=" + chaveFixture, "CNPJCPF=12345678000190"} {
				if !strings.Contains(f.iniEvento, quero) {
					t.Errorf("INI não tem %q:\n%s", quero, f.iniEvento)
				}
			}
			var resp RespostaEvento
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.Status != "concluido" || resp.XMLEventoBase64 == "" {
				t.Errorf("resposta = %+v", resp)
			}
		})
	}
}

func TestEventoExigeCamposDoTipo(t *testing.T) {
	casos := map[string]string{
		"encerramento sem cMun":          "encerramento",
		"cancelamento sem justificativa": "cancelamento",
		"condutor sem nome/cpf":          "inclusao-condutor",
		"dfe sem documentos":             "inclusao-dfe",
		"pagamento sem pagamentos":       "pagamento-operacao",
	}
	for nome, tipo := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{}
			rec := post(t, muxDe(f), "/mdfe/eventos/"+tipo, map[string]any{
				"chave": chaveFixture, "certificado": certValido(), "evento": map[string]any{},
			})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.iniEvento != "" {
				t.Error("enviou o evento com o pedido incompleto")
			}
		})
	}
}

func TestEventoDesconhecidoEh404(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/mdfe/eventos/inventado", map[string]any{
		"chave": chaveFixture, "certificado": certValido(),
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404", rec.Code)
	}
}

// --- rotas ------------------------------------------------------------------

// A recepção assíncrona foi desativada pela SEFAZ; no MDF-e o ACBr sequer olha
// o parâmetro. Expor recibo seria oferecer caminho que só sabe falhar.
func TestNaoExisteRotaDeRecibo(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/mdfe/recibo", map[string]any{"certificado": certValido()})
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /mdfe/recibo = %d, quero 404", rec.Code)
	}
	for _, c := range NovoModulo(&libFake{}).Capacidades() {
		if c == "recibo" {
			t.Error("capacidades ainda anunciam 'recibo'")
		}
	}
}

func TestNaoEncerradosExigeCNPJ(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/mdfe/nao-encerrados", map[string]any{"certificado": certValido()})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
	rec = post(t, muxDe(&libFake{}), "/mdfe/nao-encerrados", map[string]any{
		"cnpj": "12.345.678/0001-90", "certificado": certValido(),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "cnpj=12345678000190") {
		t.Errorf("o CNPJ deveria chegar só com dígitos: %s", rec.Body)
	}
}

func TestNomeECapacidades(t *testing.T) {
	m := NovoModulo(&libFake{})
	if m.Nome() != "mdfe" {
		t.Errorf("nome = %q", m.Nome())
	}
	caps := strings.Join(m.Capacidades(), ",")
	for _, quero := range []string{"xml", "transmissao", "eventos", "nao-encerrados", "pdf"} {
		if !strings.Contains(caps, quero) {
			t.Errorf("capacidade %q não anunciada", quero)
		}
	}
}
