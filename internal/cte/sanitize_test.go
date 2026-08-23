package cte

import (
	"strings"
	"testing"
)

// A sanitização em si vive em internal/platform/inifmt e é testada lá. Este
// teste guarda o OUTRO lado: que o construtor DESTE documento continue passando
// por ela.
//
// Antes ele exercitava o apelido local sanitizeINIVal, que era uma linha
// encaminhando para o inifmt: cobertura aparente, porque trocar a escrita real
// por WriteString direto passaria. Agora a injeção entra pelo pedido e sai (ou
// não) no arquivo intermediário, que é onde o estrago aconteceria.
func TestINI_NaoDeixaTextoLivreForjarSecao(t *testing.T) {
	p := pedidoFixture()
	p.InfCte.Emit.XNome = "Transportadora\n[emit]\nCNPJ=00000000000191\rmais"

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
