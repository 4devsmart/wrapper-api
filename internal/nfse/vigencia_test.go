package nfse

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Uma linha errada aqui desvia nota de verdade, e sem erro de compilação.
func TestVigencias_LinhasValidas(t *testing.T) {
	vistas := map[string]bool{}
	for _, v := range vigencias {
		nome := v.Municipio + "/" + v.Desde
		if _, ok := LayoutDoMunicipio(v.Municipio); !ok {
			t.Errorf("%s: município fora da tabela de provedores", nome)
		}
		// O layout não serve aqui: o ISSNet de Brasília também é padrao_nacional.
		if strings.EqualFold(provedorDoMunicipio(v.Municipio), "PadraoNacional") {
			t.Errorf("%s: já é do Padrão Nacional, a linha não muda nada", nome)
		}
		if v.OpSimpNac < 1 || v.OpSimpNac > 3 {
			t.Errorf("%s: opSimpNac %d não existe", nome, v.OpSimpNac)
		}
		if _, err := time.Parse(time.DateOnly, v.Desde); err != nil {
			t.Errorf("%s: Desde tem de ser AAAA-MM-DD: %v", nome, err)
		}
		if strings.TrimSpace(v.Base) == "" {
			t.Errorf("%s: sem Base, ninguém saberá por que a linha existe", nome)
		}
		chave := fmt.Sprint(v.Municipio, v.OpSimpNac)
		if vistas[chave] {
			t.Errorf("%s: regime repetido no município", nome)
		}
		vistas[chave] = true
	}
}

// É a promessa da tabela: município novo é uma linha, sem tocar em handler.
func TestVigencias_UmaLinhaBastaParaOutroMunicipio(t *testing.T) {
	const competencia = "2027-01-01"
	original := vigencias
	t.Cleanup(func() { vigencias = original })
	vigencias = append(vigencias[:len(vigencias):len(vigencias)], Vigencia{
		Municipio: munAbrasf, OpSimpNac: opSimpNacMEEPP, Desde: competencia, Base: "teste",
	})

	rota, ok := RotaDoMunicipio(munAbrasf)
	if !ok {
		t.Fatalf("%s fora da tabela", munAbrasf)
	}
	casos := []struct {
		nome    string
		dCompet string
		op      int
		sessao  string
	}{
		{"antes da vigência", "2026-12-31", opSimpNacMEEPP, munAbrasf},
		{"na vigência", competencia, opSimpNacMEEPP, SessaoNacional},
		{"outro regime", competencia, 2, munAbrasf},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := rota.DaDPSPronta(dpsDe(munAbrasf, c.dCompet, c.op))
			if got.Sessao != c.sessao {
				t.Errorf("sessão %s, quero %s", got.Sessao, c.sessao)
			}
		})
	}
	if s := rota.DaNota(chaveDe(munAbrasf, "2")).Sessao; s != SessaoNacional {
		t.Errorf("nota nacional na sessão %s, quero %s", s, SessaoNacional)
	}
	if s := rota.SessoesDaConsultaDPS(); len(s) != 2 || s[0] != SessaoNacional {
		t.Errorf("consulta-dps em %v, quero o nacional primeiro", s)
	}
}
