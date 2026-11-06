// Package tabelas carrega tabelas de referência embutidas. A de municípios usa a
// base oficial do IBGE (5.571 municípios, com código de 7 dígitos, nome e UF), a
// mesma base que o ACBr usa para resolver o provedor de NFS-e.
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

// Municipios devolve todos os municípios do TSV embutido.
func Municipios() []Municipio {
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
