package cte

// PedidoInsucessoEntrega é o corpo de POST /v1/cte/eventos/insucesso-entrega
// (tpEvento 110190). Registra a tentativa frustrada de entrega.
type PedidoInsucessoEntrega struct {
	NProt                  string   `json:"nProt,omitempty"`
	DhTentativaEntrega     string   `json:"dhTentativaEntrega,omitempty" fmt:"data-hora"`
	NTentativa             int      `json:"nTentativa,omitempty"`
	TpMotivo               string   `json:"tpMotivo,omitempty"` // default 1
	XJustMotivo            string   `json:"xJustMotivo,omitempty"`
	Latitude               float64  `json:"latitude,omitempty"`
	Longitude              float64  `json:"longitude,omitempty"`
	HashTentativaEntrega   string   `json:"hashTentativaEntrega,omitempty"`
	DhHashTentativaEntrega string   `json:"dhHashTentativaEntrega,omitempty" fmt:"data-hora"`
	Documentos             []string `json:"documentos"` // chaves de NF-e não entregues
	NSeqEvento             int      `json:"nSeqEvento,omitempty"`
}

// ToINIInsucessoEntrega monta o INI do insucesso de entrega (110190): campos na
// [EVENTO001] + uma seção [infEntrega0001..] por NF-e.
func ToINIInsucessoEntrega(chave, cnpj, dhEvento string, p PedidoInsucessoEntrega) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110190", p.NSeqEvento)
	b.kvOpt("nProt", p.NProt)
	b.kvOpt("dhTentativaEntrega", p.DhTentativaEntrega)
	b.kvIntOpt("nTentativa", p.NTentativa)
	b.kv("tpMotivo", defaultStr(p.TpMotivo, "1"))
	b.kvOpt("xJustMotivo", p.XJustMotivo)
	b.kvOpt("latitude", coord(p.Latitude))
	b.kvOpt("longitude", coord(p.Longitude))
	b.kvOpt("hashTentativaEntrega", p.HashTentativaEntrega)
	b.kvOpt("dhHashTentativaEntrega", p.DhHashTentativaEntrega)
	for i, ch := range p.Documentos {
		b.section("infEntrega" + seq4(i+1))
		b.kv("chNFe", ch)
	}
	return b.String()
}
