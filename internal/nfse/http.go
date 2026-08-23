package nfse

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
)

// secaoACBr é a seção de configuração deste documento na ACBrLib.
const secaoACBr = "NFSe"

// Modulo expõe a NFS-e sob /v1/nfse.
//
// É o mais diferente dos três documentos fiscais: multi-provedor (o município
// decide o layout), sem validação de regras de negócio na lib, e com PDF
// recuperável pela chave — coisa que CT-e e MDF-e não têm.
type Modulo struct{ svc acbr.NFSeServico }

// NovoModulo liga o módulo ao binding.
func NovoModulo(svc acbr.NFSeServico) *Modulo { return &Modulo{svc: svc} }

func (m *Modulo) Nome() string { return "nfse" }

func (m *Modulo) Capacidades() []string {
	return []string{
		"xml", "transmissao", "eventos",
		"consulta", "consulta-dps", "consultas", "distribuicao", "pdf", "municipios",
	}
}

func (m *Modulo) Registrar(r modulo.Router) {
	// Fase 1: monta o XML da DPS. Sem certificado, sem rede.
	r.HandleFunc("POST /xml", m.handleXML)
	// Fase 2: assina e transmite. É aqui que o certificado entra.
	r.HandleFunc("POST /transmissao", m.handleTransmissao)
	// Eventos: cancelamento e substituição — uma fase.
	r.HandleFunc("POST /eventos/{tipo}", m.handleEvento)
	// Consultas ao provedor — POST porque levam o certificado no corpo.
	r.HandleFunc("POST /consulta", m.handleConsulta)
	r.HandleFunc("POST /consulta-dps", m.handleConsultaDPS)
	r.HandleFunc("POST /consultas/{tipo}", m.handleConsultas)
	r.HandleFunc("POST /distribuicao", m.handleDistribuicao)
	// PDF: a NFS-e recupera o DANFSE pela CHAVE (ObterDANFSE), diferente de CT-e
	// e MDF-e — perder o XML aqui não é definitivo. Também aceita XML.
	r.HandleFunc("POST /pdf", m.handlePDF)
	// Tabela de municípios: pública quanto a segredo (não leva certificado) e é
	// como o cliente descobre, ANTES de emitir, se o município é atendido.
	r.HandleFunc("GET /municipios/{codigo}", m.handleMunicipio)
}

// --- fase 1: montar ---------------------------------------------------------

// RespostaXML é o retorno da fase 1.
//
// Não há "chave" aqui: a chave de acesso da NFS-e é atribuída pelo provedor na
// autorização. O que a fase 1 entrega é o IDENTIFICADOR DA DPS, que é
// determinístico a partir do pedido — e é ele que recupera uma transmissão
// perdida, via POST /consulta-dps.
type RespostaXML struct {
	IDdps     string           `json:"id_dps,omitempty"`
	XMLBase64 string           `json:"xml_b64"`
	Assinado  bool             `json:"assinado"`
	Layout    string           `json:"layout"`
	Provedor  string           `json:"provedor,omitempty"`
	Validacao fiscal.Validacao `json:"validacao"`
}

func (m *Modulo) handleXML(w http.ResponseWriter, r *http.Request) {
	var p DPSPedido
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	cnpj := fiscal.SoDigitos(fiscal.Primeiro(p.InfDPS.Prest.CNPJ, p.InfDPS.Prest.CPF))
	if cnpj == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "infDPS.prest.CNPJ é obrigatório")
		return
	}
	cmun := MunicipioDoPedido(p)
	layout, ok := m.layout(w, cmun)
	if !ok {
		return
	}

	t := m.tenant(cnpj, cmun, p.InfDPS.Prest, p.Ambiente, layout, fiscal.Certificado{}, Credenciais{})
	xml, val, res, err := fiscal.Montar(m.svc, t, ToINIDoLayout(layout, p))
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if strings.TrimSpace(xml) == "" {
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "xml_nao_montado",
			"a lib não produziu XML a partir do pedido", map[string]any{"resposta": res.Resposta})
		return
	}
	httpx.JSON(w, http.StatusOK, RespostaXML{
		IDdps:     IDdoXML(xml),
		XMLBase64: fiscal.Base64(xml),
		// O provedor assina a DPS dentro do envio — a NFS-e não expõe assinatura
		// separada, ao contrário de CT-e e MDF-e. Dizer a verdade aqui evita que
		// o cliente arquive isto achando que é documento assinado.
		Assinado: false,
		Layout:   string(layout),
		Provedor: provedorDoMunicipio(cmun),
		// Validacao.Suportada virá false: o ACBrNFSeX não exporta validação de
		// regras de negócio (CT-e e MDF-e exportam). Fingir "ok" seria mentir
		// sobre a garantia que o cliente tem em mãos.
		Validacao: val,
	})
}

