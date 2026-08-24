package versao

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A versão do Go está fixada em dois lugares que ninguém liga: o Dockerfile,
// que compila o binário PUBLICADO, e os workflows, que compilam o que os testes
// exercitam. O CI nunca constrói a imagem, então uma divergência entre os dois
// passa verde: os testes rodam num toolchain e o artefato sai de outro.
//
// Não é hipótese. Um bump automático chegou mexendo só no Dockerfile, com dez
// checks verdes que não tocaram em uma linha do que ele mudava. E os CVEs de
// stdlib que o govulncheck aponta foram conferidos como corrigidos numa versão
// ESPECÍFICA: trocar o compilador da imagem sem o CI acompanhar apaga essa
// verificação sem avisar ninguém.

const raiz = "../../../"

var (
	reImagemGo = regexp.MustCompile(`(?m)^FROM\s+\S*\s*golang:([0-9]+\.[0-9]+\.[0-9]+)`)
	reGoVersao = regexp.MustCompile(`go-version:\s*"([0-9]+\.[0-9]+\.[0-9]+)"`)
)

func TestVersaoDoGo_DockerfileEWorkflowsConcordam(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join(raiz, "Dockerfile"))
	if err != nil {
		t.Fatalf("Dockerfile: %v", err)
	}
	naImagem := reImagemGo.FindAllStringSubmatch(string(dockerfile), -1)
	if len(naImagem) == 0 {
		t.Fatal("nenhum 'FROM golang:X.Y.Z' no Dockerfile: o padrão mudou e este teste " +
			"parou de olhar o que deveria")
	}

	vistos := map[string][]string{}
	for _, m := range naImagem {
		vistos[m[1]] = append(vistos[m[1]], "Dockerfile")
	}

	fluxos, err := filepath.Glob(filepath.Join(raiz, ".github/workflows/*.yml"))
	if err != nil || len(fluxos) == 0 {
		t.Fatalf("nenhum workflow encontrado: %v", err)
	}
	achouNoCI := false
	for _, f := range fluxos {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range reGoVersao.FindAllStringSubmatch(string(b), -1) {
			achouNoCI = true
			vistos[m[1]] = append(vistos[m[1]], filepath.Base(f))
		}
	}
	if !achouNoCI {
		t.Fatal("nenhum 'go-version: \"X.Y.Z\"' nos workflows: ou o CI deixou de fixar a " +
			"versão, ou este teste parou de enxergá-la")
	}

	if len(vistos) == 1 {
		return
	}

	versoes := make([]string, 0, len(vistos))
	for v := range vistos {
		versoes = append(versoes, v)
	}
	sort.Strings(versoes)
	t.Errorf("a versão do Go diverge entre o que constrói o binário publicado e o que o CI testa:")
	for _, v := range versoes {
		onde := vistos[v]
		sort.Strings(onde)
		t.Errorf("  %s  em %s", v, strings.Join(dedup(onde), ", "))
	}
	t.Errorf("suba as duas juntas: o CI precisa exercitar o toolchain que produz a imagem")
}

func dedup(s []string) []string {
	fora := s[:0]
	var ultimo string
	for _, v := range s {
		if v != ultimo {
			fora = append(fora, v)
		}
		ultimo = v
	}
	return fora
}
