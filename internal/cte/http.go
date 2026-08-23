package cte

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
const secaoACBr = "CTe"

// Modulo expõe o CT-e sob /v1/cte. É a única parte deste pacote que conhece
// HTTP — o resto é tradução JSON→INI e leitura de resposta.
type Modulo struct{ svc acbr.CTeServico }

// NovoModulo liga o módulo ao binding.
func NovoModulo(svc acbr.CTeServico) *Modulo { return &Modulo{svc: svc} }

func (m *Modulo) Nome() string { return "cte" }

func (m *Modulo) Capacidades() []string {
	return []string{
		"xml", "transmissao", "simplificado", "eventos",
		"consulta", "status-servico", "cadastro", "pdf",
	}
}

func (m *Modulo) Registrar(r modulo.Router) {
	// Fase 1: monta e valida. Sem certificado, sem rede.
	r.HandleFunc("POST /xml", m.handleXML)
	r.HandleFunc("POST /simp/xml", m.handleXMLSimp)
	// Fase 2: assina e transmite. É aqui que o certificado entra — e o
	// Simplificado NÃO tem rota própria: transmitir é a mesma operação, e o
	// XML já diz o que é. Dois caminhos idênticos seriam dois para manter.
	r.HandleFunc("POST /transmissao", m.handleTransmissao)
	// Eventos: uma fase só (a lib não expõe o XML do evento antes de enviá-lo).
	r.HandleFunc("POST /eventos/{tipo}", m.handleEvento)
	// Consultas ao webservice — todas POST porque levam o certificado no corpo.
	r.HandleFunc("POST /consulta", m.handleConsulta)
	r.HandleFunc("POST /status-servico", m.handleStatusServico)
	r.HandleFunc("POST /cadastro", m.handleCadastro)
	// NÃO existe rota de recibo: a recepção assíncrona foi desativada pela
	// SEFAZ. O CTE_ConsultarRecibo bate no CTeRetRecepcao, que não atende mais —
	// expor a rota seria oferecer um caminho que só sabe falhar.
	// Render local: não fala com a SEFAZ, então não pede certificado.
	r.HandleFunc("POST /pdf", m.handlePDF)
	r.HandleFunc("POST /pdf/evento", m.handlePDFEvento)
}

// --- fase 1: montar e validar ----------------------------------------------

// RespostaXML é o retorno da fase 1.
type RespostaXML struct {
	Chave     string           `json:"chave,omitempty"`
	XMLBase64 string           `json:"xml_b64"`
	Assinado  bool             `json:"assinado"`
	Validacao fiscal.Validacao `json:"validacao"`
}

func (m *Modulo) handleXML(w http.ResponseWriter, r *http.Request) {
	var p PedidoEmissao
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	cnpj := fiscal.SoDigitos(fiscal.Primeiro(p.InfCte.Emit.CNPJ, p.InfCte.Emit.CPF))
	if cnpj == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "infCte.emit.CNPJ é obrigatório")
		return
	}
	m.montar(w, fiscal.Tenant(cnpj, secaoACBr, p.Ambiente, fiscal.Certificado{}), ToINI(p))
}

func (m *Modulo) handleXMLSimp(w http.ResponseWriter, r *http.Request) {
	var p PedidoSimp
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	cnpj := fiscal.SoDigitos(fiscal.Primeiro(p.InfCte.Emit.CNPJ, p.InfCte.Emit.CPF))
	if cnpj == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "infCte.emit.CNPJ é obrigatório")
		return
	}
	m.montar(w, fiscal.Tenant(cnpj, secaoACBr, p.Ambiente, fiscal.Certificado{}), ToINISimp(p))
}

// montar é o corpo comum da fase 1 deste módulo: delega a fiscal.Montar (que é
// igual para todo documento) e monta a resposta do CT-e.
func (m *Modulo) montar(w http.ResponseWriter, t acbr.TenantConfig, ini string) {
	xml, val, res, err := fiscal.Montar(m.svc, t, ini)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}
	if strings.TrimSpace(xml) == "" {
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "xml_nao_montado",
			"a lib não produziu XML a partir do pedido", map[string]any{"resposta": res.Resposta})
		return
	}

	resp := RespostaXML{
		Chave:     ChaveDoXML(xml),
		XMLBase64: fiscal.Base64(xml),
		// A ACBrLib assina no envio, não na montagem. Dizer a verdade aqui
		// evita que o cliente arquive isto achando que é documento assinado.
		Assinado:  false,
		Validacao: val,
	}
	if !resp.Validacao.OK {
		// 422 e não 200: o pedido está inválido. O XML vai junto de propósito —
		// é com ele na mão que o cliente enxerga o que a lib entendeu.
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "regras_de_negocio",
			"o CT-e não passou nas regras de negócio; nada foi transmitido", resp)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// --- fase 2: transmitir -----------------------------------------------------

