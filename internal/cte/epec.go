package cte

import (
	"cmp"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoEPEC é o corpo de POST /v1/cte/eventos/epec (Evento Prévio de Emissão em
// Contingência, tpEvento 110113). Valores com vírgula decimal no INI.
type PedidoEPEC struct {
	VICMS      float64     `json:"vICMS"`
	VICMSST    float64     `json:"vICMSST,omitempty"`
	VTPrest    float64     `json:"vTPrest"`
	VCarga     float64     `json:"vCarga"`
	Modal      string      `json:"modal,omitempty" enum:"Modal"` // default 01
	UFIni      string      `json:"UFIni,omitempty"`
	UFFim      string      `json:"UFFim,omitempty"`
	DhEmi      string      `json:"dhEmi,omitempty" fmt:"data-hora"`
	Tomador    TomadorEPEC `json:"tomador"`
	NSeqEvento int         `json:"nSeqEvento,omitempty"`
}

// TomadorEPEC é a seção [TOMADOR] do EPEC.
type TomadorEPEC struct {
	Toma    string `json:"toma,omitempty" enum:"Tomador"` // default 1
	UF      string `json:"UF,omitempty"`
	CNPJCPF string `json:"CNPJCPF,omitempty"`
	IE      string `json:"IE,omitempty"`
}

// ToINIEPEC monta o INI do EPEC (110113), indexado: campos do detEvento na
// [EVENTO001] + seção fixa [TOMADOR]. Ver
// .claude/skills/acbr-especialista/referencia/cte-eventos.md.
func ToINIEPEC(chave, cnpj, dhEvento string, p PedidoEPEC) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110113", p.NSeqEvento)
	b.KV("vICMS", inifmt.Money(p.VICMS))
	b.KV("vICMSST", inifmt.Money(p.VICMSST))
	b.KV("vTPrest", inifmt.Money(p.VTPrest))
	b.KV("vCarga", inifmt.Money(p.VCarga))
	b.KV("modal", cmp.Or(p.Modal, "01"))
	b.KVOpt("UFIni", p.UFIni)
	b.KVOpt("UFFim", p.UFFim)
	b.KVOpt("dhEmi", p.DhEmi)
	b.Secao("TOMADOR")
	b.KV("toma", cmp.Or(p.Tomador.Toma, "1"))
	b.KVOpt("UF", p.Tomador.UF)
	b.KVOpt("CNPJCPF", p.Tomador.CNPJCPF)
	b.KVOpt("IE", p.Tomador.IE)
	return b.String()
}
