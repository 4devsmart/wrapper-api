package nfse

import (
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
)

// Lockstep espelhado da NFS-e: do CONTRATO para o INI. Ver o comentário em
// internal/cte/espelho_test.go para o porquê desta direção.
//
// São dois construtores sobre o MESMO modelo, e é por isso que há duas listas:
// um campo do Padrão Nacional não tem para onde ir no RPS do ABRASF, e o que o
// ABRASF exige o Padrão Nacional resolve de outro jeito. Confundir as duas
// esconderia exatamente o campo esquecido em um dos dois.
var gruposNFSe = espelho.Grupos{
	// O grupo tribMun detalhado SUBSTITUI os atalhos de [Valores]: informado ele,
	// tribISSQN e pAliq saem de lá e não do nível de cima. Sem declarar isso, os
	// atalhos pareceriam esquecidos.
	"Valores": {
		{"TribMun", "TribISSQN"},
		{"TribMun", "PAliq"},
	},
	// Município brasileiro fixa o país em 1058, e um cPais mandado junto não vai
	// ao INI (ver codigoPais). O cPais só chega sem município, que é o endereço
	// no exterior.
	"Pessoa": {{"CMun", "CPais"}},
	// Idem no serviço: município brasileiro da prestação fixa o país em 1058, e o
	// codigoPais mandado junto não vai ao INI (ver codigoPaisServico).
	"Servico": {
		{"CMunPrestacao", "CodigoPais"},
		// O INI tem um endereço de execução por ITEM. Com lista de itens, vale o
		// endereço de cada item; sem ela, vale o do serviço, que alimenta o
		// [Itens001] derivado. São alternativas, não campos concorrentes.
		{"Itens", "EnderecoServico"},
	},
}

func TestEspelho_NFSe_PadraoNacional(t *testing.T) {
	espelho.Conferir(t, espelho.Caso{
		Nome:       "nfse.ToINI (Padrão Nacional)",
		Novo:       func() any { return &DPSPedido{} },
		Gerar:      func(a any) string { return ToINI(*a.(*DPSPedido)) },
		Grupos:     gruposNFSe,
		Permitidas: "testdata/nao_espelhadas_pn.tsv",
	})
}

func TestEspelho_NFSe_Abrasf(t *testing.T) {
	espelho.Conferir(t, espelho.Caso{
		Nome:       "nfse.ToINIAbrasf",
		Novo:       func() any { return &DPSPedido{} },
		Gerar:      func(a any) string { return ToINIAbrasf(*a.(*DPSPedido)) },
		Grupos:     gruposNFSe,
		Permitidas: "testdata/nao_espelhadas_abrasf.tsv",
	})
}