// --- fase 2: transmitir -----------------------------------------------------

// Credenciais são as credenciais de webservice de provedores não-Padrão
// Nacional (ABRASF e próprios), que exigem login/token além do certificado.
//
// Como o certificado, viajam no payload e não são persistidas.
type Credenciais struct {
	Usuario string `json:"usuario,omitempty"`
	Senha   string `json:"senha,omitempty"`
	Token   string `json:"token,omitempty"`
}

// String redige o conteúdo: senha e token de prefeitura são segredo tanto
// quanto o certificado.
func (c Credenciais) String() string { return "Credenciais{redigido}" }

// PedidoTransmissao é o corpo da fase 2.
type PedidoTransmissao struct {
	XMLBase64 string `json:"xml_b64"`
	// Municipio decide o provedor. A DPS não o carrega em lugar previsível o
	// bastante para extrair do XML com segurança, então é obrigatório aqui.
	Municipio string `json:"municipio"`
	// Ambiente é FALLBACK: sai do tpAmb do XML quando a tag existe.
	Ambiente    string             `json:"ambiente,omitempty"`
	Emitente    Emitente           `json:"emitente"`
	Certificado fiscal.Certificado `json:"certificado"`
	Credenciais Credenciais        `json:"credenciais,omitempty"`
}

// Emitente identifica o prestador para a configuração da sessão. Sem cadastro
// no servidor, ele vem no pedido.
type Emitente struct {
	CNPJ        string `json:"cnpj"`
	InscMun     string `json:"inscricao_municipal,omitempty"`
	RazaoSocial string `json:"razao_social,omitempty"`
}

// RespostaTransmissao é o retorno da fase 2.
type RespostaTransmissao struct {
	Numero            string     `json:"numero,omitempty"`
	Chave             string     `json:"chave,omitempty"`
	CodigoVerificacao string     `json:"codigo_verificacao,omitempty"`
	Protocolo         string     `json:"protocolo,omitempty"`
	Status            string     `json:"status"`
	Situacao          string     `json:"situacao,omitempty"`
	XMLBase64         string     `json:"xml_b64,omitempty"`
	Erros             []Mensagem `json:"erros,omitempty"`
	Alertas           []Mensagem `json:"alertas,omitempty"`
}

func (m *Modulo) handleTransmissao(w http.ResponseWriter, r *http.Request) {
	var p PedidoTransmissao
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	xml, ok := fiscal.XMLdeBase64(w, "xml_b64", p.XMLBase64)
	if !ok {
		return
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return
	}
	layout, ok := m.layout(w, p.Municipio)
	if !ok {
		return
	}

	ambiente := fiscal.Primeiro(AmbienteDoXML(xml), p.Ambiente, "homologacao")
	t := m.tenantEmitente(p.Emitente, p.Municipio, ambiente, layout, p.Certificado, p.Credenciais)

	res, err := m.svc.Transmitir(t, xml)
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, res.Resposta, "emissão") {
		return
	}

	e := ParseEnvio(res.Resposta)
	resp := RespostaTransmissao{
		Numero: e.Numero, Chave: e.Chave, CodigoVerificacao: e.CodigoVerificacao,
		Protocolo: e.Protocolo, Situacao: e.Situacao,
		Status:    statusEmissao(e),
		XMLBase64: fiscal.Base64(res.XML),
		Erros:     e.Erros, Alertas: e.Alertas,
	}
	httpx.JSON(w, fiscal.StatusDoDesfecho(resp.Status), resp)
}

