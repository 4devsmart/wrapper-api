package mdfe

import (
	"strings"
	"testing"
)

// Ver o comentário em internal/cte/sanitize_test.go: a injeção entra pelo
// pedido e sai (ou não) no arquivo intermediário, em vez de exercitar um
// apelido de uma linha.
func TestINI_NaoDeixaTextoLivreForjarSecao(t *testing.T) {
	p := pedidoFixture()
	p.InfMDFe.Emit.XNome = "Transportadora\n[emit]\nCNPJ=00000000000191\rmais"

	ini := ToINI(p)
	if n := contaSecao(ini, "[emit]"); n != 1 {
		t.Errorf("o texto livre forjou seção: %d cabeçalhos [emit]\n%s", n, ini)
	}
	if strings.Contains(ini, "\nCNPJ=00000000000191") {
		t.Errorf("o par injetado virou chave do INI:\n%s", ini)
	}
}

// secoes devolve os nomes das seções do INI: só as linhas que SÃO cabeçalho,
// não um "[emit]" que aparece dentro de um valor.
func secoes(ini string) []string {
	var out []string
	for _, l := range strings.Split(ini, "\n") {
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			out = append(out, l)
		}
	}
	return out
}

// conta quantas vezes a seção aparece como CABEÇALHO.
func contaSecao(ini, nome string) int {
	n := 0
	for _, s := range secoes(ini) {
		if s == nome {
			n++
		}
	}
	return n
}
