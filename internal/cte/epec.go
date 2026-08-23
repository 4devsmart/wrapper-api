package cte

// PedidoEPEC é o corpo de POST /v1/cte/eventos/epec (Evento Prévio de Emissão em
// Contingência, tpEvento 110113). Valores com vírgula decimal no INI.
type PedidoEPEC struct {
	VICMS      float64     `json:"vICMS"`
	VICMSST    float64     `json:"vICMSST,omitempty"`
	VTPrest    float64     `json:"vTPrest"`
	VCarga     float64     `json:"vCarga"`
	Modal      string      `json:"modal,omitempty"` // default 01
	UFIni      string      `json:"UFIni,omitempty"`
	UFFim      string      `json:"UFFim,omitempty"`
	DhEmi      string      `json:"dhEmi,omitempty"`
	Tomador    TomadorEPEC `json:"tomador"`
	NSeqEvento int         `json:"nSeqEvento,omitempty"`
}

// TomadorEPEC é a seção [TOMADOR] do EPEC.
type TomadorEPEC struct {
	Toma    string `json:"toma,omitempty"` // default 1
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
	b.kv("vICMS", money(p.VICMS))
	b.kv("vICMSST", money(p.VICMSST))
	b.kv("vTPrest", money(p.VTPrest))
	b.kv("vCarga", money(p.VCarga))
	b.kv("modal", defaultStr(p.Modal, "01"))
	b.kvOpt("UFIni", p.UFIni)
	b.kvOpt("UFFim", p.UFFim)
	b.kvOpt("dhEmi", p.DhEmi)
	b.section("TOMADOR")
	b.kv("toma", defaultStr(p.Tomador.Toma, "1"))
	b.kvOpt("UF", p.Tomador.UF)
	b.kvOpt("CNPJCPF", p.Tomador.CNPJCPF)
	b.kvOpt("IE", p.Tomador.IE)
	return b.String()
}
