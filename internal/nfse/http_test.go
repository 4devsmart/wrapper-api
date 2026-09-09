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
	iniEvento, iniEmitir                              string
	xmlTransmitido, chaveDPS, chaveConsulta, chavePDF string
	tenant                                            acbr.TenantConfig
	sub                                               acbr.SubstituicaoNFSe

	resMontar, resTransmitir, resCancelar, resConsulta acbr.Result
	resEvento, resProvedor                             acbr.Result
	errTransmitir                                      error
}

func (f *libFake) Emitir(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEmitir, f.tenant = ini, t
	return f.resTransmitir, nil
}

func (f *libFake) EnviarEvento(t acbr.TenantConfig, ini string) (acbr.Result, error) {
	f.iniEvento, f.tenant = ini, t
	return f.resEvento, nil
}

// InformacoesProvedor NÃO registra o tenant: ela é chamada DEPOIS da operação,
// para enriquecer um erro, e sobrescrever f.tenant aqui apagaria a sessão que os
// testes da operação conferem.
func (f *libFake) InformacoesProvedor(acbr.TenantConfig) (acbr.Result, error) {
	return f.resProvedor, nil
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
		`<tpAmb>` + tpAmb + `</tpAmb>` +
		`<cLocEmi>` + munPadraoNacional + `</cLocEmi></infDPS></DPS>`
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

// O município do XML é o que seleciona o provedor no render do DANFSE. A ordem
// importa: no ABRASF o documento traz vários CodigoMunicipio (prestador,
// prestação, órgão gerador) e só o do OrgaoGerador diz quem emitiu.
func TestMunicipioDoXML(t *testing.T) {
	casos := []struct {
		nome, xml, quero string
	}{
		{"padrão nacional: cLocEmi", `<DPS><infDPS><cLocEmi>4314902</cLocEmi>` +
			`<prest><cMun>3550308</cMun></prest></infDPS></DPS>`, "4314902"},
		{"abrasf: o município do OrgaoGerador, não o do prestador", `<Nfse><InfNfse>` +
			`<PrestadorServico><Endereco><CodigoMunicipio>3550308</CodigoMunicipio></Endereco></PrestadorServico>` +
			`<OrgaoGerador><CodigoMunicipio>3304557</CodigoMunicipio><Uf>RJ</Uf></OrgaoGerador>` +
			`</InfNfse></Nfse>`, "3304557"},
		{"abrasf com prefixo de namespace", `<ns2:Nfse><ns2:OrgaoGerador>` +
			`<ns2:CodigoMunicipio>1100015</ns2:CodigoMunicipio></ns2:OrgaoGerador></ns2:Nfse>`, "1100015"},
		{"sem emissor: cai na incidência do ISS", `<NFSe><infNFSe><cLocIncid>4314902</cLocIncid></infNFSe></NFSe>`, "4314902"},
		{"nada identificável", `<NFSe><infNFSe><nNFSe>18</nNFSe></infNFSe></NFSe>`, ""},
		{"código truncado não passa por município", `<DPS><cLocEmi>431490</cLocEmi></DPS>`, ""},
	}
	for _, c := range casos {
		if got := MunicipioDoXML(c.xml); got != c.quero {
			t.Errorf("%s: MunicipioDoXML = %q, quero %q", c.nome, got, c.quero)
		}
	}
}

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
//
// E a FRASE importa tanto quanto o código. A anterior dizia que o provedor do
// município não oferecia a operação por webservice, e isso é falso: quem não
// implementa é a biblioteca fiscal, que decide antes de qualquer byte sair. Um
// cliente que lesse aquilo ligava para a prefeitura errada.
func TestOperacaoNaoImplementadaViraErroTipado(t *testing.T) {
	f := &libFake{
		resCancelar: acbr.Result{
			Resposta: "[Erro1]\nCodigo=E999\nDescricao=Metodo nao implementado para este provedor\n",
		},
		resProvedor: acbr.Result{
			Resposta: "[ObterInformacoesProvedor]\n" +
				"IdentificacaoProvedor=Nome:WebISS|Versao:2.02\n" +
				"ServicosDisponibilizados=EnviarLoteSincrono|ConsultarNfse|\n",
		},
	}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munAbrasf, map[string]any{
		"chave":  "chave-da-nfse",
		"evento": map[string]any{"motivo": "erro na emissao", "numero": "100"},
	}))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "operacao_nao_suportada") {
		t.Errorf("código do erro inesperado: %s", corpo)
	}
	if !strings.Contains(corpo, "biblioteca fiscal") {
		t.Errorf("a mensagem não diz de quem é o limite: %s", corpo)
	}
	if strings.Contains(corpo, "não oferece") {
		t.Errorf("a mensagem voltou a culpar o município: %s", corpo)
	}
	// O que o provedor DE FATO expõe vem junto: é o que transforma "não dá" em
	// "não dá, e o que dá é isto".
	if !strings.Contains(corpo, "ConsultarNfse") {
		t.Errorf("os serviços do provedor não vieram nos detalhes: %s", corpo)
	}
}