// PedidoTransmissao é o corpo da fase 2.
type PedidoTransmissao struct {
	XMLBase64 string `json:"xml_b64"`
	// Ambiente é FALLBACK. O ambiente sai do tpAmb do próprio XML; este campo
	// só é usado se o XML não trouxer a tag.
	Ambiente    string             `json:"ambiente,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
}

// RespostaTransmissao é o retorno da fase 2.
type RespostaTransmissao struct {
	Chave     string `json:"chave,omitempty"`
	Protocolo string `json:"protocolo,omitempty"`
	Status    string `json:"status"`
	CStat     string `json:"cstat,omitempty"`
	Motivo    string `json:"motivo,omitempty"`
	// Recibo é vestígio: com a recepção síncrona ele não vem. Fica porque a
	// própria lib ainda trata o caso de um autorizador responder cStat=103
	// (ACBrLibCTeBase.pas:948) — se isso acontecer, o cliente vê em vez de
	// receber uma resposta sem explicação.
	Recibo        string     `json:"recibo,omitempty"`
	XMLProcBase64 string     `json:"xml_proc_b64,omitempty"`
	Erros         []Mensagem `json:"erros,omitempty"`
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

	chave := ChaveDoXML(xml)
	// O ambiente vem do XML, não do cliente: divergir dá rejeição 252 — ou,
	// pior, manda um documento de homologação para o webservice de produção.
	ambiente := fiscal.Primeiro(AmbienteDoXML(xml), p.Ambiente, "homologacao")
	t := fiscal.Tenant(fiscal.CNPJDaChave(chave), secaoACBr, ambiente, p.Certificado)

	res, err := m.svc.Transmitir(t, xml)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}

	e := ParseEnvio(res.Resposta)
	if e.Chave == "" {
		e.Chave = chave // a lib nem sempre repete a chave na rejeição
	}
	resp := RespostaTransmissao{
		Chave: e.Chave, Protocolo: e.Protocolo, Status: StatusEmissao(e),
		CStat: e.CStat, Motivo: e.XMotivo, Recibo: e.Recibo, Erros: e.Erros,
	}
	if res.XML != "" {
		resp.XMLProcBase64 = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, fiscal.StatusDoDesfecho(resp.Status), resp)
}

// --- eventos ----------------------------------------------------------------

// PedidoEvento é o envelope de qualquer evento do CT-e. Os campos próprios de
// cada tipo vão aninhados em "evento" — assim os dois níveis podem recusar
// campo desconhecido, que é como o cliente descobre que errou um nome.
type PedidoEvento struct {
	Chave       string             `json:"chave"`
	Protocolo   string             `json:"protocolo,omitempty"`
	Ambiente    string             `json:"ambiente,omitempty"`
	DhEvento    string             `json:"dhEvento,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
	Evento      json.RawMessage    `json:"evento,omitempty"`
}

// RespostaEvento é o retorno de um evento.
type RespostaEvento struct {
	Tipo            string     `json:"tipo"`
	Chave           string     `json:"chave"`
	Status          string     `json:"status"`
	CStat           string     `json:"cstat,omitempty"`
	Motivo          string     `json:"motivo,omitempty"`
	Protocolo       string     `json:"protocolo,omitempty"`
	XMLEventoBase64 string     `json:"xml_evento_b64,omitempty"`
	Erros           []Mensagem `json:"erros,omitempty"`
}

