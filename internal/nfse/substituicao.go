package nfse

// SubstituicaoPedido é o corpo de POST /v1/nfse/eventos/substituicao: a DPS substituta
// (nova nota) + os identificadores da NFS-e antiga + o motivo do cancelamento.
// Espelha os parâmetros do NFSE_SubstituirNFSe (provedores que expõem o
// webservice SubstituiNFSe: o Padrão Nacional faz substituição via grupo subst
// na DPS, não por este método).
type SubstituicaoPedido struct {
	DPS         DPSPedido          `json:"dps"`              // nota nova (substituta)
	Substituida NFSeSubstituidaRef `json:"substituida"`      // identifica a NFS-e antiga
	Codigo      string             `json:"codigo,omitempty"` // CodCancelamento, default "1"
	Motivo      string             `json:"motivo,omitempty"`
}

// NFSeSubstituidaRef identifica a NFS-e que está sendo substituída. Os valores
// vêm da emissão original (número/série/cód. verificação retornados pelo provedor).
type NFSeSubstituidaRef struct {
	Numero            string `json:"numero"`
	Serie             string `json:"serie,omitempty"`
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	NumeroLote        string `json:"numero_lote,omitempty"`
}
