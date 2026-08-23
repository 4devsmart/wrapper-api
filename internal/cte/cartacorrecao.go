package cte

import (
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoCartaCorrecao é o corpo de POST /v1/cte/eventos/carta-correcao. A chave vem no envelope; o CNPJ é
// derivado dela. Aqui vão só os grupos de correção.
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
		b.Secao("DETEVENTO" + inifmt.Seq3(i+1))
		b.KV("grupoAlterado", c.GrupoAlterado)
		b.KV("campoAlterado", c.CampoAlterado)
		b.KV("valorAlterado", c.ValorAlterado)
		b.KV("nroItemAlterado", strconv.Itoa(c.NroItemAlterado))
	}
	return b.String()
}
