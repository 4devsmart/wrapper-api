package nfse

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// secaoACBr é a seção de configuração deste documento na ACBrLib.
const secaoACBr = "NFSe"

// Modulo expõe a NFS-e sob /v1/nfse.
//
// É o mais diferente dos três documentos fiscais: multi-provedor (o município
// decide o layout), sem validação de regras de negócio na lib, e com PDF
// recuperável pela chave: coisa que CT-e e MDF-e não têm.
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
	// Gerar: monta o XML da DPS. Sem certificado, sem rede.
	r.HandleFunc("POST /xml", operacao("xml", m.handleXML))
	// Transmitir: assina e envia. É aqui que o certificado entra.
	r.HandleFunc("POST /transmissao", operacao("transmissao", m.handleTransmissao))
	// Desfecho de um lote assíncrono, pelo protocolo que a transmissão devolveu.
	r.HandleFunc("POST /transmissao/lote", operacao("lote", m.handleLote))
	// Eventos (cancelamento e substituição): chamada única.
	r.HandleFunc("POST /eventos/{tipo}", operacao("evento", m.handleEvento))
	// Consultas ao provedor: POST porque levam o certificado no corpo.
	r.HandleFunc("POST /consulta", operacao("consulta", m.handleConsulta))
	r.HandleFunc("POST /consulta-dps", operacao("consulta-dps", m.handleConsultaDPS))
	r.HandleFunc("POST /consultas/{tipo}", operacao("consultas", m.handleConsultas))
	r.HandleFunc("POST /distribuicao", operacao("distribuicao", m.handleDistribuicao))
	// PDF: a NFS-e recupera o DANFSE pela CHAVE (ObterDANFSE), diferente de CT-e
	// e MDF-e: perder o XML aqui não é definitivo. Também aceita XML.
	r.HandleFunc("POST /pdf", operacao("pdf", m.handlePDF))
	// Tabela de municípios: pública quanto a segredo (não leva certificado) e é
	// como o cliente descobre, ANTES de emitir, se o município é atendido.
	r.HandleFunc("GET /municipios/{codigo}", m.handleMunicipio)
}

// --- gerar o XML -----------------------------------------------------------

// RespostaXML é o retorno da geração.
//
// Não há "chave" aqui: a chave de acesso da NFS-e é atribuída pelo provedor na
// autorização. O que a geração entrega é o IDENTIFICADOR DA DPS, que é
// determinístico a partir do pedido, e é ele que recupera uma transmissão
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
	rota, ok := m.rota(w, cmun)
	if !ok {
		return
	}
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	if msg := ValidarDPSDoMunicipio(cmun, p); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", msg)
		return
	}
	rota = rota.DaEmissao(p)
	registrarRota(w, rota, p.Ambiente)
	layout := rota.Layout

	t := m.sessao(cnpj, rota.Sessao, p.InfDPS.Prest.Pessoa, p.Ambiente, fiscal.Certificado{}, Credenciais{})
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
	if raiz, _ := RaizDoXML(xml); !LayoutConfere(layout, raiz) {
		slog.Error("nfse: XML gerado fora do layout do município",
			"municipio", cmun, "provedor", rota.Provedor(),
			"layout", layout, "raiz", raiz.Nome, "namespace", raiz.Namespace)
		httpx.ErroDetalhado(w, http.StatusInternalServerError, "layout_divergente",
			"o XML gerado não é do layout do município; nada foi devolvido para não ser transmitido",
			map[string]any{"municipio": cmun, "provedor": rota.Provedor(),
				"layout": layout, "raiz": raiz})
		return
	}
	httpx.JSON(w, http.StatusOK, RespostaXML{
		IDdps:     IDdoXML(xml),
		XMLBase64: fiscal.Base64(xml),
		// O provedor assina a DPS dentro do envio: a NFS-e não expõe assinatura
		// separada, ao contrário de CT-e e MDF-e. Dizer a verdade aqui evita que
		// o cliente arquive isto achando que é documento assinado.
		Assinado: false,
		Layout:   string(layout),
		Provedor: rota.Provedor(),
		// Validacao.Suportada virá false: o ACBrNFSeX não exporta validação de
		// regras de negócio (CT-e e MDF-e exportam). Fingir "ok" seria mentir
		// sobre a garantia que o cliente tem em mãos.
		Validacao: val,
	})
}

