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

func TestProvedoresNFSe(t *testing.T) {
	provs := ProvedoresNFSe()
	if len(provs) == 0 {
		t.Fatal("nenhum provedor carregado")
	}
	// PadraoNacional é o mais adotado → primeiro da lista.
	if provs[0].Provedor != "PadraoNacional" {
		t.Errorf("primeiro provedor = %q, quero PadraoNacional", provs[0].Provedor)
	}
	// Ordenado por contagem desc.
	for i := 1; i < len(provs); i++ {
		if provs[i-1].Municipios < provs[i].Municipios {
			t.Errorf("provedores fora de ordem em %d: %d < %d", i, provs[i-1].Municipios, provs[i].Municipios)
		}
	}
}

func TestBuscarMunicipios(t *testing.T) {
	// Busca na base completa: deve achar Campinas e anotar o provedor.
	got := BuscarMunicipios("campinas", 8)
	var campinas *MunicipioNFSe
	for i := range got {
		if got[i].Codigo == "3509502" {
			campinas = &got[i]
		}
	}
	if campinas == nil {
		t.Fatal("BuscarMunicipios(campinas) não retornou Campinas")
	}
	if campinas.Provedor != "ISSCampinas" {
		t.Errorf("provedor de Campinas = %q, quero ISSCampinas", campinas.Provedor)
	}

	// Query vazia ou curta → lista vazia (widget só busca com termo).
	if len(BuscarMunicipios("", 8)) != 0 {
		t.Error("query vazia deveria devolver lista vazia")
	}

	// Respeita o limite.
	if n := len(BuscarMunicipios("san", 3)); n > 3 {
		t.Errorf("limite ignorado: %d itens", n)
	}

	// Município sem provedor configurado aparece com Provedor vazio (não some).
	porCodigo := BuscarMunicipios("3509502", 8)
	if len(porCodigo) == 0 {
		t.Error("busca por código IBGE não encontrou o município")
	}
}

func TestListarMunicipiosNFSe(t *testing.T) {
	// Filtro por provedor: todos os itens devem ter o provedor pedido, e o nome
	// canônico do IBGE deve estar preenchido.
	itens, total := ListarMunicipiosNFSe(FiltroMunicipioNFSe{Provedor: "padraonacional", Limit: 10})
	if total < 2000 {
		t.Errorf("total PadraoNacional = %d, esperava > 2000", total)
	}
	if len(itens) != 10 {
		t.Fatalf("página = %d itens, quero 10", len(itens))
	}
	for _, m := range itens {
		if m.Provedor != "PadraoNacional" {
			t.Errorf("item com provedor %q no filtro PadraoNacional", m.Provedor)
		}
		if m.Nome == "" || m.UF == "" {
			t.Errorf("município %s sem nome/UF: %+v", m.Codigo, m)
		}
	}

	// Paginação: offset avança sem repetir o primeiro.
	pag2, _ := ListarMunicipiosNFSe(FiltroMunicipioNFSe{Provedor: "PadraoNacional", Limit: 10, Offset: 10})
	if len(pag2) > 0 && pag2[0].Codigo == itens[0].Codigo {
		t.Error("offset não avançou a página")
	}

	// Filtro por UF + busca por nome.
	porUF, _ := ListarMunicipiosNFSe(FiltroMunicipioNFSe{UF: "sp", Q: "campinas", Limit: 50})
	achou := false
	for _, m := range porUF {
		if m.UF != "SP" {
			t.Errorf("filtro UF=SP retornou %s", m.UF)
		}
		if m.Codigo == "3509502" { // Campinas/SP
			achou = true
		}
	}
	if !achou {
		t.Error("busca uf=SP q=campinas não encontrou Campinas (3509502)")
	}
}
