package cte

// PedidoDesacordo é o corpo de POST /v1/cte/eventos/prestacao-desacordo (tpEvento
// 610110). Registrado pelo TOMADOR quando a prestação está em desacordo;
// xObs descreve o motivo.
type PedidoDesacordo struct {
	XObs       string `json:"xObs"`
	NSeqEvento int    `json:"nSeqEvento,omitempty"`
}

// ToINIDesacordo monta o INI da prestação em desacordo (610110): xObs na [EVENTO001].
func ToINIDesacordo(chave, cnpj, dhEvento string, p PedidoDesacordo) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "610110", p.NSeqEvento)
	b.kv("xObs", p.XObs)
	return b.String()
}