// --- transmitir -----------------------------------------------------

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

// LogValue é obrigatório junto com String: o serviço loga em JSON, e o
// manipulador JSON do slog NÃO usa Stringer, ele serializa a struct campo a
// campo. Só com String() a senha e o token saíam inteiros no log. Verificado.
func (c Credenciais) LogValue() slog.Value { return slog.StringValue("Credenciais{redigido}") }

// PedidoTransmissao é o corpo da transmissão.
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

// RespostaTransmissao é o retorno da transmissão.
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
	rota, ok := m.rota(w, p.Municipio)
	if !ok {
		return
	}

	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	ambiente := fiscal.Primeiro(fiscal.AmbienteDoXML(xml), p.Ambiente)
	rota = rota.DaDPSPronta(xml)
	registrarRota(w, rota, ambiente)
	t := m.sessaoDoEmitente(p.Emitente, rota.Sessao, ambiente, p.Certificado, p.Credenciais)

	res, err := m.svc.Transmitir(t, xml)
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, rota.Sessao, res.Resposta, "emissão") {
		return
	}

	e := ParseEnvio(res.Resposta)
	resp := RespostaTransmissao{
		Numero: e.Numero, Chave: chaveDaEmissao(e, res.XML), CodigoVerificacao: e.CodigoVerificacao,
		Protocolo: e.Protocolo, Situacao: e.Situacao,
		Status:    statusEmissao(e),
		XMLBase64: fiscal.Base64(res.XML),
		Erros:     e.Erros, Alertas: e.Alertas,
	}
	registrarDesfecho(w, resp.Status, e.Erros)
	httpx.JSON(w, fiscal.StatusDoDesfecho(resp.Status), resp)
}

// statusEmissao traduz a Emissao no vocabulário comum aos módulos.
func statusEmissao(e Emissao) string {
	switch {
	case e.Sucesso && e.AguardaProcessamento():
		return "processando"
	case e.Sucesso:
		return "autorizado"
	case len(e.Erros) > 0:
		return "rejeitado"
	default:
		return "erro"
	}
}

// handleLote devolve o desfecho de um lote assíncrono no mesmo formato da
// transmissão: é a segunda metade dela. O cliente que recebeu "processando"
// chama isto com o protocolo até ter autorizado ou rejeitado.
//
// Na autorização o XML é o da NFS-e que o provedor devolveu na consulta, e não
// o do RPS enviado: é o documento que se guarda e de onde sai o DANFSE.
func (m *Modulo) handleLote(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(p.Protocolo) == "" && strings.TrimSpace(p.NumeroLote) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"protocolo (devolvido pela transmissão) é obrigatório")
		return
	}
	res, err := m.svc.ConsultarLoteRps(t, p.Protocolo, p.NumeroLote)
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, p.Municipio, res.Resposta, "consulta de lote") {
		return
	}
	l := ParseLoteRps(res.Resposta)
	resp := RespostaTransmissao{
		Numero: l.Numero, CodigoVerificacao: l.CodigoVerificacao,
		Protocolo: fiscal.Primeiro(l.Protocolo, p.Protocolo),
		Situacao:  l.DescSituacao,
		Status:    l.Status(),
		Erros:     l.Erros, Alertas: l.Alertas,
	}
	if resp.Status == "autorizado" {
		resp.XMLBase64 = fiscal.Base64(res.XML)
		resp.Chave = ChaveDoXML(res.XML)
	}
	registrarDesfecho(w, resp.Status, l.Erros)
	httpx.JSON(w, fiscal.StatusDoDesfecho(resp.Status), resp)
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
	Situacao  string     `json:"situacao,omitempty"`
	Protocolo string     `json:"protocolo,omitempty"`
	DataHora  string     `json:"data_hora,omitempty"`
	XMLBase64 string     `json:"xml_b64,omitempty"`
	Erros     []Mensagem `json:"mensagens,omitempty"`
	Alertas   []Mensagem `json:"alertas,omitempty"`
	// NFSe é a nota que o evento atingiu: a cancelada ou a substituída.
	NFSe *NotaDoEvento `json:"nfse,omitempty"`
	// Substituta é a nota que nasceu da substituição. Só vem nela, e só quando o
	// provedor a aceitou.
	Substituta *NotaDoEvento `json:"substituta,omitempty"`
	// NFSeJaCancelada marca o cancelamento recusado porque a nota já estava
	// cancelada. O status continua rejeitado, como sempre foi: o campo só diz
	// que o desfecho que o cliente queria já existe.
	NFSeJaCancelada bool `json:"nfse_ja_cancelada,omitempty"`
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
	rota, ok := m.rota(w, p.Municipio)
	if !ok {
		return
	}
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	rota = rota.DoEvento(tipo, p)
	registrarRota(w, rota, p.Ambiente)
	layout := rota.Layout
	p.Municipio = rota.Sessao
	t := m.sessaoDoEmitente(p.Emitente, p.Municipio, p.Ambiente,
		p.Certificado, p.Credenciais)

	if tipo == "substituicao" {
		m.substituir(w, t, layout, p)
		return
	}
	m.cancelar(w, t, layout, p)
}

