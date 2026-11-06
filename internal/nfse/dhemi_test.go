package nfse

import (
	"regexp"
	"strings"
	"testing"
)

var reDataEmissao = regexp.MustCompile(`(?m)^DataEmissao=(.*)$`)

func dataEmissaoDoINI(t *testing.T, ini string) string {
	t.Helper()
	m := reDataEmissao.FindStringSubmatch(ini)
	if len(m) != 2 {
		t.Fatalf("INI sem DataEmissao:\n%s", ini)
	}
	return strings.TrimSpace(m[1])
}

// A hora da emissão chega à lib, nos dois construtores. Ela já foi cortada aqui,
// e toda DPS saía às 00:00:00. O fuso é o da UF do prestador, a mesma que a lib
// usa para carimbar o offset: com outro, o XML declararia um instante diferente
// do pedido, e uma hora a mais é emissão no futuro em quem está a oeste.
func TestDataEmissaoLevaAHoraNoFusoDoPrestador(t *testing.T) {
	casos := []struct {
		nome, uf, dhEmi, quero string
	}{
		{"instante em Brasília", "DF", "2026-11-05T13:22:07-03:00", "05/11/2026 13:22:07"},
		{"instante convertido para Manaus", "AM", "2026-11-05T13:22:07-03:00", "05/11/2026 12:22:07"},
		{"instante convertido para o Acre", "AC", "2026-11-05T13:22:07-03:00", "05/11/2026 11:22:07"},
		{"UTC vira hora local", "SP", "2026-11-05T16:22:07Z", "05/11/2026 13:22:07"},
		{"conversão atravessa o dia", "AM", "2026-11-05T00:30:00-03:00", "04/11/2026 23:30:00"},
		{"relógio de parede fica como veio", "AM", "2026-11-05T13:22:07", "05/11/2026 13:22:07"},
		{"só a data segue à meia-noite", "AM", "2026-11-05", "05/11/2026 00:00:00"},
		{"formato brasileiro segue à meia-noite", "DF", "05/11/2026 00:00:00", "05/11/2026 00:00:00"},
		{"sem UF cai em Brasília", "", "2026-11-05T13:22:07-03:00", "05/11/2026 13:22:07"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var p DPSPedido
			p.InfDPS.DhEmi = c.dhEmi
			p.InfDPS.Prest.UF = c.uf
			if got := dataEmissaoDoINI(t, ToINI(p)); got != c.quero {
				t.Errorf("Padrão Nacional: DataEmissao = %q, quero %q", got, c.quero)
			}
			if got := dataEmissaoDoINI(t, ToINIAbrasf(p)); got != c.quero {
				t.Errorf("ABRASF: DataEmissao = %q, quero %q", got, c.quero)
			}
		})
	}
}

// Sem dhEmi vale o momento da geração, com hora: é o que a lib recebia cortado.
func TestDataEmissaoVaziaEhAgora(t *testing.T) {
	var p DPSPedido
	p.InfDPS.Prest.UF = "DF"
	if got := dataEmissaoDoINI(t, ToINI(p)); !regexp.MustCompile(`^\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2}$`).MatchString(got) {
		t.Errorf("DataEmissao = %q, quero data e hora", got)
	}
}

// A competência continua sendo só a data, e no fuso do prestador: a nota das
// 00:30 de Brasília ainda é do dia anterior em Manaus.
func TestCompetenciaSegueSemHora(t *testing.T) {
	var p DPSPedido
	p.InfDPS.DCompet = "2026-11-05T00:30:00-03:00"
	p.InfDPS.Prest.UF = "AM"
	if ini := ToINI(p); !strings.Contains(ini, "Competencia=04/11/2026\n") {
		t.Errorf("competência errada:\n%s", ini)
	}
}