func (m *Modulo) handleEvento(w http.ResponseWriter, r *http.Request) {
	tipo := r.PathValue("tipo")
	var p PedidoEvento
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	chave, ok := fiscal.ChaveValida(w, p.Chave)
	if !ok {
		return
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return
	}

	cnpj := fiscal.CNPJDaChave(chave)
	dh := fiscal.Primeiro(p.DhEvento, fiscal.AgoraLocal())

	ini, envia, errMsg := m.prepararEvento(tipo, chave, cnpj, dh, p)
	if errMsg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", errMsg)
		return
	}
	if envia == nil {
		httpx.ErroJSON(w, http.StatusNotFound, "evento_desconhecido",
			"evento '"+tipo+"' não existe para CT-e; consulte GET /v1/capacidades")
		return
	}

	t := fiscal.Tenant(cnpj, secaoACBr, fiscal.Primeiro(p.Ambiente, "homologacao"), p.Certificado)
	res, err := envia(t, ini)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}

	c := ParseCancelamento(res.Resposta) // leitura genérica de retorno de evento
	resp := RespostaEvento{
		Tipo: tipo, Chave: chave, Status: StatusCancelamento(c),
		CStat: c.CStat, Motivo: c.XMotivo, Protocolo: c.Protocolo, Erros: c.Erros,
	}
	if res.XML != "" {
		resp.XMLEventoBase64 = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, fiscal.StatusDoEvento(resp.Status), resp)
}

// prepararEvento monta o INI do tipo pedido e escolhe o método de envio.
// errMsg != "" é erro do cliente; envia == nil é tipo inexistente.
func (m *Modulo) prepararEvento(tipo, chave, cnpj, dh string, p PedidoEvento) (
	ini string, envia func(acbr.TenantConfig, string) (acbr.Result, error), errMsg string) {

	// Genérico cobre EPEC, comprovante, insucesso e desacordo; cancelamento e
	// CC-e têm método próprio na lib.
	generico := m.svc.EnviarEvento

	switch tipo {
	case "cancelamento":
		var e PedidoCancelamento
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if strings.TrimSpace(e.Justificativa) == "" {
			return "", nil, "evento.justificativa é obrigatória"
		}
		return ToINICancelamento(chave, cnpj, p.Protocolo, dh, e), m.svc.Cancelar, ""

	case "carta-correcao":
		var e PedidoCartaCorrecao
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if len(e.Correcoes) == 0 {
			return "", nil, "informe ao menos uma correção em evento.correcoes"
		}
		return ToINICartaCorrecao(chave, cnpj, dh, e), m.svc.CartaCorrecao, ""

	case "epec":
		var e PedidoEPEC
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		return ToINIEPEC(chave, cnpj, dh, e), generico, ""

	case "comprovante-entrega":
		var e PedidoComprovanteEntrega
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if len(e.Documentos) == 0 {
			return "", nil, "informe ao menos uma chave de NF-e em evento.documentos"
		}
		return ToINIComprovanteEntrega(chave, cnpj, dh, e), generico, ""

	case "insucesso-entrega":
		var e PedidoInsucessoEntrega
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if len(e.Documentos) == 0 {
			return "", nil, "informe ao menos uma chave de NF-e em evento.documentos"
		}
		return ToINIInsucessoEntrega(chave, cnpj, dh, e), generico, ""

	case "prestacao-desacordo":
		var e PedidoDesacordo
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if strings.TrimSpace(e.XObs) == "" {
			return "", nil, "evento.xObs (motivo do desacordo) é obrigatório"
		}
		return ToINIDesacordo(chave, cnpj, dh, e), generico, ""
	}
	return "", nil, ""
}

// --- consultas --------------------------------------------------------------

// PedidoConsulta consulta um CT-e pela chave.
type PedidoConsulta struct {
	Chave       string             `json:"chave"`
	Ambiente    string             `json:"ambiente,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
}

// handleConsulta é a recuperação de uma transmissão de desfecho desconhecido:
// depois de um timeout, é ISTO que se chama — nunca repetir a transmissão.
func (m *Modulo) handleConsulta(w http.ResponseWriter, r *http.Request) {
	var p PedidoConsulta
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	chave, ok := fiscal.ChaveValida(w, p.Chave)
	if !ok {
		return
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return
	}
	t := fiscal.Tenant(fiscal.CNPJDaChave(chave), secaoACBr, fiscal.Primeiro(p.Ambiente, "homologacao"), p.Certificado)
	res, err := m.svc.Consultar(t, chave)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}
	e := ParseEnvio(res.Resposta)
	resp := RespostaTransmissao{
		Chave: fiscal.Primeiro(e.Chave, chave), Protocolo: e.Protocolo, Status: StatusEmissao(e),
		CStat: e.CStat, Motivo: e.XMotivo, Erros: e.Erros,
	}
	if res.XML != "" {
		resp.XMLProcBase64 = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// PedidoSefaz é o corpo das consultas que não têm chave.
type PedidoSefaz struct {
	Ambiente    string             `json:"ambiente,omitempty"`
	CNPJ        string             `json:"cnpj,omitempty"`
	UF          string             `json:"uf,omitempty"`
	Documento   string             `json:"documento,omitempty"`
	EhIE        bool               `json:"eh_ie,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
}