// cancelar escolhe o caminho do cancelamento pelo layout do município.
//
// São dois webservices distintos, não duas variações do mesmo. O Padrão
// Nacional cancela por EVENTO (pedido de registro de evento, tpEvento e101101);
// os demais provedores cancelam pelo webservice CancelaNFSe. Chamar o segundo
// num município do Padrão Nacional é o que devolvia "não implementado para este
// provedor" em 300 ms, sem nada sair para o ADN.
//
// A identificação da nota também muda com o layout. O evento do Padrão Nacional
// aponta a chave de acesso. O ABRASF não tem chave: o que a wrapper devolve
// nesse campo é o link da nota, quando o provedor manda um, e o CancelaNFSe
// localiza a nota pelo número. Exigir a chave ali recusava o cancelamento de
// nota GISS autorizada sem link, com o número em mãos.
func (m *Modulo) cancelar(w http.ResponseWriter, t acbr.Sessao, layout Layout, p PedidoEvento) {
	if layout == LayoutPadraoNacional && strings.TrimSpace(p.Chave) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "chave da NFS-e é obrigatória")
		return
	}
	var e CancelamentoPedido
	if msg := fiscal.DecodarAninhado("evento", p.Evento, &e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}

	if layout == LayoutPadraoNacional {
		m.cancelarPorEvento(w, t, p, e)
		return
	}

	if strings.TrimSpace(p.Chave) == "" && strings.TrimSpace(e.Numero) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"evento.numero (NFS-e a cancelar) é obrigatório fora do Padrão Nacional")
		return
	}

	res, err := m.svc.Cancelar(t, ToINICancelamento(p.Chave, p.Municipio, e))
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, p.Municipio, res.Resposta, "cancelamento") {
		return
	}
	c := ParseCancelamento(res.Resposta)
	// O XML que o worker pega depois do cancelamento é o da lista de notas, e a
	// sessão começa vazia: o que volta ali é a mensagem de índice inválido. O
	// documento do evento ABRASF é a resposta do provedor, que só vale guardar
	// quando o cancelamento foi confirmado.
	xmlDoEvento := res.XML
	if c.Sucesso && strings.HasPrefix(c.XmlRetorno, "<") {
		xmlDoEvento = c.XmlRetorno
	}
	status := StatusCancelamento(c)
	registrarDesfecho(w, status, c.Erros)
	httpx.JSON(w, fiscal.StatusDoEvento(status), RespostaEvento{
		Tipo: "cancelamento", Chave: p.Chave, Status: status,
		Protocolo: c.Protocolo, DataHora: c.DataHora,
		XMLBase64: fiscal.Base64(xmlDoEvento),
		Erros:     c.Erros, Alertas: c.Alertas,
		NFSe:            notaCancelada(p.Chave, e),
		NFSeJaCancelada: status == "rejeitado" && JaCancelada(c.Erros),
	})
}

