package tabelas

import (
	_ "embed"
	"sort"
	"strings"
	"sync"
)

// municipiosProvedorTSV mapeia código IBGE → provedor de NFS-e, derivado do
// ACBrNFSeXServicos.ini do FONTE OFICIAL do ACBr (SVN trunk2), a MESMA tabela
// que é compilada dentro da `.so` e usada pelo ACBrNFSeX para selecionar o
// provedor pelo município. Só entram municípios COM provedor configurado
// (4.674 de 5.571); "PadraoNacional" é o Padrão Nacional ADN/SEFIN.
//
// Regenerar (em lockstep com a `.so`, ao bumpar a revisão): `make acbr-tabelas`
// : ver scripts/gerar-tabelas-nfse.sh. Já é chamado por `make acbr-libs`.
//
//go:embed municipios_provedor.tsv
var municipiosProvedorTSV string

// MunicipioNFSe é um município cujo provedor de NFS-e está configurado na
// ACBrLib (nome/UF vêm da base canônica do IBGE).
type MunicipioNFSe struct {
	Codigo   string `json:"codigo"` // IBGE, 7 dígitos
	Nome     string `json:"nome"`
	UF       string `json:"uf"`
	Provedor string `json:"provedor"` // ex.: "PadraoNacional", "Betha", "Fiorilli"
	// Versao é o layout que ESTE município usa dentro do provedor: vazio quando
	// o provedor não versiona. Importa porque o mesmo provedor serve layouts
	// diferentes por cidade: o fintelISS atende Juiz de Fora em 2.00 e Itatiba em
	// 2.02, e só o 2.02 exige lista de itens. A superfície real do NFS-e são os
	// ~120 pares (provedor, versão), não os 106 provedores.
	Versao string `json:"versao,omitempty"`
}

// ProvedorContagem é um provedor, sua família de layout e quantos municípios o
// adotam (mais se a emissão dessa família já é suportada hoje).
type ProvedorContagem struct {
	Provedor   string  `json:"provedor"`
	Municipios int     `json:"municipios"`
	Familia    Familia `json:"familia"`
	Layout     string  `json:"layout"`    // padrao_nacional | abrasf | proprio
	Suportado  bool    `json:"suportado"` // emissão já funciona hoje
}

// FiltroMunicipioNFSe filtra a listagem de municípios por provedor.
type FiltroMunicipioNFSe struct {
	Provedor string // exato, case-insensitive; "" = todos os provedores
	UF       string // exato, case-insensitive; "" = todas as UFs
	Q        string // busca por nome (acento-insensível) ou prefixo do código
	Limit    int
	Offset   int
}

var (
	nfseOnce     sync.Once
	todosIndex   []MunicipioNFSe   // TODOS os municípios IBGE, anotados (Provedor "" = não configurado), ordenado por nome, UF
	nfseIndex    []MunicipioNFSe   // subconjunto de todosIndex COM provedor configurado
	nfseByCod    map[string]string // código IBGE → provedor
	nfseVerByCod map[string]string // código IBGE -> versão do layout
)

func carregarNFSe() {
	nfseOnce.Do(func() {
		nfseByCod = make(map[string]string)
		nfseVerByCod = make(map[string]string)
		for _, l := range strings.Split(municipiosProvedorTSV, "\n") {
			l = strings.TrimRight(l, "\r")
			if l == "" {
				continue
			}
			c := strings.Split(l, "\t")
			if len(c) < 2 || c[0] == "" || c[1] == "" {
				continue
			}
			nfseByCod[c[0]] = c[1]
			if len(c) > 2 {
				nfseVerByCod[c[0]] = c[2]
			}
		}
		todos := MunicipiosSeed()
		todosIndex = make([]MunicipioNFSe, 0, len(todos))
		for _, m := range todos {
			todosIndex = append(todosIndex, MunicipioNFSe{Codigo: m.Codigo, Nome: m.Nome, UF: m.UF,
				Provedor: nfseByCod[m.Codigo], Versao: nfseVerByCod[m.Codigo]})
		}
		sort.Slice(todosIndex, func(i, j int) bool {
			if todosIndex[i].Nome != todosIndex[j].Nome {
				return todosIndex[i].Nome < todosIndex[j].Nome
			}
			return todosIndex[i].UF < todosIndex[j].UF
		})
		nfseIndex = make([]MunicipioNFSe, 0, len(nfseByCod))
		for _, m := range todosIndex {
			if m.Provedor != "" {
				nfseIndex = append(nfseIndex, m)
			}
		}
	})
}

