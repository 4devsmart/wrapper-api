package cte

import (
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoComprovanteEntrega é o corpo de POST /v1/cte/eventos/comprovante-entrega
// (tpEvento 110180). Registra a entrega da carga (com geolocalização opcional)
// e as NF-e entregues.
type PedidoComprovanteEntrega struct {
	NProt         string   `json:"nProt,omitempty"`
	UF            string   `json:"UF,omitempty"`
	DhEntrega     string   `json:"dhEntrega,omitempty" fmt:"data-hora"`
	NDoc          string   `json:"nDoc,omitempty"`
	XNome         string   `json:"xNome,omitempty"`
	Latitude      float64  `json:"latitude,omitempty"`
	Longitude     float64  `json:"longitude,omitempty"`
	HashEntrega   string   `json:"hashEntrega,omitempty"`
	DhHashEntrega string   `json:"dhHashEntrega,omitempty" fmt:"data-hora"`
	Documentos    []string `json:"documentos"` // chaves de NF-e entregues
	NSeqEvento    int      `json:"nSeqEvento,omitempty"`
}

// ToINIComprovanteEntrega monta o INI do comprovante de entrega (110180):
// campos na [EVENTO001] + uma seção [infEntrega0001..] (4 dígitos) por NF-e.
func ToINIComprovanteEntrega(chave, cnpj, dhEvento string, p PedidoComprovanteEntrega) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110180", p.NSeqEvento)
	b.kvOpt("nProt", p.NProt)
	b.kvOpt("UF", p.UF)
	b.kvOpt("dhEntrega", p.DhEntrega)
	b.kvOpt("nDoc", p.NDoc)
	b.kvOpt("xNome", p.XNome)
	b.kvOpt("latitude", coord(p.Latitude))
	b.kvOpt("longitude", coord(p.Longitude))
	b.kvOpt("hashEntrega", p.HashEntrega)
	b.kvOpt("dhHashEntrega", p.DhHashEntrega)
	for i, ch := range p.Documentos {
		b.section("infEntrega" + seq4(i+1))
		b.kv("chNFe", ch)
	}
	return b.String()
}

// coord formata uma coordenada com vírgula decimal (precisão preservada); vazio se 0.
func coord(v float64) string {
	if v == 0 {
		return ""
	}
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}

// seq4 formata índice com 4 dígitos (ex.: [infEntrega0001]).
func seq4(n int) string { return inifmt.Seq4(n) }