// cancelarPorEvento cancela no Padrão Nacional, que é o evento e101101.
//
// A validação do motivo é local de propósito: xMotivo é obrigatório e tem
// mínimo de 15 caracteres no schema do evento, e o ADN recusa por schema sem
// dizer qual campo faltou.
func (m *Modulo) cancelarPorEvento(w http.ResponseWriter, t acbr.Sessao,
	p PedidoEvento, e CancelamentoPedido) {

	if msg := ValidarCancelamentoPN(e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}
	res, err := m.svc.EnviarEvento(t, ToINIEvento(EventoPN{
		Chave:     p.Chave,
		TpEvento:  TpEventoCancelamento,
		TpAmb:     fiscal.TpAmb(p.Ambiente),
		CodMotivo: cmp.Or(e.Codigo, "1"),
		Motivo:    strings.TrimSpace(e.Motivo),
	}))
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, p.Municipio, res.Resposta, "cancelamento") {
		return
	}
	ev := ParseEvento(res.Resposta)
	status := StatusEvento(ev)
	registrarDesfecho(w, status, ev.Erros)
	httpx.JSON(w, fiscal.StatusDoEvento(status), RespostaEvento{
		Tipo: "cancelamento", Chave: p.Chave, Status: status,
		Situacao:  ev.Situacao,
		XMLBase64: fiscal.Base64(fiscal.Primeiro(res.XML, ev.XML)),
		Erros:     ev.Erros, Alertas: ev.Alertas,
		NFSe:            notaCancelada(p.Chave, e),
		NFSeJaCancelada: status == "rejeitado" && JaCancelada(ev.Erros),
	})
}

// substituir emite a DPS substituta identificando a NFS-e antiga.
//
// Como no cancelamento, são dois caminhos. O Padrão Nacional NÃO tem webservice
// de substituição: a nota nova carrega o grupo subst apontando a chave da
// antiga, e é a emissão dela que substitui. Os demais provedores usam o
// SubstituiNFSe, que identifica a antiga por número.
func (m *Modulo) substituir(w http.ResponseWriter, t acbr.Sessao, layout Layout, p PedidoEvento) {
	var e SubstituicaoPedido
	if msg := fiscal.DecodarAninhado("evento", p.Evento, &e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}

	if layout == LayoutPadraoNacional {
		m.substituirPorDPS(w, t, p.Municipio, e)
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
	if m.naoSuportada(w, t, p.Municipio, res.Resposta, "substituição") {
		return
	}
	m.responderSubstituicao(w, res, e)
}

// substituirPorDPS é a substituição do Padrão Nacional: emitir a DPS nova com o
// grupo subst. A chamada é a MESMA da emissão, e a resposta também: o que
// identifica a substituição é o grupo dentro do documento.
func (m *Modulo) substituirPorDPS(w http.ResponseWriter, t acbr.Sessao,
	cmun string, e SubstituicaoPedido) {

	if msg := ValidarSubstituicaoPN(e); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "pedido_invalido", msg)
		return
	}
	// cmun é o da sessão, que pode ser o do Ambiente Nacional. A validação olha o
	// município da nota nova, que é quem tem regra.
	if msg := ValidarDPSDoMunicipio(fiscal.Primeiro(MunicipioDoPedido(e.DPS), cmun), e.DPS); msg != "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", msg)
		return
	}
	res, err := m.svc.Emitir(t, ToINISubstituicaoPN(e))
	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, cmun, res.Resposta, "substituição") {
		return
	}
	m.responderSubstituicao(w, res, e)
}

