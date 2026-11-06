package tabelas

import "testing"

func TestProvedorNFSe(t *testing.T) {
	// Cabixi/RO (1100031) é Padrão Nacional na tabela do ACBr.
	if got := ProvedorNFSe("1100031"); got != "PadraoNacional" {
		t.Errorf("provedor de 1100031 = %q, quero PadraoNacional", got)
	}
	// Código inexistente / sem provedor configurado → vazio.
	if got := ProvedorNFSe("0000000"); got != "" {
		t.Errorf("provedor de código inexistente = %q, quero vazio", got)
	}
}

// A lista é a base da integração por combinação de provedor: só município com
// provedor, e com a versão do layout preenchida a partir da tabela.
func TestMunicipiosNFSe(t *testing.T) {
	m := MunicipiosNFSe()
	if len(m) < 4000 {
		t.Fatalf("len = %d, a tabela encolheu", len(m))
	}
	for _, x := range m {
		if x.Provedor == "" || x.UF == "" {
			t.Fatalf("município incompleto: %+v", x)
		}
		if x.Provedor != ProvedorNFSe(x.Codigo) {
			t.Fatalf("%s: provedor %q difere da tabela", x.Codigo, x.Provedor)
		}
	}
}
