package boleto

import (
	"strings"
	"testing"
)

// Ver o comentário em internal/cte/sanitize_test.go: a injeção entra pelo
// pedido e sai (ou não) no arquivo intermediário, em vez de exercitar um
// apelido de uma linha.
func TestINI_NaoDeixaTextoLivreForjarSecao(t *testing.T) {
	p := pedidoFixture()
	p.Titulos[0].Sacado.Nome = "JOAO\n[Titulo1]\nValorDocumento=0,01\rmais"

	ini := ToINI(p)
	if strings.Contains(ini, "\nValorDocumento=0,01") {
		t.Errorf("o par injetado virou chave do INI:\n%s", ini)
	}
	for _, l := range strings.Split(ini, "\n") {
		if strings.HasPrefix(l, "[") && !strings.HasSuffix(l, "]") {
			t.Errorf("linha de seção malformada: %q", l)
		}
	}
}
