package nfse

import (
	"strings"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// Os dois lados: NT que a lib liga sem linha aqui é mudança de XML sem decisão, e
// linha aqui sem a lib aplicar é promessa que o documento não cumpre.
func TestNotasTecnicas_CobremOQueALibAplica(t *testing.T) {
	daTabela := map[string]bool{}
	for _, n := range notasTecnicas {
		daTabela[n.Provedor+"|"+n.Ambiente+"|"+n.Versao] = true
	}
	daLib := map[string]bool{}
	for _, n := range tabelas.NotasTecnicasDaLib() {
		chave := n.Provedor + "|" + n.Ambiente + "|" + n.Versao
		daLib[chave] = true
		if !daTabela[chave] {
			t.Errorf("a lib aplica %s e notasTecnicas não registra: decida e escreva a linha", chave)
		}
	}
	for chave := range daTabela {
		if !daLib[chave] {
			t.Errorf("notasTecnicas registra %s e a lib não aplica", chave)
		}
	}
}

func TestNotasTecnicas_LinhasValidas(t *testing.T) {
	for _, n := range notasTecnicas {
		nome := n.Provedor + "/" + n.Ambiente
		if n.Ambiente != fiscal.AmbienteProducao && n.Ambiente != fiscal.AmbienteHomologacao {
			t.Errorf("%s: ambiente inválido", nome)
		}
		if strings.TrimSpace(n.Base) == "" {
			t.Errorf("%s: sem Base, ninguém saberá por que a linha existe", nome)
		}
		if n.Desde != "" {
			if _, err := time.Parse(time.DateOnly, n.Desde); err != nil {
				t.Errorf("%s: Desde tem de ser AAAA-MM-DD: %v", nome, err)
			}
		}
	}
}

func TestNotaTecnicaDe_LeProvedorEAmbiente(t *testing.T) {
	original := notasTecnicas
	t.Cleanup(func() { notasTecnicas = original })
	notasTecnicas = []NotaTecnica{{
		Provedor: "PadraoNacional", Ambiente: fiscal.AmbienteHomologacao, Versao: "009.1.01", Base: "teste",
	}}

	if got := NotaTecnicaDe("PadraoNacional", fiscal.AmbienteHomologacao); got != "009.1.01" {
		t.Errorf("homologação = %q", got)
	}
	if got := NotaTecnicaDe("PadraoNacional", fiscal.AmbienteProducao); got != "" {
		t.Errorf("produção = %q: a NT não pode valer em ambiente sem linha", got)
	}
	if got := NotaTecnicaDe("ISSNet", fiscal.AmbienteHomologacao); got != "" {
		t.Errorf("outro provedor = %q", got)
	}
}