// BuscarMunicipios pesquisa na base COMPLETA do IBGE (inclusive municípios sem
// provedor) por nome (acento-insensível) ou prefixo do código, anotando o
// provedor de NFS-e de cada um (Provedor == "" → não configurado na ACBrLib).
// Alimenta o widget público de consulta. Limita a `limit` (default 8, máx 20);
// query vazia devolve lista vazia.
func BuscarMunicipios(q string, limit int) []MunicipioNFSe {
	carregarNFSe()
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	termo := Normalizar(q)
	digitos := soDigitos(q)
	if termo == "" && digitos == "" {
		return []MunicipioNFSe{}
	}
	out := make([]MunicipioNFSe, 0, limit)
	for _, m := range todosIndex {
		if (termo != "" && strings.Contains(Normalizar(m.Nome), termo)) ||
			(digitos != "" && strings.HasPrefix(m.Codigo, digitos)) {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// ProvedorNFSe devolve o provedor de NFS-e configurado para o código IBGE, ou
// "" se o município não tem provedor configurado na ACBrLib.
func ProvedorNFSe(codigo string) string {
	carregarNFSe()
	return nfseByCod[codigo]
}

// ListarMunicipiosNFSe filtra e pagina os municípios com provedor configurado.
// Devolve a página e o total de itens que casam com o filtro (antes da página).
func ListarMunicipiosNFSe(f FiltroMunicipioNFSe) ([]MunicipioNFSe, int) {
	carregarNFSe()
	prov := strings.ToLower(strings.TrimSpace(f.Provedor))
	uf := strings.ToUpper(strings.TrimSpace(f.UF))
	termo := Normalizar(f.Q)
	digitos := soDigitos(f.Q)

	casa := func(m MunicipioNFSe) bool {
		if prov != "" && strings.ToLower(m.Provedor) != prov {
			return false
		}
		if uf != "" && m.UF != uf {
			return false
		}
		if termo != "" && !strings.Contains(Normalizar(m.Nome), termo) &&
			!(digitos != "" && strings.HasPrefix(m.Codigo, digitos)) {
			return false
		}
		return true
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	out := make([]MunicipioNFSe, 0, limit)
	total := 0
	for _, m := range nfseIndex {
		if !casa(m) {
			continue
		}
		if total >= offset && len(out) < limit {
			out = append(out, m)
		}
		total++
	}
	return out, total
}

// ProvedoresNFSe lista os provedores distintos e a contagem de municípios de
// cada um, ordenado por contagem (desc) e depois por nome.
func ProvedoresNFSe() []ProvedorContagem {
	carregarNFSe()
	cont := make(map[string]int)
	for _, m := range nfseIndex {
		cont[m.Provedor]++
	}
	out := make([]ProvedorContagem, 0, len(cont))
	for p, n := range cont {
		fam := FamiliaDoProvedor(p)
		out = append(out, ProvedorContagem{
			Provedor: p, Municipios: n,
			Familia: fam, Layout: fam.LayoutBase(), Suportado: fam.Suportada(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Municipios != out[j].Municipios {
			return out[i].Municipios > out[j].Municipios
		}
		return out[i].Provedor < out[j].Provedor
	})
	return out
}

// soDigitos remove tudo que não é dígito de um código IBGE. Morava em
// postgres.go, que não veio no porte (não há banco): é helper puro.
func soDigitos(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	return string(out)
}
