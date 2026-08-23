package acbr

// Protocolo RPC entre a API e o processo fiscal-worker.
//
// Motivação: a lib nativa roda no MESMO processo de quem a chama, e um SIGSEGV
// dentro dela derruba esse processo inteiro. Isolando-a num worker, a API deixa
// de linkar o .so e um crash mata apenas a requisição em curso.
//
// O transporte é HTTP/1.1 (stdlib nos dois lados, timeout e cancelamento
// prontos, depurável com `curl --unix-socket`) sobre socket unix — assim o
// certificado A1 nunca sai do host. O corpo é JSON: TODOS os argumentos e
// retornos das interfaces de serviço já são dados simples (nenhum handle,
// ponteiro nativo ou callback atravessa a fronteira).

// Serviços endereçáveis no worker (o campo Servico do Pedido).
const (
	ServicoNFSe   = "nfse"
	ServicoCTe    = "cte"
	ServicoMDFe   = "mdfe"
	ServicoNFe    = "nfe"
	ServicoBoleto = "boleto"
)

// RotaRPC é o caminho HTTP da chamada; RotaHealth responde ao healthcheck do
// container (o worker não escuta em porta TCP).
const (
	RotaRPC    = "/rpc"
	RotaHealth = "/healthz"
)

// Pedido é uma chamada de método do binding.
type Pedido struct {
	Servico string       `json:"servico"`
	Metodo  string       `json:"metodo"`
	Tenant  TenantConfig `json:"tenant"`
	Args    Args         `json:"args"`
}

// Args reúne os argumentos de TODOS os métodos das interfaces de serviço, cada
// um em seu campo nomeado (só os do método em questão vêm preenchidos). É
// deliberadamente explícito: um campo genérico tipo []any economizaria linhas e
// custaria a legibilidade dos dois lados do protocolo.
type Args struct {
	INI            string `json:"ini,omitempty"`             // Emitir, MontarXML, Cancelar, eventos, GerarPDF/GerarRemessa
	XML            string `json:"xml,omitempty"`             // XmlParaIni, RenderizarPDF, Transmitir
	XMLDoc         string `json:"xml_doc,omitempty"`         // SalvarEventoPDF
	XMLEvento      string `json:"xml_evento,omitempty"`      // SalvarEventoPDF
	Chave          string `json:"chave,omitempty"`           // Consultar, ObterPDF
	Numero         string `json:"numero,omitempty"`          // ConsultarPorNumero/Faixa (inicial), ConsultarPorRps
	NumeroFinal    string `json:"numero_final,omitempty"`    // ConsultarPorFaixa
	Serie          string `json:"serie,omitempty"`           // ConsultarPorRps
	Tipo           string `json:"tipo,omitempty"`            // ConsultarPorRps
	CodVerificacao string `json:"cod_verificacao,omitempty"` // ConsultarPorRps
	Protocolo      string `json:"protocolo,omitempty"`       // ConsultarSituacao, ConsultarLoteRps
	NumLote        string `json:"num_lote,omitempty"`        // ConsultarSituacao, ConsultarLoteRps
	Recibo         string `json:"recibo,omitempty"`          // ConsultarRecibo
	UF             string `json:"uf,omitempty"`              // ConsultaCadastro
	Documento      string `json:"documento,omitempty"`       // ConsultaCadastro (CNPJ/CPF ou IE)
	CNPJ           string `json:"cnpj,omitempty"`            // ConsultaNaoEncerrados
	ConfigINI      string `json:"config_ini,omitempty"`      // LerRetorno
	Retorno        string `json:"retorno,omitempty"`         // LerRetorno (conteúdo do arquivo CNAB)
	EhIE           bool   `json:"eh_ie,omitempty"`           // ConsultaCadastro
	Pagina         int    `json:"pagina,omitempty"`          // ConsultarPorNumero/Faixa
	NSU            int    `json:"nsu,omitempty"`             // ConsultarDFe (NFS-e/ADN)
	NumArquivo     int    `json:"num_arquivo,omitempty"`     // GerarRemessa

	Dist   *DistDFeParams    `json:"dist,omitempty"`   // DistribuicaoDFe
	Sub    *SubstituicaoNFSe `json:"sub,omitempty"`    // SubstituirNFSe
	Boleto *BoletoOnline     `json:"boleto,omitempty"` // Registrar, ConsultarTitulos
}

// Resposta devolve o Result da chamada mais o erro, se houve. Result e erro NÃO
// são excludentes: a lib costuma devolver código/mensagem junto com a falha, e
// os handlers usam os dois.
type Resposta struct {
	Codigo   int    `json:"codigo,omitempty"`
	Resposta string `json:"resposta,omitempty"`
	XML      string `json:"xml,omitempty"`
	PDF      []byte `json:"pdf,omitempty"`    // JSON serializa []byte em base64
	Versao   string `json:"versao,omitempty"` // método Version
	Erro     string `json:"erro,omitempty"`
	// Indisponivel preserva a sentinela ErrUnavailable através do RPC: o handler
	// da API distingue "a lib não está disponível" (503) de "a lib falhou" (502).
	Indisponivel bool `json:"indisponivel,omitempty"`
	// NaoSuportado preserva ErrNaoSuportado. Sem este campo a sentinela morreria
	// na serialização (do outro lado sobraria uma string) e a API responderia 502
	// a algo que é 422: a lib está viva, a operação é que não existe para aquele
	// documento — ValidarRegras na NFS-e é o caso.
	NaoSuportado bool `json:"nao_suportado,omitempty"`
}

// Result monta o Result a partir da resposta.
func (r Resposta) Result() Result {
	return Result{Codigo: r.Codigo, Resposta: r.Resposta, XML: r.XML, PDF: r.PDF}
}