// statusEmissao traduz a Emissao no vocabulário comum aos módulos.
func statusEmissao(e Emissao) string {
	switch {
	case e.Sucesso:
		return "autorizado"
	case len(e.Erros) > 0:
		return "rejeitado"
	default:
		return "erro"
	}
}

// --- eventos ----------------------------------------------------------------

// PedidoEvento é o envelope dos eventos da NFS-e.
type PedidoEvento struct {
	Chave       string             `json:"chave"`
	Municipio   string             `json:"municipio"`
	Ambiente    string             `json:"ambiente,omitempty"`
	Emitente    Emitente           `json:"emitente"`
	Certificado fiscal.Certificado `json:"certificado"`
	Credenciais Credenciais        `json:"credenciais,omitempty"`
	Evento      json.RawMessage    `json:"evento,omitempty"`
}

// RespostaEvento é o retorno de um evento.
type RespostaEvento struct {
	Tipo      string     `json:"tipo"`
	Chave     string     `json:"chave,omitempty"`
	Status    string     `json:"status"`
	Protocolo string     `json:"protocolo,omitempty"`
	DataHora  string     `json:"data_hora,omitempty"`
	XMLBase64 string     `json:"xml_b64,omitempty"`
	Erros     []Mensagem `json:"mensagens,omitempty"`
	Alertas   []Mensagem `json:"alertas,omitempty"`
}

func (m *Modulo) handleEvento(w http.ResponseWriter, r *http.Request) {
	tipo := r.PathValue("tipo")
	if tipo != "cancelamento" && tipo != "substituicao" {
		httpx.ErroJSON(w, http.StatusNotFound, "evento_desconhecido",
			"evento '"+tipo+"' não existe para NFS-e; consulte GET /v1/capacidades")
		return
	}
	var p PedidoEvento
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return
	}
	layout, ok := m.layout(w, p.Municipio)
	if !ok {
		return
	}
	t := m.tenantEmitente(p.Emitente, p.Municipio, fiscal.Primeiro(p.Ambiente, "homologacao"),
		layout, p.Certificado, p.Credenciais)

	if tipo == "substituicao" {
		m.substituir(w, t, layout, p)
		return
	}

	if strings.TrimSpace(p.Chave) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "chave da NFS-e é obrigatória")
		return
	}
	var e CancelamentoPedido
	if msg := fiscal.DecodarAninhado("evento", p.Evento, &e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}
	res, err := m.svc.Cancelar(t, ToINICancelamento(p.Chave, p.Municipio, e))
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, res.Resposta, "cancelamento") {
		return
	}
	c := ParseCancelamento(res.Resposta)
	httpx.JSON(w, fiscal.StatusDoEvento(StatusCancelamento(c)), RespostaEvento{
		Tipo: tipo, Chave: p.Chave, Status: StatusCancelamento(c),
		Protocolo: c.Protocolo, DataHora: c.DataHora,
		XMLBase64: fiscal.Base64(res.XML),
		Erros:     c.Erros, Alertas: c.Alertas,
	})
}

// substituir emite a DPS substituta identificando a NFS-e antiga.
func (m *Modulo) substituir(w http.ResponseWriter, t acbr.TenantConfig, layout Layout, p PedidoEvento) {
	var e SubstituicaoPedido
	if msg := fiscal.DecodarAninhado("evento", p.Evento, &e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}
	if strings.TrimSpace(e.Substituida.Numero) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"evento.substituida.numero (NFS-e a substituir) é obrigatório")
		return
	}
	sub := acbr.SubstituicaoNFSe{
		NumeroNFSe:         e.Substituida.Numero,
		SerieNFSe:          e.Substituida.Serie,
		CodigoVerificacao:  e.Substituida.CodigoVerificacao,
		NumeroLote:         e.Substituida.NumeroLote,
		CodigoCancelamento: fiscal.Primeiro(e.Codigo, "1"),
		MotivoCancelamento: e.Motivo,
	}
	res, err := m.svc.SubstituirNFSe(t, ToINIDoLayout(layout, e.DPS), sub)
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, res.Resposta, "substituição") {
		return
	}
	em := ParseEnvio(res.Resposta)
	httpx.JSON(w, fiscal.StatusDoDesfecho(statusEmissao(em)), RespostaEvento{
		Tipo: "substituicao", Chave: em.Chave, Status: statusEmissao(em),
		Protocolo: em.Protocolo, XMLBase64: fiscal.Base64(res.XML),
		Erros: em.Erros, Alertas: em.Alertas,
	})
}