func (m *Modulo) responderSubstituicao(w http.ResponseWriter, res acbr.Result, e SubstituicaoPedido) {
	em := ParseEnvio(res.Resposta)
	registrarDesfecho(w, statusEmissao(em), em.Erros)
	httpx.JSON(w, fiscal.StatusDoDesfecho(statusEmissao(em)), RespostaEvento{
		Tipo: "substituicao", Chave: chaveDaEmissao(em, res.XML), Status: statusEmissao(em),
		Situacao:  em.Situacao,
		Protocolo: em.Protocolo, XMLBase64: fiscal.Base64(res.XML),
		Erros: em.Erros, Alertas: em.Alertas,
		NFSe:       notaSubstituida(e.Substituida),
		Substituta: notaSubstituta(em, res.XML, e.DPS),
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
	m.responderConsulta(w, t, p.Municipio, res, err)
}

// handleConsultaDPS é a recuperação de uma transmissão de desfecho desconhecido.
//
// Depois de um timeout na transmissão, é ISTO que se chama: nunca reenviar a DPS.
// Aceita o id_dps como a geração devolveu (com o prefixo "DPS") ou a chave nua: o
// webservice quer a chave sem prefixo, e essa diferença não é problema do cliente.
func (m *Modulo) handleConsultaDPS(w http.ResponseWriter, r *http.Request) {
	p, t, ok := m.lerConsulta(w, r)
	if !ok {
		return
	}
	chave := ChaveDPS(p.Chave)
	if chave == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"chave (o id_dps devolvido ao gerar) é obrigatória")
		return
	}
	// Só a NFS-e encontrada encerra a busca. Não achar, ou falhar, passa para a
	// sessão seguinte, onde a nota também pode estar, e a última responde o que
	// tiver.
	rota, _ := RotaDoMunicipio(p.Municipio)
	rotas := rota.RotasDaConsultaDPS()
	for _, rt := range rotas[:len(rotas)-1] {
		tn := m.sessaoDoEmitente(p.Emitente, rt.Sessao, p.Ambiente, p.Certificado, p.Credenciais)
		if res, err := m.svc.ConsultarDPSPorChave(tn, chave); err == nil && reChaveNaResposta.MatchString(res.Resposta) {
			registrarRota(w, rt, p.Ambiente)
			m.responderConsulta(w, tn, rt.Sessao, res, nil)
			return
		}
	}
	res, err := m.svc.ConsultarDPSPorChave(t, chave)
	m.responderConsulta(w, t, p.Municipio, res, err)
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
		m.responderConsulta(w, t, p.Municipio, res, err)
	case "faixa":
		if p.Numero == "" || p.NumeroFinal == "" {
			httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "numero e numero_final são obrigatórios")
			return
		}
		res, err := m.svc.ConsultarPorFaixa(t, p.Numero, p.NumeroFinal, pagina)
		m.responderConsulta(w, t, p.Municipio, res, err)
	case "rps":
		if p.Numero == "" {
			httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "numero (do RPS) é obrigatório")
			return
		}
		res, err := m.svc.ConsultarPorRps(t, p.Numero, p.Serie, p.Tipo, p.CodigoVerificacao)
		m.responderConsulta(w, t, p.Municipio, res, err)
	case "situacao":
		res, err := m.svc.ConsultarSituacao(t, p.Protocolo, p.NumeroLote)
		m.responderConsulta(w, t, p.Municipio, res, err)
	case "lote-rps":
		res, err := m.svc.ConsultarLoteRps(t, p.Protocolo, p.NumeroLote)
		m.responderConsulta(w, t, p.Municipio, res, err)
	default:
		httpx.ErroJSON(w, http.StatusNotFound, "consulta_desconhecida",
			"consulta '"+tipo+"' não existe; use numero, faixa, rps, situacao ou lote-rps")
	}
}

// handleDistribuicao consulta a Distribuição DF-e do ADN por NSU.
//
// O cursor (NSU) vai e volta no payload: sem estado no servidor, é o cliente que
// guarda onde parou. Não paralelize a distribuição do mesmo CNPJ, sem o lock
// que existia com banco, chamadas simultâneas embaralham o cursor.
//
// O município é opcional. Informado e conhecido, a distribuição sai pela classe
// do provedor dele, como sempre saiu: em alguns a fila é a do sefin da
// prefeitura, e trocar de fila em silêncio quebraria o cursor de quem já a lê.
// Só quando essa classe não implementa a distribuição, ou sem município, a
// sessão sai por SessaoNacional, sem credenciais de prefeitura: o ADN autentica
// pelo certificado. Erro de outra natureza não cai para o ADN, porque o NSU do
// cliente é da fila do município.
func (m *Modulo) handleDistribuicao(w http.ResponseWriter, r *http.Request) {
	var p PedidoConsulta
	if !m.lerPedidoConsulta(w, r, &p) {
		return
	}
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return
	}
	rota, ok := RotaDoMunicipio(p.Municipio)
	if ok {
		registrarRota(w, rota, p.Ambiente)
		t := m.sessaoDoEmitente(p.Emitente, rota.Sessao, p.Ambiente, p.Certificado, p.Credenciais)
		res, err := m.svc.ConsultarDFe(t, cnpjConsulta(p.Emitente, rota.Sessao), p.NSU)
		if err != nil || !OperacaoNaoSuportada(res.Resposta) {
			m.responderDistribuicao(w, t, rota.Sessao, res, err)
			return
		}
	}
	registrarRota(w, rota.nacional(motivoDistribuicao), p.Ambiente)
	t := m.sessaoDoEmitente(p.Emitente, SessaoNacional, p.Ambiente, p.Certificado, Credenciais{})
	res, err := m.svc.ConsultarDFe(t, cnpjConsulta(p.Emitente, SessaoNacional), p.NSU)
	m.responderDistribuicao(w, t, SessaoNacional, res, err)
}

