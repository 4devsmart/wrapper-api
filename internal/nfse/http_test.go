package nfse

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

// Municípios REAIS da tabela embutida, um por família: testar com código
// inventado esconderia justamente o roteamento, que é o que a NFS-e tem de
// diferente.
const (
	munPadraoNacional = "3304557" // Rio de Janeiro  → PadraoNacional
	munAbrasf         = "1100015" // Alta Floresta d'Oeste → WebISS (abrasf_v1_v2)
	munProprio        = "3550308" // São Paulo → ISSSaoPaulo (próprio)
	munDesconhecido   = "9999999"
)

// --- duplo do binding -------------------------------------------------------

type libFake struct {
	acbr.NFSeServico // nil: método não sobrescrito estoura, o que é bom num teste

	iniMontar, iniCancelar, iniSubstituir             string
	xmlTransmitido, chaveDPS, chaveConsulta, chavePDF string
	tenant                                            acbr.TenantConfig
	sub                                               acbr.SubstituicaoNFSe

	resMontar, resTransmitir, resCancelar, resConsulta acbr.Result
	errTransmitir                                      error
}

func (f *libFake) MontarXML(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniMontar, f.tenant = ini, t
	return f.resMontar, nil
}

// A lib NÃO expõe validação de regras para NFS-e. O fake reproduz isso: é a
// diferença que a resposta precisa comunicar em vez de esconder.
func (f *libFake) ValidarRegras(acbr.TenantConfig, string) (acbr.Result, error) {
	return acbr.Result{}, acbr.ErrNaoSuportado
}

func (f *libFake) Transmitir(t acbr.TenantConfig, xml string) (acbr.Result, error) {
	f.xmlTransmitido, f.tenant = xml, t
	return f.resTransmitir, f.errTransmitir
}

func (f *libFake) Cancelar(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniCancelar, f.tenant = ini, t
	return f.resCancelar, nil
}

func (f *libFake) SubstituirNFSe(t acbr.TenantConfig, ini string, s acbr.SubstituicaoNFSe) (acbr.Result, error) {
	f.iniSubstituir, f.sub, f.tenant = ini, s, t
	return f.resTransmitir, nil
}

func (f *libFake) ConsultarDPSPorChave(t acbr.TenantConfig, chave string) (acbr.Result, error) {
	f.chaveDPS, f.tenant = chave, t
	return f.resConsulta, nil
}

func (f *libFake) Consultar(t acbr.TenantConfig, chave string) (acbr.Result, error) {
	f.chaveConsulta, f.tenant = chave, t
	return f.resConsulta, nil
}

func (f *libFake) ObterPDF(t acbr.TenantConfig, chave string) (acbr.Result, error) {
	f.chavePDF, f.tenant = chave, t
	return acbr.Result{PDF: []byte("%PDF-1.4")}, nil
}

func (f *libFake) RenderizarPDF(t acbr.TenantConfig, _ string) (acbr.Result, error) {
	f.tenant = t
	return acbr.Result{PDF: []byte("%PDF-1.4 local")}, nil
}

