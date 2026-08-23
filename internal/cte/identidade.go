package cte

import "strconv"

// Identidade extrai a identidade fiscal (modelo, série, número) do pedido — base
// da unicidade (empresa, modelo, série, número, ambiente) que previne
// duplicidade. Modelo default 57 (CT-e); 67 = CT-e OS.
func Identidade(p PedidoEmissao) (modelo, serie, numero string) {
	return identidadeIde(p.InfCte.Ide)
}

// IdentidadeSimp é o equivalente para o CT-e Simplificado (mesmo grupo Ide).
func IdentidadeSimp(p PedidoSimp) (modelo, serie, numero string) {
	return identidadeIde(p.InfCte.Ide)
}

func identidadeIde(ide Ide) (modelo, serie, numero string) {
	mod := ide.Mod
	if mod == 0 {
		mod = 57
	}
	return strconv.Itoa(mod), strconv.Itoa(ide.Serie), strconv.Itoa(ide.NCT)
}
