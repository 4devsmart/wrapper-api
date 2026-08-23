package cte

import (
	"cmp"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoInsucessoEntrega é o corpo de POST /v1/cte/eventos/insucesso-entrega
// (tpEvento 110190). Registra a tentativa frustrada de entrega.
type PedidoInsucessoEntrega struct {
	NProt                  string   `json:"nProt,omitempty"`
	DhTentativaEntrega     string   `json:"dhTentativaEntrega,omitempty" fmt:"data-hora"`
	NTentativa             int      `json:"nTentativa,omitempty"`
	TpMotivo               string   `json:"tpMotivo,omitempty" enum:"TipoMotivoInsucesso"` // default 1
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
	b.KVOpt("nProt", p.NProt)
	b.KVOpt("dhTentativaEntrega", p.DhTentativaEntrega)
	b.KVIntOpt("nTentativa", p.NTentativa)
	b.KV("tpMotivo", cmp.Or(p.TpMotivo, "1"))
	b.KVOpt("xJustMotivo", p.XJustMotivo)
	b.KVOpt("latitude", coord(p.Latitude))
	b.KVOpt("longitude", coord(p.Longitude))
	b.KVOpt("hashTentativaEntrega", p.HashTentativaEntrega)
	b.KVOpt("dhHashTentativaEntrega", p.DhHashTentativaEntrega)
	for i, ch := range p.Documentos {
		b.Secao("infEntrega" + inifmt.Seq4(i+1))
		b.KV("chNFe", ch)
	}
	return b.String()
}
