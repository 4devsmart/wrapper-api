package mdfe

import (
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
)

// Lockstep espelhado do MDF-e: do CONTRATO para o INI. Ver o comentário em
// internal/cte/espelho_test.go para o porquê desta direção.
var gruposMDFe = espelho.Grupos{}

func TestEspelho_MDFe_ToINI(t *testing.T) {
	espelho.Conferir(t, espelho.Caso{
		Nome:       "mdfe.ToINI",
		Novo:       func() any { return &PedidoEmissao{} },
		Gerar:      func(a any) string { return ToINI(*a.(*PedidoEmissao)) },
		Grupos:     gruposMDFe,
		Permitidas: "testdata/nao_espelhadas.tsv",
	})
}