// responderDistribuicao é o responderConsulta, menos a rejeição muda do ADN.
//
// O ADN pode responder REJEICAO sem nenhum erro listado: é o que faz quando
// falta o cnpjConsulta. A lib só monta [ErroN] a partir da lista, e a resposta
// chegava com código 0, sem erro e sem documento, igual à de uma fila em dia.
// O cliente seguia achando que não havia nada a baixar.
func (m *Modulo) responderDistribuicao(w http.ResponseWriter, t acbr.Sessao,
	cmun string, res acbr.Result, err error) {

	if err == nil && rejeicaoSemErro(res.Resposta) {
		httpx.ErroDetalhado(w, http.StatusBadGateway, "distribuicao_rejeitada",
			"o ADN rejeitou a distribuição sem informar o motivo",
			map[string]any{"resposta": res.Resposta})
		return
	}
	m.responderConsulta(w, t, cmun, res, err)
}

// rejeicaoSemErro diz se o retorno do ADN, que a lib devolve cru em XmlRetorno,
// é uma REJEICAO com a lista de erros vazia. Retorno que não é JSON (o sefin da
// prefeitura, por exemplo) nunca é.
func rejeicaoSemErro(resposta string) bool {
	for linha := range strings.Lines(resposta) {
		corpo, ok := strings.CutPrefix(strings.TrimRight(linha, "\r\n"), "XmlRetorno=")
		if !ok {
			continue
		}
		var r struct {
			StatusProcessamento string
			Erros               []json.RawMessage
		}
		if json.Unmarshal([]byte(corpo), &r) != nil {
			return false
		}
		return r.StatusProcessamento == "REJEICAO" && len(r.Erros) == 0
	}
	return false
}

// cnpjConsulta é o CNPJ que a distribuição leva no cnpjConsulta, que o ADN
// exige: sem o parâmetro, a consulta volta REJEICAO sem erro listado, mesmo com
// o certificado do próprio CNPJ.
//
// A fila é a do ADN em toda classe que implementa a distribuição: o Padrão
// Nacional e as de API própria que herdam dele (Betha, ISSNet, Fiorilli e
// outras) reaproveitam a URL dele (ServicosPadraoNacional no
// ACBrNFSeXServicos.ini). A exceção é o município com URL própria, o sefin da
// prefeitura nos do Infisc no RS, onde o parâmetro não foi testado: ali a
// consulta continua saindo sem ele, como sempre saiu.
//
// Vazio também quando o emitente não tem 14 dígitos: a lib recusa outro tamanho
// antes de enviar.
func cnpjConsulta(e Emitente, cmun string) string {
	if tabelas.DistribuicaoPropriaNFSe(fiscal.SoDigitos(cmun)) {
		return ""
	}
	if c := fiscal.SoDigitos(e.CNPJ); len(c) == 14 {
		return c
	}
	return ""
}

// lerConsulta lê o pedido e abre a sessão no município informado, que é quem
// decide o provedor consultado.
func (m *Modulo) lerConsulta(w http.ResponseWriter, r *http.Request) (PedidoConsulta, acbr.Sessao, bool) {
	var p PedidoConsulta
	if !m.lerPedidoConsulta(w, r, &p) {
		return p, acbr.Sessao{}, false
	}
	rota, ok := m.rota(w, p.Municipio)
	if !ok {
		return p, acbr.Sessao{}, false
	}
	if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
		return p, acbr.Sessao{}, false
	}
	rota = rota.DaNota(p.Chave)
	registrarRota(w, rota, p.Ambiente)
	p.Municipio = rota.Sessao
	t := m.sessaoDoEmitente(p.Emitente, p.Municipio, p.Ambiente,
		p.Certificado, p.Credenciais)
	return p, t, true
}

