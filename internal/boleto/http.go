package boleto

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

// Modulo expõe o boleto bancário sob /v1/boletos.
//
// É o único módulo que NÃO é fiscal: não fala com SEFAZ, não assina, não tem
// chave de acesso, e gerar já é transmitir. Não há envio a perder — gerar um
// boleto é uma operação local, e registrá-lo no banco é uma chamada idempotente
// do lado deles. Por isso aqui é uma chamada só — o que não é exceção à regra
// dos outros documentos: é outra regra, para outro problema.
type Modulo struct{ svc acbr.BoletoServico }

// NovoModulo liga o módulo ao binding.
func NovoModulo(svc acbr.BoletoServico) *Modulo { return &Modulo{svc: svc} }

func (m *Modulo) Nome() string { return "boletos" }

func (m *Modulo) Capacidades() []string {
	return []string{"pdf", "remessa", "retorno", "registro"}
}

func (m *Modulo) Registrar(r modulo.Router) {
	r.HandleFunc("POST /pdf", m.handlePDF)
	r.HandleFunc("POST /remessa", m.handleRemessa)
	r.HandleFunc("POST /retorno", m.handleRetorno)
	r.HandleFunc("POST /registro", m.handleRegistro)
	// ConsultarTitulos existe no binding mas NÃO é exposto: a seção de filtro que
	// ele exigiria ([BoletoConsulta]) não aparece em lugar nenhum do fonte oficial
	// do ACBr, então o formato do INI seria chute. Rota que só sabe falhar é pior
	// que rota ausente — ver docs/LIMITACOES.md.
}

// --- PDF --------------------------------------------------------------------

func (m *Modulo) handlePDF(w http.ResponseWriter, r *http.Request) {
	var p Pedido
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	if !m.exigirContaETitulos(w, p) {
		return
	}
	res, err := m.svc.GerarPDF(m.tenant(p.Conta), ToINI(p))
	if err != nil {
		fiscal.ResponderErroDaLib(w, "boleto", res, err)
		return
	}
	if len(res.PDF) == 0 {
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "pdf_nao_gerado",
			"a lib não devolveu o PDF do boleto", map[string]any{"resposta": res.Resposta})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"pdf_b64": base64.StdEncoding.EncodeToString(res.PDF)})
}

// --- remessa CNAB -----------------------------------------------------------

// PedidoRemessa é o corpo da geração de remessa.
type PedidoRemessa struct {
	Pedido
	// NumeroArquivo é o sequencial da remessa; o banco usa para ordenar e
	// detectar buraco. Sem estado no servidor, o controle é do cliente.
	NumeroArquivo int `json:"numeroArquivo,omitempty"`
}

func (m *Modulo) handleRemessa(w http.ResponseWriter, r *http.Request) {
	var p PedidoRemessa
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	if !m.exigirContaETitulos(w, p.Pedido) {
		return
	}
	num := p.NumeroArquivo
	if num <= 0 {
		num = 1
	}
	res, err := m.svc.GerarRemessa(m.tenant(p.Conta), ToINI(p.Pedido), num)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "boleto", res, err)
		return
	}
	if strings.TrimSpace(res.Resposta) == "" {
		httpx.ErroJSON(w, http.StatusUnprocessableEntity, "remessa_vazia", "a lib devolveu remessa vazia")
		return
	}
	// JSON, e não anexo text/plain como no projeto de origem: uma resposta por
	// formato em toda a API é um tratamento só no cliente. Quem quiser um
	// arquivo grava o campo.
	httpx.JSON(w, http.StatusOK, map[string]any{
		"remessa":        res.Resposta,
		"numero_arquivo": num,
	})
}

// --- retorno CNAB -----------------------------------------------------------

// PedidoRetorno é o corpo do processamento de retorno.
type PedidoRetorno struct {
	// Conta é só a configuração do banco — é ela que ensina a lib a interpretar
	// o arquivo. Títulos não entram aqui.
	Conta Conta `json:"conta"`
	// Arquivo é o conteúdo CRU do arquivo de retorno.
	Arquivo string `json:"arquivo"`
}

