// Package tabelas carrega tabelas de referência embutidas. A de municípios usa a
// base oficial do IBGE (5.571 municípios, com código de 7 dígitos, nome e UF), a
// mesma base que o ACBr usa para resolver o provedor de NFS-e. O TSV embutido é
// a fonte do seed da tabela `municipios` no PostgreSQL (ver MunicipioRepo).
package tabelas

import (
	_ "embed"
	"strings"
)

//go:embed municipios.tsv
var municipiosTSV string

// Municipio é uma linha da tabela do IBGE.
type Municipio struct {
	Codigo string `json:"codigo"` // IBGE, 7 dígitos
	Nome   string `json:"nome"`
	UF     string `json:"uf"`
}

// MunicipiosSeed devolve todos os municípios parseados do TSV embutido (usado
// para popular o banco no primeiro start).
func MunicipiosSeed() []Municipio {
	linhas := strings.Split(municipiosTSV, "\n")
	out := make([]Municipio, 0, len(linhas))
	for _, l := range linhas {
		l = strings.TrimRight(l, "\r")
		if l == "" {
			continue
		}
		c := strings.Split(l, "\t")
		if len(c) < 3 {
			continue
		}
		out = append(out, Municipio{Codigo: c[0], Nome: c[1], UF: c[2]})
	}
	return out
}

// Normalizar baixa a caixa e remove acentos do PT-BR (para a coluna de busca
// acento-insensível). Exportada para o repo computar `nome_busca` no seed e na
// consulta.
func Normalizar(s string) string {
	return acentos.Replace(strings.ToLower(strings.TrimSpace(s)))
}

var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "î", "i", "ì", "i", "ï", "i",
	"ó", "o", "ô", "o", "õ", "o", "ò", "o", "ö", "o",
	"ú", "u", "û", "u", "ù", "u", "ü", "u",
	"ç", "c", "ñ", "n", "'", "", "`", "",
)