func (f *libFake) ConsultarPorNumero(t acbr.TenantConfig, n string, pg int) (acbr.Result, error) {
	f.tenant = t
	return acbr.Result{Resposta: "numero=" + n + ";pagina=" + itoa(pg)}, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// --- apoio ------------------------------------------------------------------

const idDPSFixture = "DPS330455721234567800019000001000000000000001"

func xmlFixture(tpAmb string) string {
	return `<?xml version="1.0"?><DPS xmlns="http://www.sped.fazenda.gov.br/nfse">` +
		`<infDPS versao="1.00" Id="` + idDPSFixture + `">` +
		`<tpAmb>` + tpAmb + `</tpAmb></infDPS></DPS>`
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
	return metodo + " /nfse" + caminho
}

var _ modulo.Modulo = (*Modulo)(nil)

func chamar(t *testing.T, mux *http.ServeMux, metodo, caminho string, corpo any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if corpo == nil {
		r = httptest.NewRequest(metodo, caminho, nil)
	} else {
		b, err := json.Marshal(corpo)
		if err != nil {
			t.Fatal(err)
		}
		r = httptest.NewRequest(metodo, caminho, strings.NewReader(string(b)))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func post(t *testing.T, mux *http.ServeMux, caminho string, corpo any) *httptest.ResponseRecorder {
	t.Helper()
	return chamar(t, mux, http.MethodPost, caminho, corpo)
}

func certValido() map[string]string {
	return map[string]string{"pfx_b64": base64.StdEncoding.EncodeToString([]byte("pfx-falso")), "senha": "s3nh4"}
}

func pedidoMinimo(cmun string) map[string]any {
	return map[string]any{
		"ambiente": "homologacao",
		"infDPS": map[string]any{
			"serie": "1", "nDPS": "1", "dCompet": "2026-08-01",
			"prest": map[string]any{
				"CNPJ": "12345678000190", "xNome": "ACME", "cMun": cmun, "IM": "9876",
			},
			"serv":    map[string]any{},
			"valores": map[string]any{},
		},
	}
}

func envelope(cmun string, extra map[string]any) map[string]any {
	c := map[string]any{
		"municipio":   cmun,
		"certificado": certValido(),
		"emitente":    map[string]any{"cnpj": "12345678000190", "inscricao_municipal": "9876", "razao_social": "ACME"},
	}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func cfgDoTenant(t acbr.TenantConfig, chave string) string {
	for _, kv := range t.Config {
		if kv.Key == chave {
			return kv.Value
		}
	}
	return ""
}

// --- roteamento por município -----------------------------------------------

// O roteamento é o que a NFS-e tem de diferente: o município decide o provedor,
// o provedor decide a família, e a família decide o construtor de INI.
func TestLayoutPorMunicipio(t *testing.T) {
	casos := map[string]struct {
		cmun   string
		layout Layout
		ok     bool
	}{
		"padrão nacional": {munPadraoNacional, LayoutPadraoNacional, true},
		"abrasf":          {munAbrasf, LayoutAbrasf, true},
		"próprio":         {munProprio, LayoutProprio, true},
		"desconhecido":    {munDesconhecido, "", false},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			l, ok := LayoutDoMunicipio(c.cmun)
			if ok != c.ok || l != c.layout {
				t.Errorf("LayoutDoMunicipio(%s) = %q,%v; quero %q,%v", c.cmun, l, ok, c.layout, c.ok)
			}
		})
	}
}

// Município sem provedor conhecido é recusado ANTES de qualquer coisa sair:
// mandar XML que a prefeitura rejeitaria só troca um erro claro por um obscuro.
func TestMunicipioSemProvedorEhRecusadoAntesDeMontar(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/xml", pedidoMinimo(munDesconhecido))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "provedor_nao_suportado") {
		t.Errorf("código do erro inesperado: %s", rec.Body)
	}
	if f.iniMontar != "" {
		t.Error("montou o INI de um município sem provedor")
	}
}

// O construtor de INI muda com a família: Padrão Nacional gera DPS, os demais
// geram o RPS genérico. Trocá-los produz XML que a prefeitura recusa.
func TestConstrutorDeINIMudaComAFamilia(t *testing.T) {
	pn := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	post(t, muxDe(pn), "/nfse/xml", pedidoMinimo(munPadraoNacional))

	ab := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	post(t, muxDe(ab), "/nfse/xml", pedidoMinimo(munAbrasf))

	if pn.iniMontar == "" || ab.iniMontar == "" {
		t.Fatal("algum INI não foi montado")
	}
	if pn.iniMontar == ab.iniMontar {
		t.Error("Padrão Nacional e ABRASF geraram o MESMO INI; o roteamento não está sendo aplicado")
	}
}

// --- gerar -----------------------------------------------------------------

func TestGerarNaoRecebeCertificadoEDevolveOIDdaDPS(t *testing.T) {
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	rec := post(t, muxDe(f), "/nfse/xml", pedidoMinimo(munPadraoNacional))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.tenant.PFXBase64 != "" {
		t.Errorf("a geração montou tenant com certificado: %+v", f.tenant)
	}
	if got := cfgDoTenant(f.tenant, "CodigoMunicipio"); got != munPadraoNacional {
		t.Errorf("CodigoMunicipio = %q, quero %q", got, munPadraoNacional)
	}

	var resp RespostaXML
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.IDdps != idDPSFixture {
		t.Errorf("id_dps = %q, quero %q, sem ele não há como recuperar transmissão perdida", resp.IDdps, idDPSFixture)
	}
	if resp.Assinado {
		t.Error("assinado = true; na NFS-e quem assina é o provedor, dentro do envio")
	}
	if resp.Layout != string(LayoutPadraoNacional) || resp.Provedor == "" {
		t.Errorf("layout/provedor não informados: %+v", resp)
	}
}

// A lib não expõe validação de regras para NFS-e. Dizer "ok" sem ressalva seria
// mentir sobre a garantia que o cliente tem em mãos.
func TestValidacaoDeclaraQueNaoRodou(t *testing.T) {
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	rec := post(t, muxDe(f), "/nfse/xml", pedidoMinimo(munPadraoNacional))
	var resp RespostaXML
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Validacao.Suportada {
		t.Error("validacao.suportada = true, mas o ACBrNFSeX não exporta validação de regras")
	}
}

func TestGerarExigeCNPJDoPrestador(t *testing.T) {
	p := pedidoMinimo(munPadraoNacional)
	p["infDPS"].(map[string]any)["prest"] = map[string]any{"cMun": munPadraoNacional}
	rec := post(t, muxDe(&libFake{}), "/nfse/xml", p)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// --- transmitir -----------------------------------------------------------------

func TestTransmiteComCertificadoEEmitente(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[Envio]\nSucesso=1\nNumeroNota=123\nLink=chave-da-nfse\nCodigoVerificacao=ABC\nProtocolo=P1\n",
		XML:      "<NFSe/>",
	}}
	xml := xmlFixture("2")
	rec := post(t, muxDe(f), "/nfse/transmissao", envelope(munPadraoNacional, map[string]any{
		"xml_b64": base64.StdEncoding.EncodeToString([]byte(xml)),
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.xmlTransmitido != xml {
		t.Error("a lib recebeu XML diferente do enviado")
	}
	if f.tenant.PFXBase64 == "" {
		t.Error("o certificado não chegou ao binding")
	}
	if got := cfgDoTenant(f.tenant, "Emitente.InscMun"); got != "9876" {
		t.Errorf("Emitente.InscMun = %q; sem cadastro no servidor ele vem do pedido", got)
	}
	var resp RespostaTransmissao
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "autorizado" || resp.Numero != "123" || resp.CodigoVerificacao != "ABC" {
		t.Errorf("resposta = %+v", resp)
	}
}

func TestAmbienteVemDoXMLNaoDoCliente(t *testing.T) {
	casos := map[string]struct{ tpAmb, ordinal string }{
		"produção":    {"1", "0"},
		"homologação": {"2", "1"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\n"}}
			post(t, muxDe(f), "/nfse/transmissao", envelope(munPadraoNacional, map[string]any{
				"xml_b64":  base64.StdEncoding.EncodeToString([]byte(xmlFixture(c.tpAmb))),
				"ambiente": map[string]string{"1": "homologacao", "2": "producao"}[c.tpAmb],
			}))
			if got := cfgDoTenant(f.tenant, "Ambiente"); got != c.ordinal {
				t.Errorf("ordinal = %q, quero %q (tpAmb=%s)", got, c.ordinal, c.tpAmb)
			}
		})
	}
}

// Credenciais de prefeitura só valem para provedores não-Padrão Nacional. Mandar
// login/senha para o ADN seria configurar lixo numa sessão que não os usa.
func TestCredenciaisSoValemForaDoPadraoNacional(t *testing.T) {
	corpo := func(cmun string) map[string]any {
		return envelope(cmun, map[string]any{
			"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
			"credenciais": map[string]string{"usuario": "u", "senha": "p", "token": "tk"},
		})
	}
	pn := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\n"}}
	post(t, muxDe(pn), "/nfse/transmissao", corpo(munPadraoNacional))
	if cfgDoTenant(pn.tenant, "Emitente.WSUser") != "" {
		t.Error("credenciais de prefeitura foram enviadas ao Padrão Nacional")
	}

	ab := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\n"}}
	post(t, muxDe(ab), "/nfse/transmissao", corpo(munAbrasf))
	if cfgDoTenant(ab.tenant, "Emitente.WSUser") != "u" || cfgDoTenant(ab.tenant, "Emitente.WSChaveAcesso") != "tk" {
		t.Errorf("credenciais não chegaram ao provedor ABRASF: %+v", ab.tenant.Config)
	}
}

func TestDesfechoIndeterminadoMandaConsultar(t *testing.T) {
	f := &libFake{errTransmitir: acbr.ErrIndeterminado}
	rec := post(t, muxDe(f), "/nfse/transmissao", envelope(munPadraoNacional, map[string]any{
		"xml_b64": base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
	}))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, quero 502: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "consulte pela chave") {
		t.Errorf("a mensagem precisa dizer o que fazer em vez de repetir: %s", rec.Body)
	}
}

// --- recuperação ------------------------------------------------------------

// É o fecho do modelo sem estado para a NFS-e: a geração devolve o id_dps, e é
// com ele que se descobre se a DPS virou nota. O ADN atende GET /dps/{chave}
// com a chave SEM o prefixo "DPS": o cliente não precisa saber disso.
func TestConsultaDPSAceitaOIDComOuSemPrefixo(t *testing.T) {
	chaveNua := strings.TrimPrefix(idDPSFixture, "DPS")
	for _, entrada := range []string{idDPSFixture, chaveNua} {
		t.Run(entrada[:6], func(t *testing.T) {
			f := &libFake{resConsulta: acbr.Result{Resposta: "[Envio]\nSucesso=1\n"}}
			rec := post(t, muxDe(f), "/nfse/consulta-dps", envelope(munPadraoNacional, map[string]any{
				"chave": entrada,
			}))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if f.chaveDPS != chaveNua {
				t.Errorf("a lib recebeu %q, quero a chave sem prefixo %q", f.chaveDPS, chaveNua)
			}
		})
	}
}

func TestConsultaDPSExigeChave(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/nfse/consulta-dps", envelope(munPadraoNacional, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// --- capacidade por provedor ------------------------------------------------

// A capacidade de cada provedor é descoberta em RUNTIME, não há tabela dizendo
// o que cada município aceita. O erro nativo vira 422 tipado em vez de 502.
func TestOperacaoNaoImplementadaViraErroTipado(t *testing.T) {
	f := &libFake{resCancelar: acbr.Result{
		Resposta: "[Erro1]\nCodigo=E999\nDescricao=Metodo nao implementado para este provedor\n",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munAbrasf, map[string]any{
		"chave":  "chave-da-nfse",
		"evento": map[string]any{"motivo": "erro na emissao"},
	}))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "operacao_nao_suportada") {
		t.Errorf("código do erro inesperado: %s", rec.Body)
	}
}

// --- eventos ----------------------------------------------------------------

func TestCancelamentoMontaINI(t *testing.T) {
	f := &libFake{resCancelar: acbr.Result{
		Resposta: "[Cancelamento]\nSucesso=1\nProtocolo=P9\n", XML: "<evento/>",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
		"chave":  "chave-da-nfse",
		"evento": map[string]any{"codigo": "2", "motivo": "servico nao prestado"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	for _, quero := range []string{"[CancelarNFSe]", "ChaveNFSe=chave-da-nfse", "CodCancelamento=2", "MotCancelamento=servico nao prestado"} {
		if !strings.Contains(f.iniCancelar, quero) {
			t.Errorf("INI não tem %q:\n%s", quero, f.iniCancelar)
		}
	}
	var resp RespostaEvento
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "concluido" || resp.Protocolo != "P9" {
		t.Errorf("resposta = %+v", resp)
	}
}

func TestSubstituicaoExigeNumeroDaSubstituida(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munAbrasf, map[string]any{
		"evento": map[string]any{"dps": pedidoMinimo(munAbrasf), "substituida": map[string]any{}},
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
	if f.iniSubstituir != "" {
		t.Error("substituiu sem identificar a nota antiga")
	}
}

func TestSubstituicaoPassaOsIdentificadoresDaNotaAntiga(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\nNumeroNota=999\n"}}
	rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munAbrasf, map[string]any{
		"evento": map[string]any{
			"dps":         pedidoMinimo(munAbrasf),
			"substituida": map[string]any{"numero": "100", "serie": "A", "codigo_verificacao": "XYZ"},
			"motivo":      "erro de valor",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.sub.NumeroNFSe != "100" || f.sub.SerieNFSe != "A" || f.sub.CodigoVerificacao != "XYZ" {
		t.Errorf("identificadores da substituída = %+v", f.sub)
	}
	if f.sub.CodigoCancelamento != "1" {
		t.Errorf("código de cancelamento default = %q, quero 1", f.sub.CodigoCancelamento)
	}
	if f.iniSubstituir == "" {
		t.Error("a DPS substituta não foi montada")
	}
}

func TestEventoDesconhecidoEh404(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/nfse/eventos/inventado", envelope(munPadraoNacional, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404", rec.Code)
	}
}

// --- DANFSE -----------------------------------------------------------------

// Diferente de CT-e e MDF-e, a NFS-e recupera o PDF pela CHAVE: perder o XML
// não é definitivo. Por chave fala com o provedor (pede certificado); por XML é
// render local (não pede).
func TestPDFPorChaveEPorXML(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/pdf", envelope(munPadraoNacional, map[string]any{"chave": "chave-da-nfse"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("por chave: status = %d: %s", rec.Code, rec.Body)
	}
	if f.chavePDF != "chave-da-nfse" {
		t.Errorf("a lib recebeu chave %q", f.chavePDF)
	}

	f2 := &libFake{}
	rec = post(t, muxDe(f2), "/nfse/pdf", map[string]any{
		"xml_b64": base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("por XML: status = %d: %s", rec.Code, rec.Body)
	}
	if f2.tenant.PFXBase64 != "" {
		t.Error("render local pediu certificado")
	}
	if !strings.Contains(rec.Body.String(), "pdf_b64") {
		t.Errorf("resposta sem pdf_b64: %s", rec.Body)
	}
}

func TestPDFExigeChaveOuXML(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/nfse/pdf", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
}

// --- consultas e municípios -------------------------------------------------

func TestConsultaPorNumeroUsaPaginaUmPorPadrao(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/consultas/numero", envelope(munPadraoNacional, map[string]any{"numero": "42"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "pagina=1") {
		t.Errorf("página default deveria ser 1: %s", rec.Body)
	}
}

func TestConsultaDesconhecidaEh404(t *testing.T) {
	rec := post(t, muxDe(&libFake{}), "/nfse/consultas/inventada", envelope(munPadraoNacional, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quero 404", rec.Code)
	}
}

// A tabela de municípios é como o cliente descobre, antes de montar qualquer
// coisa, se vale a pena tentar. Não leva certificado.
func TestConsultaDeMunicipioNaoPedeCertificado(t *testing.T) {
	rec := chamar(t, muxDe(&libFake{}), http.MethodGet, "/nfse/municipios/"+munPadraoNacional, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var corpo struct {
		Provedor  string `json:"provedor"`
		Layout    string `json:"layout"`
		Suportado bool   `json:"suportado"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &corpo)
	if !corpo.Suportado || corpo.Layout != string(LayoutPadraoNacional) || corpo.Provedor == "" {
		t.Errorf("corpo = %+v", corpo)
	}

	rec = chamar(t, muxDe(&libFake{}), http.MethodGet, "/nfse/municipios/"+munDesconhecido, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &corpo)
	if corpo.Suportado {
		t.Error("município inexistente marcado como suportado")
	}
}

func TestNomeECapacidades(t *testing.T) {
	m := NovoModulo(&libFake{})
	if m.Nome() != "nfse" {
		t.Errorf("nome = %q", m.Nome())
	}
	caps := strings.Join(m.Capacidades(), ",")
	for _, quero := range []string{"xml", "transmissao", "eventos", "consulta-dps", "pdf"} {
		if !strings.Contains(caps, quero) {
			t.Errorf("capacidade %q não anunciada", quero)
		}
	}
}