func (m *Modulo) handleRetorno(w http.ResponseWriter, r *http.Request) {
	var p PedidoRetorno
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	if strings.TrimSpace(p.Conta.Banco) == "" || strings.TrimSpace(p.Arquivo) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"conta.banco e arquivo (conteúdo do retorno) são obrigatórios")
		return
	}
	// Só a config do banco: o INI de retorno não leva títulos.
	cfg := ToINI(Pedido{Conta: p.Conta})
	res, err := m.svc.LerRetorno(m.tenant(p.Conta), cfg, p.Arquivo)
	if err != nil {
		fiscal.ResponderErroDaLib(w, "boleto", res, err)
		return
	}
	// A lib responde JSON (TipoResposta=2). Devolve aninhado; se não for JSON
	// válido, cai para string em vez de quebrar o cliente.
	var retorno any = res.Resposta
	if json.Valid([]byte(res.Resposta)) {
		retorno = json.RawMessage(res.Resposta)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"retorno": retorno})
}

// --- registro online --------------------------------------------------------

// PedidoRegistro é o corpo do registro online no banco.
type PedidoRegistro struct {
	Pedido
	// Operacao: 0 = incluir (default), 2 = baixar.
	Operacao int `json:"operacao,omitempty"`
}

func (m *Modulo) handleRegistro(w http.ResponseWriter, r *http.Request) {
	var p PedidoRegistro
	if !httpx.LerJSON(w, r, &p) {
		return
	}
	if strings.TrimSpace(p.Conta.Banco) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "conta.banco é obrigatório")
		return
	}
	if p.Conta.WS == nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"conta.ws (credenciais do banco) é obrigatório no registro online")
		return
	}
	// Baixa (operação 2) pode vir sem títulos novos; inclusão, não.
	if p.Operacao == 0 && len(p.Titulos) == 0 {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "informe ao menos um título")
		return
	}

	crt, errCRT := decodarOpcional(p.Conta.WS.CertCRT)
	key, errKEY := decodarOpcional(p.Conta.WS.CertKEY)
	if errCRT != nil || errKEY != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "certificado_invalido",
			"conta.ws.certCRT e conta.ws.certKEY devem ser base64")
		return
	}

	res, err := m.svc.Registrar(m.tenant(p.Conta), acbr.BoletoOnline{
		INI:      ToINI(p.Pedido),
		Operacao: p.Operacao,
		CertCRT:  crt,
		CertKEY:  key,
	})
	if err != nil {
		fiscal.ResponderErroDaLib(w, "boleto", res, err)
		return
	}
	if res.Codigo != 0 {
		// O banco recusou. 502 e não 500: a falha é do outro lado, e o detalhe
		// dela é o que o cliente precisa.
		httpx.ErroDetalhado(w, http.StatusBadGateway, "registro_recusado",
			"o banco recusou o registro", map[string]any{"codigo": res.Codigo, "resposta": res.Resposta})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"operacao": p.Operacao, "resposta": res.Resposta})
}

// --- apoio ------------------------------------------------------------------

func (m *Modulo) exigirContaETitulos(w http.ResponseWriter, p Pedido) bool {
	if strings.TrimSpace(p.Conta.Banco) == "" {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "conta.banco é obrigatório")
		return false
	}
	if len(p.Titulos) == 0 {
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio", "informe ao menos um título")
		return false
	}
	return true
}

// tenant rotula a sessão nativa com o cedente. O boleto não tem certificado A1
// nem ambiente fiscal: o banco é configuração, e o mTLS (quando existe) vai em
// BoletoOnline, não no TenantConfig.
func (m *Modulo) tenant(c Conta) acbr.TenantConfig {
	return acbr.TenantConfig{CNPJ: fiscal.SoDigitos(c.CNPJCPF)}
}

func decodarOpcional(b64 string) ([]byte, error) {
	if strings.TrimSpace(b64) == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
}
