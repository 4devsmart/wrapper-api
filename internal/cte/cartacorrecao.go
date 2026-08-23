package cte

import "strconv"

// PedidoCartaCorrecao é o corpo de POST /cte/{id}/carta-correcao. chCTe/cnpj
// vêm do documento; o cliente envia o(s) grupo(s) de correção.
type PedidoCartaCorrecao struct {
	Correcoes  []Correcao `json:"correcoes"`
	NSeqEvento int        `json:"nSeqEvento,omitempty"` // opcional (default 1)
}

// Correcao é um item de correção da CC-e (grupo [DETEVENTOxxx] no INI).
// grupoAlterado/campoAlterado/valorAlterado conforme o leiaute do CT-e;
// nroItemAlterado = nº do item quando o campo é de uma lista (0 se não se aplica).
type Correcao struct {
	GrupoAlterado   string `json:"grupoAlterado"`
	CampoAlterado   string `json:"campoAlterado"`
	ValorAlterado   string `json:"valorAlterado"`
	NroItemAlterado int    `json:"nroItemAlterado,omitempty"`
}

// ToINICartaCorrecao monta o INI da CC-e (tpEvento 110110) no formato indexado:
// [EVENTO]/[EVENTO001] (cabeçalho) + [DETEVENTO001..] (uma por correção). Ver
// .claude/skills/acbr-especialista/referencia/cte-eventos.md.
func ToINICartaCorrecao(chave, cnpj, dhEvento string, p PedidoCartaCorrecao) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110110", p.NSeqEvento)
	for i, c := range p.Correcoes {
		b.section("DETEVENTO" + seq(i+1))
		b.kv("grupoAlterado", c.GrupoAlterado)
		b.kv("campoAlterado", c.CampoAlterado)
		b.kv("valorAlterado", c.ValorAlterado)
		b.kv("nroItemAlterado", strconv.Itoa(c.NroItemAlterado))
	}
	return b.String()
}