// lerPedidoConsulta é o que toda consulta exige antes de saber com quem fala:
// JSON válido e certificado.
func (m *Modulo) lerPedidoConsulta(w http.ResponseWriter, r *http.Request, p *PedidoConsulta) bool {
	if !httpx.LerJSON(w, r, p) {
		return false
	}
	if err := p.Certificado.Validar(); err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", err.Error())
		return false
	}
	return true
}

func (m *Modulo) responderConsulta(w http.ResponseWriter, t acbr.Sessao,
	cmun string, res acbr.Result, err error) {

	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, cmun, res.Resposta, "esta consulta") {
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
// pela CHAVE (ObterDANFSE no provedor), então perder o XML não é definitivo.
// Por chave a operação fala com o provedor e exige certificado; por XML é render
// local e não exige.
//
// O município é obrigatório por chave e opcional por XML: é ele que decide o
// provedor, e no caminho do XML sai do próprio documento quando não vem no
// pedido. Informe-o para vencer o que está no XML.
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
	// A sessão e o município usados sobrevivem ao switch: são eles que dizem
	// QUAL provedor recusou, quando a lib recusa.
	var t acbr.Sessao
	var cmun string
	switch {
	case strings.TrimSpace(p.XMLBase64) != "":
		xml, ok := fiscal.XMLdeBase64(w, "xml_b64", p.XMLBase64)
		if !ok {
			return
		}
		// O provedor precisa estar selecionado ANTES do CarregarXML: quem LÊ o
		// XML é a classe do provedor, e ela só é escolhida pelo município. Sem
		// isso a lib recusa o documento sem olhá-lo ("Nenhum provedor
		// selecionado"). O município vem do pedido; na falta dele, do próprio
		// XML, que já o carrega.
		cmun = fiscal.Primeiro(fiscal.SoDigitos(p.Municipio), MunicipioDoXML(xml))
		rota, ok := m.rota(w, cmun)
		if !ok {
			return
		}
		rota = rota.DaNota(ChaveDoXML(xml))
		registrarRota(w, rota, "")
		cmun = rota.Sessao
		// Render local: não fala com o provedor, então não pede certificado.
		t = fiscal.Sessao("", secaoACBr, "", fiscal.Certificado{})
		t.Config = append(t.Config,
			acbr.ConfigKV{Section: secaoACBr, Key: "CodigoMunicipio", Value: cmun})
		res, err = m.svc.RenderizarPDF(t, xml)
	case strings.TrimSpace(p.Chave) != "":
		if errCert := p.Certificado.Validar(); errCert != nil {
			httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido", errCert.Error())
			return
		}
		rota, ok := m.rota(w, p.Municipio)
		if !ok {
			return
		}
		if !fiscal.AmbienteDoPedido(w, &p.Ambiente, 0, "") {
			return
		}
		rota = rota.DaNota(p.Chave)
		registrarRota(w, rota, p.Ambiente)
		cmun = rota.Sessao
		t = m.sessaoDoEmitente(p.Emitente, cmun, p.Ambiente,
			p.Certificado, p.Credenciais)
		res, err = m.svc.ObterPDF(t, p.Chave)
	default:
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "informe chave ou xml_b64")
		return
	}

	if err != nil {
		m.responderErro(w, res, err)
		return
	}
	if m.naoSuportada(w, t, cmun, res.Resposta, "geração do DANFSE") {
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
//
// Com ?capacidades=1 a resposta ganha o que o provedor DE FATO expõe, lido da
// lib, e quais eventos desta API funcionam ali. É opcional porque a leitura
// abre uma sessão nativa (sem rede, mas nada barata), e o resto do endpoint é
// consulta de tabela.
func (m *Modulo) handleMunicipio(w http.ResponseWriter, r *http.Request) {
	codigo := fiscal.SoDigitos(r.PathValue("codigo"))
	layout, ok := LayoutDoMunicipio(codigo)
	corpo := map[string]any{
		"codigo":    codigo,
		"provedor":  provedorDoMunicipio(codigo),
		"layout":    string(layout),
		"suportado": ok,
	}
	if ok && r.URL.Query().Get("capacidades") == "1" {
		t := m.sessaoDoEmitente(Emitente{}, codigo, "", fiscal.Certificado{}, Credenciais{})
		if c, lido := m.capacidades(t); lido {
			corpo["capacidades"] = c
			corpo["operacoes"] = Operacoes(layout, c)
		}
	}
	httpx.JSON(w, http.StatusOK, corpo)
}

// --- apoio ------------------------------------------------------------------

// sessao monta a sessão a partir do prestador do próprio pedido (ao gerar).
func (m *Modulo) sessao(cnpj, cmun string, prest Pessoa, ambiente string,
	cert fiscal.Certificado, cred Credenciais) acbr.Sessao {
	return m.sessaoDoEmitente(Emitente{
		CNPJ: cnpj, InscMun: prest.IM, RazaoSocial: prest.XNome,
	}, cmun, ambiente, cert, cred)
}

// sessaoDoEmitente monta a sessão nativa. O emitente vem do pedido porque não há
// cadastro no servidor: é a consequência direta de não persistir nada.
func (m *Modulo) sessaoDoEmitente(e Emitente, cmun, ambiente string,
	cert fiscal.Certificado, cred Credenciais) acbr.Sessao {

	cnpj := fiscal.SoDigitos(e.CNPJ)
	t := fiscal.Sessao(cnpj, secaoACBr, ambiente, cert)
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
	// Só provedores não-Padrão Nacional usam login/token de prefeitura. Decide o
	// provedor, não o layout: ABase e EL (WSChaveAcesso) e ModernizacaoPublica
	// (WSUser, WSSenha) autenticam na API própria, onde o layout é o nacional.
	if tabelas.FamiliaPorMunicipio(fiscal.SoDigitos(cmun)) != tabelas.FamiliaPadraoNacional {
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
// A frase importa. A anterior dizia que "o provedor deste município não oferece
// a operação por webservice", e isso é um diagnóstico ERRADO: o que a lib
// levanta é ERR_NAO_IMP ("Serviço %s não implementado para este provedor"), uma
// decisão da CLASSE do provedor dentro da biblioteca fiscal, tomada antes de
// qualquer byte sair. A prefeitura pode muito bem oferecer o serviço. Quem
// mandava o cliente ligar para a prefeitura estava sendo mandado ao lugar
// errado por nós.
//
// Os detalhes trazem o que aquele provedor DE FATO expõe, perguntado à lib
// (ObterInformacoesProvedor). É o que transforma "não dá" em "não dá, e o que
// dá é isto".
func (m *Modulo) naoSuportada(w http.ResponseWriter, t acbr.Sessao,
	cmun, resposta, operacao string) bool {

	if !OperacaoNaoSuportada(resposta) {
		return false
	}
	registrarDesfecho(w, "operacao_nao_suportada", nil)
	det := map[string]any{"municipio": cmun, "resposta": resposta}
	provedor := provedorDoMunicipio(cmun)
	alvo := "este provedor"
	if provedor != "" {
		det["provedor"] = provedor
		alvo = "o provedor " + provedor
	}
	if c, ok := m.capacidades(t); ok {
		det["servicos_do_provedor"] = c.Servicos
	}
	httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "operacao_nao_suportada",
		"a biblioteca fiscal não implementa "+operacao+" para "+alvo+
			": o limite é desta integração, não do município", det)
	return true
}

// capacidades pergunta à lib o que o provedor da sessão expõe. Best-effort: a
// resposta só enriquece um erro que já vai ser devolvido, e falhar aqui não
// pode trocar o erro real por outro.
func (m *Modulo) capacidades(t acbr.Sessao) (Capacidades, bool) {
	res, err := m.svc.InformacoesProvedor(t)
	if err != nil || strings.TrimSpace(res.Resposta) == "" {
		return Capacidades{}, false
	}
	c := ParseCapacidades(res.Resposta)
	return c, len(c.Servicos) > 0
}

func (m *Modulo) responderErro(w http.ResponseWriter, res acbr.Result, err error) {
	registrarDesfecho(w, fiscal.CodigoDoErroDaLib(err), nil)
	fiscal.ResponderErroDaLib(w, "NFS-e", res, err)
}