// --- consultas --------------------------------------------------------------

// PedidoConsulta é o envelope das consultas ao provedor.
type PedidoConsulta struct {
	Chave       string             `json:"chave,omitempty"`
	Municipio   string             `json:"municipio"`
	Ambiente    string             `json:"ambiente,omitempty"`
	Emitente    Emitente           `json:"emitente"`
	Certificado fiscal.Certificado `json:"certificado"`
	Credenciais Credenciais        `json:"credenciais,omitempty"`
	// Parâmetros das consultas específicas (POST /consultas/{tipo}).
	Numero            string `json:"numero,omitempty"`
	NumeroFinal       string `json:"numero_final,omitempty"`
	Serie             string `json:"serie,omitempty"`
	Tipo              string `json:"tipo,omitempty"`
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	Protocolo         string `json:"protocolo,omitempty"`
	NumeroLote        string `json:"numero_lote,omitempty"`
	Pagina            int    `json:"pagina,omitempty"`
	NSU               int    `json:"nsu,omitempty"`
}

func (m *Modulo) handleConsulta(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(p.Chave) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "chave é obrigatória")
		return
	}
	res, err := m.svc.Consultar(t, p.Chave)
	m.responderConsulta(w, res, err)
}

// handleConsultaDPS é a recuperação de uma transmissão de desfecho desconhecido.
//
// Depois de um timeout na fase 2, é ISTO que se chama — nunca reenviar a DPS.
// Aceita o id_dps como a fase 1 devolveu (com o prefixo "DPS") ou a chave nua: o
// webservice quer a chave sem prefixo, e essa diferença não é problema do cliente.
func (m *Modulo) handleConsultaDPS(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	chave := ChaveDPS(p.Chave)
	if chave == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"chave (o id_dps devolvido pela fase 1) é obrigatória")
		return
	}
	res, err := m.svc.ConsultarDPSPorChave(t, chave)
	m.responderConsulta(w, res, err)
}

func (m *Modulo) handleConsultas(w http.ResponseWriter, r *http.Request) {
	tipo := r.PathValue("tipo")
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	pagina := p.Pagina
	if pagina <= 0 {
		pagina = 1
	}
	switch tipo {
	case "numero":
		if p.Numero == "" {
			httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "numero é obrigatório")
			return
		}
		res, err := m.svc.ConsultarPorNumero(t, p.Numero, pagina)
		m.responderConsulta(w, res, err)
	case "faixa":
		if p.Numero == "" || p.NumeroFinal == "" {
			httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "numero e numero_final são obrigatórios")
			return
		}
		res, err := m.svc.ConsultarPorFaixa(t, p.Numero, p.NumeroFinal, pagina)
		m.responderConsulta(w, res, err)
	case "rps":
		if p.Numero == "" {
			httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "numero (do RPS) é obrigatório")
			return
		}
		res, err := m.svc.ConsultarPorRps(t, p.Numero, p.Serie, p.Tipo, p.CodigoVerificacao)
		m.responderConsulta(w, res, err)
	case "situacao":
		res, err := m.svc.ConsultarSituacao(t, p.Protocolo, p.NumeroLote)
		m.responderConsulta(w, res, err)
	case "lote-rps":
		res, err := m.svc.ConsultarLoteRps(t, p.Protocolo, p.NumeroLote)
		m.responderConsulta(w, res, err)
	default:
		httpx.ErroJSON(w, http.StatusNotFound, "consulta_desconhecida",
			"consulta '"+tipo+"' não existe; use numero, faixa, rps, situacao ou lote-rps")
	}
}

