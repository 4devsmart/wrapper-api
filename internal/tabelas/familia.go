package tabelas

import (
	_ "embed"
	"strings"
	"sync"
)

// provedorFamiliaTSV mapeia cada provedor de NFS-e à sua FAMÍLIA de layout,
// classificação extraída da própria fonte oficial (o agrupamento mantido no
// ProviderManager: Padrão Nacional / ABRASF v1 / ABRASF v2 / v1+v2 / próprio).
// É a base do roteamento: a família decide qual tradução JSON→layout aplicar.
//
// Regenerar (em lockstep com a `.so`, ao bumpar a revisão): `make acbr-tabelas`
// — ver scripts/gerar-tabelas-nfse.sh. Já é chamado por `make acbr-libs`.
//
//go:embed provedor_familia.tsv
var provedorFamiliaTSV string

// Familia é a família de layout de um provedor de NFS-e. Determina a tradução
// que aplicamos (entrada JSON → layout) e se já há suporte de emissão.
type Familia string

const (
	FamiliaPadraoNacional Familia = "padrao_nacional" // DPS (SEFIN/ADN)
	FamiliaABRASFv1       Familia = "abrasf_v1"       // RPS/ABRASF layout 1
	FamiliaABRASFv2       Familia = "abrasf_v2"       // RPS/ABRASF layout 2
	FamiliaABRASFv1v2     Familia = "abrasf_v1_v2"    // suporta v1 e v2 (versão por município)
	FamiliaProprio        Familia = "proprio"         // layout próprio do provedor
	FamiliaProprioABRASF  Familia = "proprio_abrasf"  // layout próprio que também faz ABRASF
	FamiliaDesconhecida   Familia = ""                // provedor não classificado
)

// LayoutBase reduz a família às 3 categorias que importam para a CAMADA DE
// ENTRADA (tradução JSON→layout): "padrao_nacional", "abrasf" ou "proprio". A
// distinção v1×v2 é resolvida internamente pela engine por município, então do
// nosso lado v1/v2/v1v2 são o mesmo trabalho de entrada (ABRASF).
func (f Familia) LayoutBase() string {
	switch f {
	case FamiliaPadraoNacional:
		return "padrao_nacional"
	case FamiliaABRASFv1, FamiliaABRASFv2, FamiliaABRASFv1v2:
		return "abrasf"
	case FamiliaProprio, FamiliaProprioABRASF:
		return "proprio"
	default:
		return ""
	}
}

// Suportada indica se a emissão dessa família já funciona ponta a ponta hoje.
// Por ora, só o Padrão Nacional.
func (f Familia) Suportada() bool { return f == FamiliaPadraoNacional }

// Rotulo devolve um nome curto e neutro da família (para UI/relatórios).
func (f Familia) Rotulo() string {
	switch f {
	case FamiliaPadraoNacional:
		return "Padrão Nacional"
	case FamiliaABRASFv1, FamiliaABRASFv2, FamiliaABRASFv1v2:
		return "ABRASF"
	case FamiliaProprio, FamiliaProprioABRASF:
		return "Próprio"
	default:
		return "Desconhecido"
	}
}

var (
	famOnce   sync.Once
	famByProv map[string]Familia // chave: nome do provedor em minúsculas
)

func carregarFamilias() {
	famOnce.Do(func() {
		famByProv = make(map[string]Familia)
		for _, l := range strings.Split(provedorFamiliaTSV, "\n") {
			l = strings.TrimRight(l, "\r")
			if l == "" {
				continue
			}
			c := strings.Split(l, "\t")
			if len(c) < 2 || c[0] == "" || c[1] == "" {
				continue
			}
			famByProv[strings.ToLower(c[0])] = Familia(c[1])
		}
	})
}

// FamiliaDoProvedor classifica um provedor (nome como na tabela de provedores,
// case-insensitive) na sua família. Devolve FamiliaDesconhecida se não mapeado.
func FamiliaDoProvedor(provedor string) Familia {
	carregarFamilias()
	return famByProv[strings.ToLower(strings.TrimSpace(provedor))]
}

// FamiliaPorMunicipio resolve a família de NFS-e de um município pelo código
// IBGE (via o provedor configurado). FamiliaDesconhecida = sem provedor.
func FamiliaPorMunicipio(codigo string) Familia {
	return FamiliaDoProvedor(ProvedorNFSe(codigo))
}