// --- eventos ----------------------------------------------------------------

// O cancelamento tem DOIS webservices, e o município escolhe qual. Estes dois
// testes são a rede contra a regressão que existiu: chamar o CancelaNFSe num
// município do Padrão Nacional devolvia "não implementado para este provedor"
// em 300 ms, sem nada sair para o ADN, e o cliente lia que a prefeitura dele
// não oferecia cancelamento.
func TestCancelamentoAbrasfVaiPeloCancelaNFSe(t *testing.T) {
	f := &libFake{resCancelar: acbr.Result{
		Resposta: "[Cancelamento]\nSucesso=1\nProtocolo=P9\n", XML: "<evento/>",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munAbrasf, map[string]any{
		"chave": "chave-da-nfse",
		"evento": map[string]any{
			"codigo": "2", "motivo": "servico nao prestado",
			"numero": "100", "serie": "A", "codigo_verificacao": "XYZ",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	// NumeroNFSe não é detalhe: o ABRASF v2 recusa de saída sem ele, antes de
	// ir ao webservice, e era a segunda forma de o cancelamento nunca funcionar.
	for _, quero := range []string{
		"[CancelarNFSe]", "ChaveNFSe=chave-da-nfse", "CodCancelamento=2",
		"MotCancelamento=servico nao prestado", "NumeroNFSe=100", "SerieNFSe=A",
		"CodVerificacao=XYZ",
	} {
		if !strings.Contains(f.iniCancelar, quero) {
			t.Errorf("INI não tem %q:\n%s", quero, f.iniCancelar)
		}
	}
	if f.iniEvento != "" {
		t.Errorf("mandou evento num município ABRASF:\n%s", f.iniEvento)
	}
	var resp RespostaEvento
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "concluido" || resp.Protocolo != "P9" {
		t.Errorf("resposta = %+v", resp)
	}
}

func TestCancelamentoPadraoNacionalVaiPorEvento(t *testing.T) {
	f := &libFake{resEvento: acbr.Result{
		Resposta: "[EnviarEvento]\nSucessoCanc=1\nDescSituacao=Nota Cancelada\nXmlRetorno=<procEveNFSe/>\n",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
		"chave":  "chave-da-nfse",
		"evento": map[string]any{"codigo": "2", "motivo": "servico nao prestado"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.iniCancelar != "" {
		t.Errorf("chamou o CancelaNFSe no Padrão Nacional:\n%s", f.iniCancelar)
	}
	for _, quero := range []string{
		"[Evento]", "tpEvento=e101101", "chNFSe=chave-da-nfse",
		"cMotivo=2", "xMotivo=servico nao prestado", "tpAmb=2",
	} {
		if !strings.Contains(f.iniEvento, quero) {
			t.Errorf("INI do evento não tem %q:\n%s", quero, f.iniEvento)
		}
	}
	var resp RespostaEvento
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "concluido" || resp.Situacao != "Nota Cancelada" {
		t.Errorf("resposta = %+v", resp)
	}
}

// xMotivo é obrigatório e tem mínimo de 15 caracteres no schema do evento. Sem
// esta recusa o ADN devolve erro de schema, que não diz qual campo faltou.
func TestCancelamentoPadraoNacionalExigeMotivoDeQuinzeCaracteres(t *testing.T) {
	for _, caso := range []struct{ nome, motivo, codigo string }{
		{"sem motivo", "", "1"},
		{"motivo curto", "erro", "1"},
		{"código fora da lista", "servico nao prestado", "3"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			f := &libFake{}
			rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
				"chave":  "chave-da-nfse",
				"evento": map[string]any{"codigo": caso.codigo, "motivo": caso.motivo},
			}))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.iniEvento != "" {
				t.Error("transmitiu um evento que o schema recusaria")
			}
		})
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

// O Padrão Nacional não tem webservice de substituição: a nota nova carrega o
// grupo subst apontando a chave da antiga, e emiti-la É a substituição. Chamar
// o SubstituiNFSe aqui era a mesma recusa local do cancelamento.
func TestSubstituicaoPadraoNacionalEmiteDPSComGrupoSubst(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\nNumeroNota=999\n"}}
	rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munPadraoNacional, map[string]any{
		"evento": map[string]any{
			"dps":         pedidoMinimo(munPadraoNacional),
			"substituida": map[string]any{"chave": "chave-da-antiga"},
			"codigo":      "05",
			"motivo":      "rejeitada pelo tomador",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.iniSubstituir != "" {
		t.Errorf("chamou o SubstituiNFSe no Padrão Nacional:\n%s", f.iniSubstituir)
	}
	for _, quero := range []string{
		"[NFSeSubstituicao]", "chSubstda=chave-da-antiga", "cMotivo=05",
		"xMotivo=rejeitada pelo tomador",
	} {
		if !strings.Contains(f.iniEmitir, quero) {
			t.Errorf("INI da DPS substituta não tem %q:\n%s", quero, f.iniEmitir)
		}
	}
	// A DPS inteira continua ali: o grupo subst é um acréscimo à nota, não um
	// documento separado.
	if !strings.Contains(f.iniEmitir, "[IdentificacaoRps]") {
		t.Errorf("a DPS substituta não foi montada:\n%s", f.iniEmitir)
	}
}

// O cMotivo da substituição tem lista PRÓPRIA (dois dígitos) e a lib converte o
// que não reconhece no primeiro da enumeração. Escolher um default seria emitir
// uma nota com justificativa que ninguém pediu.
func TestSubstituicaoPadraoNacionalExigeChaveECodigoDaLista(t *testing.T) {
	for _, caso := range []struct {
		nome   string
		evento map[string]any
	}{
		{"sem chave da substituída", map[string]any{"codigo": "05"}},
		{"sem código", map[string]any{"chave": "chave-da-antiga"}},
		{"código do cancelamento, não da substituição", map[string]any{"chave": "chave-da-antiga", "codigo": "1"}},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			f := &libFake{}
			ev := map[string]any{"dps": pedidoMinimo(munPadraoNacional)}
			if c, ok := caso.evento["codigo"]; ok {
				ev["codigo"] = c
			}
			ev["substituida"] = map[string]any{"chave": caso.evento["chave"]}
			rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munPadraoNacional, map[string]any{"evento": ev}))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
			}
			if f.iniEmitir != "" {
				t.Error("emitiu a substituta sem a justificativa certa")
			}
		})
	}
}

// A resposta de capacidades tem que dizer o que ESTA API faz no município, não
// repassar a lista crua do provedor: o Padrão Nacional declara CancelarNfse
// falso e mesmo assim cancela, por evento.
func TestCapacidadesDoMunicipioRespondemPeloCaminhoQueUsamos(t *testing.T) {
	f := &libFake{resProvedor: acbr.Result{
		Resposta: "[ObterInformacoesProvedor]\n" +
			"IdentificacaoProvedor=Nome:PadraoNacional|Versao:1.01\n" +
			"ServicosDisponibilizados=EnviarUnitario|EnviarEvento|ConsultarEvento|\n",
	}}
	rec := chamar(t, muxDe(f), http.MethodGet, "/nfse/municipios/"+munPadraoNacional+"?capacidades=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var resp struct {
		Operacoes   map[string]bool `json:"operacoes"`
		Capacidades Capacidades     `json:"capacidades"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta ilegível: %v", err)
	}
	if !resp.Operacoes["cancelamento"] {
		t.Error("disse que o Padrão Nacional não cancela; cancela por evento")
	}
	if !resp.Operacoes["substituicao"] {
		t.Error("disse que o Padrão Nacional não substitui; substitui pelo grupo subst da DPS")
	}
	if resp.Capacidades.Provedor != "PadraoNacional" {
		t.Errorf("provedor lido da lib = %q", resp.Capacidades.Provedor)
	}
}

// Sem o parâmetro, o endpoint continua sendo consulta de tabela: nada de sessão
// nativa numa rota que hoje responde na hora.
func TestCapacidadesSoQuandoPedidas(t *testing.T) {
	f := &libFake{}
	rec := chamar(t, muxDe(f), http.MethodGet, "/nfse/municipios/"+munPadraoNacional, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "operacoes") {
		t.Errorf("consultou a lib sem ninguém pedir: %s", rec.Body)
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
	// Sem CodigoMunicipio a lib nem lê o XML: quem lê é a classe do provedor, e
	// ela é escolhida pelo município. O pedido não trouxe nenhum, então tem de
	// sair do próprio XML.
	if got := cfgDoTenant(f2.tenant, "CodigoMunicipio"); got != munPadraoNacional {
		t.Errorf("CodigoMunicipio = %q, quero %q: sem ele a lib responde "+
			"\"Nenhum provedor selecionado\"", got, munPadraoNacional)
	}
}

// O município do pedido vence o do XML: é o escape para documento cujo emissor
// a tabela embutida atribui a outro provedor.
func TestPDFPorXMLPrefereOMunicipioDoPedido(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/pdf", map[string]any{
		"xml_b64":   base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
		"municipio": munAbrasf,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if got := cfgDoTenant(f.tenant, "CodigoMunicipio"); got != munAbrasf {
		t.Errorf("CodigoMunicipio = %q, quero %q", got, munAbrasf)
	}
}

// XML sem município identificável para ANTES da lib: recusar aqui diz o que
// falta, enquanto a lib só devolveria "Nenhum provedor selecionado".
func TestPDFPorXMLSemMunicipioRecusaAntesDaLib(t *testing.T) {
	f := &libFake{}
	rec := post(t, muxDe(f), "/nfse/pdf", map[string]any{
		"xml_b64": base64.StdEncoding.EncodeToString([]byte(`<NFSe><infNFSe/></NFSe>`)),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quero 400: %s", rec.Code, rec.Body)
	}
	if f.tenant.Config != nil {
		t.Error("o pedido chegou à lib mesmo sem município")
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