// handleDistribuicao consulta a Distribuição DF-e do ADN por NSU.
//
// O cursor (NSU) vai e volta no payload: sem estado no servidor, é o cliente que
// guarda onde parou. Não paralelize a distribuição do mesmo CNPJ — sem o lock
// que existia com banco, chamadas simultâneas embaralham o cursor.
func (m *Modulo) handleDistribuicao(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	res, err := m.svc.ConsultarDFe(t, p.NSU)
	m.responderConsulta(w, res, err)
}

func (m *Modulo) lerConsulta(w http.ResponseWriter, r *http.Request) (PedidoConsulta, acbr.TenantConfig, bool) {
	var p PedidoConsulta
	if !httpx.LerJSON(w, r, &p) {
		return p, acbr.TenantConfig{}, false
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return p, acbr.TenantConfig{}, false
	}
	layout, ok := m.layout(w, p.Municipio)
	if !ok {
		return p, acbr.TenantConfig{}, false
	}
	t := m.tenantEmitente(p.Emitente, p.Municipio, fiscal.Primeiro(p.Ambiente, "homologacao"),
		layout, p.Certificado, p.Credenciais)
	return p, t, true
}

func (m *Modulo) responderConsulta(w http.ResponseWriter, res acbr.Result, err error) {
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, res.Resposta, "esta consulta") {
		return
	}
	corpo := map[string]any{"codigo": res.Codigo, "resposta": res.Resposta}
	if res.XML != "" {
		corpo["xml_b64"] = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, http.StatusOK, corpo)
}

// --- DANFSE -----------------------------------------------------------------

// PedidoPDF gera o DANFSE. Diferente de CT-e e MDF-e, a NFS-e recupera o PDF
// pela CHAVE (ObterDANFSE no provedor) — então perder o XML não é definitivo.
// Por chave a operação fala com o provedor e exige certificado; por XML é render
// local e não exige.
type PedidoPDF struct {
	Chave       string             `json:"chave,omitempty"`
	XMLBase64   string             `json:"xml_b64,omitempty"`
	Municipio   string             `json:"municipio,omitempty"`
	Ambiente    string             `json:"ambiente,omitempty"`
	Emitente    Emitente           `json:"emitente,omitempty"`
	Certificado fiscal.Certificado `json:"certificado,omitempty"`
	Credenciais Credenciais        `json:"credenciais,omitempty"`
}

func (m *Modulo) handlePDF(w http.ResponseWriter, r *http.Request) {
	var p PedidoPDF
	if !httpx.LerJSON(w, r, &p) {
		return
	}

	var res acbr.Result
	var err error
	switch {
	case strings.TrimSpace(p.XMLBase64) != "":
		xml, ok := fiscal.XMLdeBase64(w, "xml_b64", p.XMLBase64)
		if !ok {
			return
		}
		// Render local: não fala com o provedor, então não pede certificado.
		res, err = m.svc.RenderizarPDF(fiscal.Tenant("", secaoACBr, "", fiscal.Certificado{}), xml)
	case strings.TrimSpace(p.Chave) != "":
		if errCert := p.Certificado.Validar(); errCert != nil {
			httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", errCert.Error())
			return
		}
		layout, ok := m.layout(w, p.Municipio)
		if !ok {
			return
		}
		t := m.tenantEmitente(p.Emitente, p.Municipio, fiscal.Primeiro(p.Ambiente, "homologacao"),
			layout, p.Certificado, p.Credenciais)
		res, err = m.svc.ObterPDF(t, p.Chave)
	default:
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "informe chave ou xml_b64")
		return
	}

	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, res.Resposta, "geração do DANFSE") {
		return
	}
	if len(res.PDF) == 0 {
		httpx.ErroJSON(w, http.StatusUnprocessableEntity, "pdf_nao_gerado", "a lib não devolveu PDF")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"pdf_b64": base64.StdEncoding.EncodeToString(res.PDF)})
}

// --- tabela de municípios ---------------------------------------------------

