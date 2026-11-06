package tabelas

import (
	_ "embed"
	"strings"
)

// notasTecnicasTSV são as notas técnicas ligadas no ACBrNFSeXServicos.ini da
// revisão pinada: provedor, ambiente e versão. Gerado por `make acbr-tabelas`.
//
//go:embed notas_tecnicas.tsv
var notasTecnicasTSV string

// NotaTecnicaDaLib é uma NT que a lib aplica por conta própria num ambiente.
type NotaTecnicaDaLib struct {
	Provedor string
	Ambiente string
	Versao   string
}

// NotasTecnicasDaLib lista o que a `.so` aplica. Quem decide é o .ini compilado
// nela, sem data: aqui só se enxerga.
func NotasTecnicasDaLib() []NotaTecnicaDaLib {
	var notas []NotaTecnicaDaLib
	for linha := range strings.Lines(notasTecnicasTSV) {
		c := strings.Split(strings.TrimSpace(linha), "\t")
		if len(c) == 3 {
			notas = append(notas, NotaTecnicaDaLib{Provedor: c[0], Ambiente: c[1], Versao: c[2]})
		}
	}
	return notas
}
