package tabelas

import "testing"

func TestMunicipios(t *testing.T) {
	m := Municipios()
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