// handleMunicipio diz se um município é atendido e por qual provedor. É como o
// cliente descobre isso ANTES de montar um documento que seria recusado.
func (m *Modulo) handleMunicipio(w http.ResponseWriter, r *http.Request) {
	codigo := fiscal.SoDigitos(r.PathValue("codigo"))
	layout, ok := LayoutDoMunicipio(codigo)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"codigo":    codigo,
		"provedor":  provedorDoMunicipio(codigo),
		"layout":    string(layout),
		"suportado": ok,
	})
}

// --- apoio ------------------------------------------------------------------

// layout resolve o município ou responde 422. Recusar aqui é melhor do que
// mandar um XML que a prefeitura rejeitaria com mensagem obscura.
func (m *Modulo) layout(w http.ResponseWriter, cmun string) (Layout, bool) {
	c := fiscal.SoDigitos(cmun)
	if c == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"município (código IBGE) é obrigatório: é ele que decide o provedor de NFS-e")
		return "", false
	}
	l, ok := LayoutDoMunicipio(c)
	if !ok {
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "provedor_nao_suportado",
			"o município "+c+" não tem provedor de NFS-e conhecido nesta versão da tabela",
			map[string]any{"municipio": c})
		return "", false
	}
	return l, true
}

// tenant monta a sessão a partir do prestador do próprio pedido (fase 1).
func (m *Modulo) tenant(cnpj, cmun string, prest Pessoa, ambiente string, layout Layout,
	cert fiscal.Certificado, cred Credenciais) acbr.TenantConfig {
	return m.tenantEmitente(Emitente{
		CNPJ: cnpj, InscMun: prest.IM, RazaoSocial: prest.XNome,
	}, cmun, ambiente, layout, cert, cred)
}

// tenantEmitente monta a sessão nativa. O emitente vem do pedido porque não há
// cadastro no servidor — é a consequência direta de não persistir nada.
func (m *Modulo) tenantEmitente(e Emitente, cmun, ambiente string, layout Layout,
	cert fiscal.Certificado, cred Credenciais) acbr.TenantConfig {

	cnpj := fiscal.SoDigitos(e.CNPJ)
	t := fiscal.Tenant(cnpj, secaoACBr, ambiente, cert)
	t.Config = append(t.Config,
		acbr.ConfigKV{Section: secaoACBr, Key: "CodigoMunicipio", Value: fiscal.SoDigitos(cmun)},
		acbr.ConfigKV{Section: secaoACBr, Key: "Emitente.CNPJ", Value: cnpj},
	)
	if e.InscMun != "" {
		t.Config = append(t.Config, acbr.ConfigKV{Section: secaoACBr, Key: "Emitente.InscMun", Value: e.InscMun})
	}
	if e.RazaoSocial != "" {
		t.Config = append(t.Config, acbr.ConfigKV{Section: secaoACBr, Key: "Emitente.RazSocial", Value: e.RazaoSocial})
	}
	// Só provedores não-Padrão Nacional usam login/token de prefeitura.
	if layout != LayoutPadraoNacional {
		for _, kv := range []struct{ k, v string }{
			{"Emitente.WSUser", cred.Usuario},
			{"Emitente.WSSenha", cred.Senha},
			{"Emitente.WSChaveAcesso", cred.Token},
		} {
			if kv.v != "" {
				t.Config = append(t.Config, acbr.ConfigKV{Section: secaoACBr, Key: kv.k, Value: kv.v})
			}
		}
	}
	return t
}

// naoSuportada mapeia o erro nativo de "provedor não implementa" num 422 claro.
//
// A capacidade de cada provedor é descoberta em RUNTIME: não existe tabela
// dizendo o que cada município aceita, e cancelamento/substituição não existem
// em todos. O marcador é a mensagem da lib.
func (m *Modulo) naoSuportada(w http.ResponseWriter, resposta, operacao string) bool {
	if !OperacaoNaoSuportada(resposta) {
		return false
	}
	httpx.ErroJSON(w, http.StatusUnprocessableEntity, "operacao_nao_suportada",
		"o provedor de NFS-e deste município não oferece "+operacao+" por webservice")
	return true
}

func (m *Modulo) responderErro(w http.ResponseWriter, res acbr.Result, err error) {
	fiscal.ResponderErroDaLib(w, "NFS-e", res, err)
}
