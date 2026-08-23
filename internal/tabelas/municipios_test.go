package tabelas

import "testing"

func TestMunicipiosSeed(t *testing.T) {
	m := MunicipiosSeed()
	// O IBGE tem 5.570+ municípios; garantimos que o TSV embutido carregou.
	if len(m) < 5000 {
		t.Fatalf("seed parseou poucos municípios: %d", len(m))
	}
	for _, x := range m {
		if len(x.Codigo) != 7 || x.Nome == "" || len(x.UF) != 2 {
			t.Fatalf("linha mal formada: %+v", x)
		}
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"São Paulo":    "sao paulo",
		"BRASÍLIA":     "brasilia",
		"Açaí D'Oeste": "acai doeste",
		"  Içara  ":    "icara",
	}
	for in, want := range casos {
		if got := Normalizar(in); got != want {
			t.Errorf("Normalizar(%q) = %q, esperado %q", in, got, want)
		}
	}
}
