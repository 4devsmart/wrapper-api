package boleto

import (
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
)

// Lockstep espelhado do boleto: do CONTRATO para o INI. Ver o comentário em
// internal/cte/espelho_test.go para o porquê desta direção.
func TestEspelho_Boleto_ToINI(t *testing.T) {
	espelho.Conferir(t, espelho.Caso{
		Nome:       "boleto.ToINI",
		Novo:       func() any { return &Pedido{} },
		Gerar:      func(a any) string { return ToINI(*a.(*Pedido)) },
		Permitidas: "testdata/nao_espelhadas.tsv",
	})
}
