package mdfe

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
)

// secaoACBr é a seção de configuração deste documento na ACBrLib.
const secaoACBr = "MDFe"

// Modulo expõe o MDF-e sob /v1/mdfe. É a única parte deste pacote que conhece
// HTTP: o resto é tradução JSON→INI e leitura de resposta.
type Modulo struct{ svc acbr.MDFeServico }

// NovoModulo liga o módulo ao binding.
func NovoModulo(svc acbr.MDFeServico) *Modulo { return &Modulo{svc: svc} }

func (m *Modulo) Nome() string { return "mdfe" }

func (m *Modulo) Capacidades() []string {
	return []string{
		"xml", "transmissao", "eventos",
		"consulta", "status-servico", "nao-encerrados", "pdf",
	}
}

func (m *Modulo) Registrar(r modulo.Router) {
	// Gerar: monta o XML e valida. Sem certificado, sem rede.
	r.HandleFunc("POST /xml", m.handleXML)
	// Transmitir: assina e envia. É aqui que o certificado entra.
	r.HandleFunc("POST /transmissao", m.handleTransmissao)
	// Eventos: chamada única (a lib não expõe o XML do evento antes de enviá-lo).
	r.HandleFunc("POST /eventos/{tipo}", m.handleEvento)
	// Consultas ao webservice: POST porque levam o certificado no corpo.
	r.HandleFunc("POST /consulta", m.handleConsulta)
	r.HandleFunc("POST /status-servico", m.handleStatusServico)
	r.HandleFunc("POST /nao-encerrados", m.handleNaoEncerrados)
	// Render local: não fala com a SEFAZ, então não pede certificado.
	r.HandleFunc("POST /pdf", m.handlePDF)
	r.HandleFunc("POST /pdf/evento", m.handlePDFEvento)
	// Como no CT-e, NÃO há rota de recibo. Aqui é ainda mais categórico: o
	// ACBr força o envio síncrono sem sequer olhar o parâmetro
	// (ACBrMDFeWebServices.pas:764, com a nota "a partir de 01/07/2024 todos os
	// MDF-e devem ser enviados no modo síncrono").
}

// --- gerar o XML ----------------------------------------------

// RespostaXML é o retorno da geração.
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
	cnpj := fiscal.SoDigitos(fiscal.Primeiro(p.InfMDFe.Emit.CNPJ, p.InfMDFe.Emit.CPF))
	if cnpj == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "infMDFe.emit.CNPJ é obrigatório")
		return
	}

	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, p.InfMDFe.Ide.TpAmb, "infMDFe.ide.tpAmb") || !fiscal.ModalSuportado(w, modalTexto(p.InfMDFe.Ide.Modal), modalRodoviario) {
		return
	}
	t := fiscal.Tenant(cnpj, secaoACBr, p.Ambiente, fiscal.Certificado{})
	xml, val, res, err := fiscal.Montar(m.svc, t, ToINI(p))
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
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
		// A ACBrLib assina no envio, não na montagem.
		Assinado:  false,
		Validacao: val,
	}
	if !resp.Validacao.OK {
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "regras_de_negocio",
			"o MDF-e não passou nas regras de negócio; nada foi transmitido", resp)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// --- transmitir -----------------------------------------------------

