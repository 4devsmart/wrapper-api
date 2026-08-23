package tabelas

import "testing"

func TestFamiliaDoProvedor(t *testing.T) {
	// As três primeiras mudaram no bump para r47859 — não porque o ACBr mudou de
	// ideia, mas porque a tabela passou a ser derivada da HERANÇA DE CLASSES em
	// vez dos comentários do ProviderManager, que estavam incompletos. Fiorilli e
	// ISSCampinas têm classe descendente de Proprio E de ABRASF; Tinus declara
	// TACBrNFSeProviderTinus(ABRASFv1) e Tinus203(ABRASFv2). Nenhuma delas muda o
	// roteamento: LayoutBase() colapsa proprio_abrasf→"proprio" e
	// abrasf_v1_v2→"abrasf".
	casos := map[string]Familia{
		"PadraoNacional": FamiliaPadraoNacional,
		"Fiorilli":       FamiliaProprioABRASF,
		"Tinus":          FamiliaABRASFv1v2,
		"WebISS":         FamiliaABRASFv1v2,
		"Betha":          FamiliaABRASFv1v2,
		"ISSCampinas":    FamiliaProprioABRASF,
		"IPM":            FamiliaProprioABRASF,
		"Primax":         FamiliaProprio, // case-insensitive: unit é "PriMax"
		"naoexiste":      FamiliaDesconhecida,
	}
	for prov, want := range casos {
		if got := FamiliaDoProvedor(prov); got != want {
			t.Errorf("FamiliaDoProvedor(%q) = %q, quero %q", prov, got, want)
		}
	}
}

func TestFamiliaLayoutEsuporte(t *testing.T) {
	if !FamiliaPadraoNacional.Suportada() {
		t.Error("Padrão Nacional deveria ser suportado")
	}
	for _, f := range []Familia{FamiliaABRASFv2, FamiliaProprio, FamiliaDesconhecida} {
		if f.Suportada() {
			t.Errorf("família %q não deveria estar suportada ainda", f)
		}
	}
	layout := map[Familia]string{
		FamiliaPadraoNacional: "padrao_nacional",
		FamiliaABRASFv1:       "abrasf",
		FamiliaABRASFv2:       "abrasf",
		FamiliaABRASFv1v2:     "abrasf",
		FamiliaProprio:        "proprio",
		FamiliaProprioABRASF:  "proprio",
		FamiliaDesconhecida:   "",
	}
	for f, want := range layout {
		if got := f.LayoutBase(); got != want {
			t.Errorf("%q.LayoutBase() = %q, quero %q", f, got, want)
		}
	}
}

func TestFamiliaPorMunicipio(t *testing.T) {
	// Cabixi/RO (1100031) é Padrão Nacional.
	if got := FamiliaPorMunicipio("1100031"); got != FamiliaPadraoNacional {
		t.Errorf("família de Cabixi = %q, quero padrao_nacional", got)
	}
	// Campinas/SP (3509502) é ISSCampinas → próprio+ABRASF (LayoutBase "proprio").
	if got := FamiliaPorMunicipio("3509502"); got != FamiliaProprioABRASF {
		t.Errorf("família de Campinas = %q, quero proprio_abrasf", got)
	}
	// Código sem provedor → desconhecida.
	if got := FamiliaPorMunicipio("0000000"); got != FamiliaDesconhecida {
		t.Errorf("família de código inexistente = %q, quero desconhecida", got)
	}
}

func TestProvedoresNFSeTemFamilia(t *testing.T) {
	provs := ProvedoresNFSe()
	// O topo (PadraoNacional) deve vir classificado e suportado.
	if provs[0].Provedor != "PadraoNacional" || !provs[0].Suportado || provs[0].Layout != "padrao_nacional" {
		t.Errorf("topo mal classificado: %+v", provs[0])
	}
	// Todo provedor com município deve ter família conhecida (cobertura da tabela).
	for _, p := range provs {
		if p.Familia == FamiliaDesconhecida {
			t.Errorf("provedor %q (%d municípios) sem família classificada", p.Provedor, p.Municipios)
		}
	}
}