func (m *Modulo) handleStatusServico(w http.ResponseWriter, r *http.Request) {
	_, t, ok := m.lerPedidoSefaz(w, r)
	if !ok {
		return
	}
	res, err := m.svc.StatusServico(t)
	m.responderCru(w, res, err)
}

func (m *Modulo) handleCadastro(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerPedidoSefaz(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(p.UF) == "" || strings.TrimSpace(p.Documento) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "uf e documento são obrigatórios")
		return
	}
	res, err := m.svc.ConsultaCadastro(t, p.UF, p.Documento, p.EhIE)
	m.responderCru(w, res, err)
}

func (m *Modulo) lerPedidoSefaz(w http.ResponseWriter, r *http.Request) (PedidoSefaz, acbr.TenantConfig, bool) {
	var p PedidoSefaz
	if !httpx.LerJSON(w, r, &p) {
		return p, acbr.TenantConfig{}, false
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return p, acbr.TenantConfig{}, false
	}
	t := fiscal.Tenant(fiscal.SoDigitos(p.CNPJ), secaoACBr, fiscal.Primeiro(p.Ambiente, "homologacao"), p.Certificado)
	return p, t, true
}

func (m *Modulo) responderCru(w http.ResponseWriter, res acbr.Result, err error) {
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}
	corpo := map[string]any{"codigo": res.Codigo, "resposta": res.Resposta}
	if res.XML != "" {
		corpo["xml_b64"] = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, http.StatusOK, corpo)
}

// --- representação gráfica --------------------------------------------------

// PedidoPDF gera o DACTE a partir do XML autorizado. Não fala com a SEFAZ, logo
// não pede certificado.
//
// É a rede de segurança do modelo sem estado: o CT-e não tem "obter PDF pela
// chave" — se o cliente perder o XML, o DACTE não se regenera de lugar nenhum.
type PedidoPDF struct {
	XMLBase64       string `json:"xml_b64"`
	XMLEventoBase64 string `json:"xml_evento_b64,omitempty"`
}

func (m *Modulo) handlePDF(w http.ResponseWriter, r *http.Request) {
	var p PedidoPDF
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	xml, ok := fiscal.XMLdeBase64(w, "xml_b64", p.XMLBase64)
	if !ok {
		return
	}
	res, err := m.svc.RenderizarPDF(fiscal.Tenant("", secaoACBr, "", fiscal.Certificado{}), xml)
	m.responderPDF(w, res, err)
}

func (m *Modulo) handlePDFEvento(w http.ResponseWriter, r *http.Request) {
	var p PedidoPDF
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	xml, ok := fiscal.XMLdeBase64(w, "xml_b64", p.XMLBase64)
	if !ok {
		return
	}
	evt, err := base64.StdEncoding.DecodeString(strings.TrimSpace(p.XMLEventoBase64))
	if err != nil || len(evt) == 0 {
		httpx.ErroJSON(w, http.StatusBadRequest, "xml_invalido", "xml_evento_b64 é obrigatório e deve ser base64")
		return
	}
	res, err := m.svc.SalvarEventoPDF(fiscal.Tenant("", secaoACBr, "", fiscal.Certificado{}), xml, string(evt))
	m.responderPDF(w, res, err)
}

func (m *Modulo) responderPDF(w http.ResponseWriter, res acbr.Result, err error) {
	if err != nil {
		fiscal.ResponderErroDaLib(w, "CT-e", res, err)
		return
	}
	if len(res.PDF) == 0 {
		httpx.ErroJSON(w, http.StatusUnprocessableEntity, "pdf_nao_gerado", "a lib não devolveu PDF")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"pdf_b64": base64.StdEncoding.EncodeToString(res.PDF)})
}

// --- apoio ------------------------------------------------------------------
