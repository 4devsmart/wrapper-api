package modulo_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const raizMod = "github.com/4devsmart/wrapper-api/"

// modulos são os pacotes de documento fiscal. Ao adicionar um módulo novo
// (MDF-e, NFS-e, boletos), acrescente-o aqui — é o que faz este teste valer.
var modulos = []string{"cte", "mdfe", "nfse", "boleto"}

// Os quatro já estão portados; a lista acima é por PACOTE (o módulo boleto se
// chama "boletos" na rota, mas o pacote é internal/boleto).

// TestModuloNaoImportaModulo tranca a fronteira que dá sentido à modularização.
//
// O projeto de origem tinha um handlers.go de 45 KB atendendo todos os
// documentos: mexer num campo de CT-e tocava dois pacotes e arriscava a NFS-e.
// A regra aqui é o que impede isso de voltar — e uma regra sem teste é um
// comentário. O que for comum sobe para platform/ (sem domínio fiscal) ou
// fiscal/ (com domínio, sem documento).
func TestModuloNaoImportaModulo(t *testing.T) {
	for _, m := range modulos {
		dir := filepath.Join("..", m)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // módulo ainda não portado
		}
		for _, imp := range importsDe(t, dir) {
			for _, outro := range modulos {
				if outro == m {
					continue
				}
				if imp == raizMod+"internal/"+outro {
					t.Errorf("internal/%s importa internal/%s — módulos não podem se conhecer; "+
						"mova o compartilhado para internal/fiscal ou internal/platform", m, outro)
				}
			}
		}
	}
}

// TestPlatformNaoConheceDominioFiscal mantém platform/ genérico. Se ele passar a
// importar acbr/fiscal/documento, deixa de ser infraestrutura reaproveitável e
// vira mais um lugar onde regra fiscal se esconde.
func TestPlatformNaoConheceDominioFiscal(t *testing.T) {
	proibidos := append([]string{"internal/acbr", "internal/fiscal"}, prefixados("internal/", modulos)...)

	raiz := filepath.Join("..", "platform")
	err := filepath.WalkDir(raiz, func(caminho string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		for _, imp := range importsDe(t, caminho) {
			for _, p := range proibidos {
				if imp == raizMod+p {
					t.Errorf("%s importa %s — platform/ não pode conhecer domínio fiscal", caminho, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestModuloNaoImportaServidor impede a inversão: é o servidor que monta os
// módulos, nunca o contrário. Um módulo que importa o servidor não é montável
// isolado — e deixa de ser testável sem subir a API inteira.
func TestModuloNaoImportaServidor(t *testing.T) {
	for _, m := range modulos {
		dir := filepath.Join("..", m)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		for _, imp := range importsDe(t, dir) {
			if imp == raizMod+"internal/servidor" {
				t.Errorf("internal/%s importa internal/servidor — a dependência é no sentido oposto", m)
			}
		}
	}
}

// importsDe devolve os imports de um diretório de pacote, incluindo os de teste
// e os que só existem sob build tag (o binding cgo entra por aqui).
func importsDe(t *testing.T, dir string) []string {
	t.Helper()
	ctx := build.Default
	ctx.UseAllFiles = true // ignora build tags: queremos ver TODOS os arquivos
	p, err := ctx.ImportDir(dir, 0)
	if err != nil {
		if _, ok := err.(*build.NoGoError); ok {
			return nil
		}
		// UseAllFiles junta arquivos de builds incompatíveis; os imports ainda
		// são coletados, então um erro de multi-package aqui não invalida a lista.
		if p == nil || (len(p.Imports) == 0 && len(p.TestImports) == 0) {
			t.Logf("%s: %v", dir, err)
			return nil
		}
	}
	var todos []string
	todos = append(todos, p.Imports...)
	todos = append(todos, p.TestImports...)
	todos = append(todos, p.XTestImports...)
	return todos
}

func prefixados(prefixo string, vs []string) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, prefixo+strings.TrimPrefix(v, prefixo))
	}
	return out
}