// PedidoTransmissao é o corpo da transmissão.
type PedidoTransmissao struct {
	XMLBase64 string `json:"xml_b64"`
	// Ambiente é FALLBACK. O ambiente sai do tpAmb do próprio XML; este campo
	// só é usado se o XML não trouxer a tag.
	Ambiente    string             `json:"ambiente,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
}

// RespostaTransmissao é o retorno da transmissão.
type RespostaTransmissao struct {
	Chave     string `json:"chave,omitempty"`
	Protocolo string `json:"protocolo,omitempty"`
	Status    string `json:"status"`
	CStat     string `json:"cstat,omitempty"`
	Motivo    string `json:"motivo,omitempty"`
	// Recibo é vestígio: o envio é síncrono e ele não vem. Fica porque a lib
	// ainda trata um autorizador que responda cStat=103, se acontecer, o
	// cliente vê em vez de receber uma resposta sem explicação.
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

	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	chave := ChaveDoXML(xml)
	// O ambiente vem do XML, não do cliente. Divergir dá rejeição 252, ou,
	// pior, manda um documento de homologação para o webservice de produção.
	ambiente := fiscal.Primeiro(fiscal.AmbienteDoXML(xml), p.Ambiente)
	t := fiscal.Tenant(fiscal.CNPJDaChave(chave), secaoACBr, ambiente, p.Certificado)

	res, err := m.svc.Transmitir(t, xml)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
		return
	}

	e := ParseEnvio(res.Resposta)
	if e.Chave == "" {
		e.Chave = chave // a lib nem sempre repete a chave na rejeição
	}
	resp := RespostaTransmissao{
		Chave: e.Chave, Protocolo: e.Protocolo, Status: StatusEmissao(e),
		CStat: e.CStat, Motivo: e.XMotivo, Recibo: e.Recibo, Erros: e.Erros,
		XMLProcBase64: fiscal.Base64(res.XML),
	}
	httpx.JSON(w, fiscal.StatusDoDesfecho(resp.Status), resp)
}

// --- eventos ----------------------------------------------------------------

// PedidoEvento é o envelope de qualquer evento do MDF-e. Os campos próprios de
// cada tipo vão aninhados em "evento": assim os dois níveis podem recusar
// campo desconhecido, que é como o cliente descobre que errou um nome.
type PedidoEvento struct {
	Chave       string             `json:"chave"`
	Protocolo   string             `json:"protocolo,omitempty"`
	Ambiente    string             `json:"ambiente,omitempty"`
	DhEvento    string             `json:"dhEvento,omitempty" fmt:"data-hora"`
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

	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
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
			"evento '"+tipo+"' não existe para MDF-e; consulte GET /v1/capacidades")
		return
	}

	t := fiscal.Tenant(cnpj, secaoACBr, p.Ambiente, p.Certificado)
	res, err := envia(t, ini)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
		return
	}

	ev := ParseEvento(res.Resposta)
	resp := RespostaEvento{
		Tipo: tipo, Chave: chave, Status: StatusEvento(ev),
		CStat: ev.CStat, Motivo: ev.XMotivo, Protocolo: ev.Protocolo, Erros: ev.Erros,
		XMLEventoBase64: fiscal.Base64(res.XML),
	}
	httpx.JSON(w, fiscal.StatusDoEvento(resp.Status), resp)
}

// prepararEvento monta o INI do tipo pedido e escolhe o método de envio.
// errMsg != "" é erro do cliente; envia == nil é tipo inexistente.
func (m *Modulo) prepararEvento(tipo, chave, cnpj, dh string, p PedidoEvento) (
	ini string, envia func(acbr.TenantConfig, string) (acbr.Result, error), errMsg string) {

	// Genérico cobre condutor, DF-e e pagamento; encerramento e cancelamento
	// têm método próprio na lib.
	generico := m.svc.EnviarEvento

	switch tipo {
	case "encerramento":
		var e PedidoEncerramento
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if strings.TrimSpace(e.CMun) == "" {
			return "", nil, "evento.cMun (município de encerramento) é obrigatório"
		}
		return ToINIEncerramento(chave, cnpj, p.Protocolo, dh, e), m.svc.Encerrar, ""

	case "cancelamento":
		var e PedidoCancelamento
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if strings.TrimSpace(e.Justificativa) == "" {
			return "", nil, "evento.justificativa é obrigatória"
		}
		return ToINICancelamento(chave, cnpj, p.Protocolo, dh, e), m.svc.Cancelar, ""

	case "inclusao-condutor":
		var e PedidoInclusaoCondutor
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if strings.TrimSpace(e.Nome) == "" || strings.TrimSpace(e.CPF) == "" {
			return "", nil, "evento.nome e evento.cpf do condutor são obrigatórios"
		}
		return ToINIInclusaoCondutor(chave, cnpj, dh, e), generico, ""

	case "inclusao-dfe":
		var e PedidoInclusaoDFe
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if len(e.Documentos) == 0 {
			return "", nil, "informe ao menos um documento (chNFe) em evento.documentos"
		}
		return ToINIInclusaoDFe(chave, cnpj, dh, e), generico, ""

	case "pagamento-operacao":
		var e PedidoPagamentoOperacao
		if errMsg = fiscal.DecodarAninhado("evento", p.Evento, &e); errMsg != "" {
			return "", nil, errMsg
		}
		if len(e.Pagamentos) == 0 {
			return "", nil, "informe ao menos um pagamento em evento.pagamentos"
		}
		return ToINIPagamentoOperacao(chave, cnpj, dh, e), generico, ""
	}
	return "", nil, ""
}

// --- consultas --------------------------------------------------------------

// PedidoConsulta consulta um MDF-e pela chave.
type PedidoConsulta struct {
	Chave       string             `json:"chave"`
	Ambiente    string             `json:"ambiente,omitempty"`
	Certificado fiscal.Certificado `json:"certificado"`
}

// handleConsulta é a recuperação de uma transmissão de desfecho desconhecido:
// depois de um timeout, é ISTO que se chama: nunca repetir a transmissão.
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
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	t := fiscal.Tenant(fiscal.CNPJDaChave(chave), secaoACBr, p.Ambiente, p.Certificado)
	res, err := m.svc.Consultar(t, chave)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
		return
	}
	e := ParseEnvio(res.Resposta)
	httpx.JSON(w, http.StatusOK, RespostaTransmissao{
		Chave: fiscal.Primeiro(e.Chave, chave), Protocolo: e.Protocolo, Status: StatusEmissao(e),
		CStat: e.CStat, Motivo: e.XMotivo, Erros: e.Erros,
		XMLProcBase64: fiscal.Base64(res.XML),
	})
}

// PedidoSefaz é o corpo das consultas que não têm chave.
type PedidoSefaz struct {
	Ambiente    string             `json:"ambiente,omitempty"`
	CNPJ        string             `json:"cnpj,omitempty"`
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

// handleNaoEncerrados lista os MDF-e do CNPJ ainda em trânsito. Serve à
// conformidade: emitir novo manifesto com outro em aberto é rejeição.
func (m *Modulo) handleNaoEncerrados(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerPedidoSefaz(w, r)
	if !ok {
		return
	}
	cnpj := fiscal.SoDigitos(p.CNPJ)
	if cnpj == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "cnpj é obrigatório")
		return
	}
	res, err := m.svc.ConsultaNaoEncerrados(t, cnpj)
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
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return p, acbr.TenantConfig{}, false
	}
	t := fiscal.Tenant(fiscal.SoDigitos(p.CNPJ), secaoACBr, p.Ambiente, p.Certificado)
	return p, t, true
}

func (m *Modulo) responderCru(w http.ResponseWriter, res acbr.Result, err error) {
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
		return
	}
	corpo := map[string]any{"codigo": res.Codigo, "resposta": res.Resposta}
	if res.XML != "" {
		corpo["xml_b64"] = fiscal.Base64(res.XML)
	}
	httpx.JSON(w, http.StatusOK, corpo)
}

// --- representação gráfica --------------------------------------------------

// PedidoPDF gera o DAMDFE a partir do XML autorizado. Não fala com a SEFAZ,
// logo não pede certificado.
//
// É a rede de segurança do modelo sem estado: o MDF-e não tem "obter PDF pela
// chave", se o cliente perder o XML, o DAMDFE não se regenera de lugar nenhum.
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
	evt, ok := fiscal.XMLdeBase64(w, "xml_evento_b64", p.XMLEventoBase64)
	if !ok {
		return
	}
	res, err := m.svc.SalvarEventoPDF(fiscal.Tenant("", secaoACBr, "", fiscal.Certificado{}), xml, evt)
	m.responderPDF(w, res, err)
}

// modalRodoviario é o único modal que este serviço emite. Ver Escopo, no README.
// Texto porque é assim que ele chega à conferência compartilhada: o código do
// rodoviário muda por documento ("01" no CT-e, "1" aqui).
const modalRodoviario = "1"

// modalTexto converte o modal do pedido para texto, e o zero (campo ausente)
// para vazio, que é como a conferência compartilhada entende "não informado".
func modalTexto(modal int) string {
	if modal == 0 {
		return ""
	}
	return strconv.Itoa(modal)
}

func (m *Modulo) responderPDF(w http.ResponseWriter, res acbr.Result, err error) {
	if err != nil {
		fiscal.ResponderErroDaLib(w, "MDF-e", res, err)
		return
	}
	if len(res.PDF) == 0 {
		httpx.ErroJSON(w, http.StatusUnprocessableEntity, "pdf_nao_gerado", "a lib não devolveu PDF")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"pdf_b64": base64.StdEncoding.EncodeToString(res.PDF)})
}
