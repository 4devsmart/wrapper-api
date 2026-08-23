package cte

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
)

// Lockstep espelhado do CT-e: do CONTRATO para o INI.
//
// O lockstep de lerini_chaves.tsv compara a biblioteca com o builder, e nenhum
// dos dois é o contrato: um campo que existe só no modelo Go não aparece em
// nenhum dos dois conjuntos, então as duas comparações passam enquanto o dado
// morre na tradução. Aqui o modelo inteiro é preenchido e cada valor tem de
// reaparecer no INI.

// Grupos mutuamente exclusivos: o builder escolhe um e ignora o resto, então
// cada passada exercita uma alternativa diferente.
var gruposCTe = espelho.Grupos{
	// O layout é ESCOLHA entre o CT-e Normal e o Complementar: infCteComp só
	// existe com tpCTe=1, e nesse caso infCTeNorm não existe. Verificado contra
	// a biblioteca: com o CT-e Normal, o grupo complementar sai do XML.
	"InfCte": {{"InfCTeNorm", "InfCteComp"}},
	"ICMS":   {{"ICMS00", "ICMS20", "ICMS45", "ICMS60", "ICMS90", "ICMSOutraUF", "ICMSSN"}},
	"Entrega": {
		{"SemData", "ComData", "NoPeriodo"},
		{"SemHora", "ComHora", "NoInter"},
	},
}

func TestEspelho_CTe_ToINI(t *testing.T) {
	espelho.Conferir(t, espelho.Caso{
		Nome:       "cte.ToINI",
		Novo:       func() any { return &PedidoEmissao{} },
		Gerar:      func(a any) string { return ToINI(*a.(*PedidoEmissao)) },
		Grupos:     gruposCTe,
		Permitidas: "testdata/nao_espelhadas.tsv",
	})
}

// O espelho prova que cada grupo chega ao INI na SUA passada. O que ele não
// cobre é o pedido contraditório: o cliente que manda o complemento num CT-e
// Normal. A biblioteca resolve pelo tpCTe e descarta o outro grupo em silêncio,
// que é a mesma classe de defeito, com a diferença de estar na entrada.
func TestGrupoDoTipoCoerente(t *testing.T) {
	casos := []struct {
		nome  string
		tpCTe int
		comp  bool
		norm  bool
		quero int
	}{
		{"normal com infCTeNorm", 0, false, true, 0},
		{"complementar com infCteComp", tipoCTeComplementar, true, false, 0},
		{"complemento num CT-e normal", 0, true, true, http.StatusBadRequest},
		{"complementar trazendo o grupo normal", tipoCTeComplementar, true, true, http.StatusBadRequest},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := pedidoFixture()
			p.InfCte.Ide.TpCTe = c.tpCTe
			if c.comp {
				p.InfCte.InfCteComp = []InfCteComp{{ChCTe: chaveDeTeste}}
			}
			if !c.norm {
				p.InfCte.InfCTeNorm = nil
			}
			w := httptest.NewRecorder()
			ok := grupoDoTipoCoerente(w, p)
			if c.quero == 0 {
				if !ok {
					t.Fatalf("pedido coerente recusado: %s", w.Body)
				}
				return
			}
			if ok {
				t.Fatal("pedido contraditório passou: o grupo seria descartado sem aviso")
			}
			if w.Code != c.quero {
				t.Errorf("status %d, quero %d", w.Code, c.quero)
			}
			if !strings.Contains(w.Body.String(), "grupo_incompativel") {
				t.Errorf("a resposta precisa trazer o código estável: %s", w.Body)
			}
		})
	}
}

const chaveDeTeste = "35200714200166000187570010000000123456789012"
