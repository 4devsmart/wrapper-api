package mdfe

import "strconv"

// Identidade extrai a identidade fiscal (modelo, série, número) do pedido: base
// da unicidade (empresa, modelo, série, número, ambiente) que previne
// duplicidade. Modelo default 58 (MDF-e).
func Identidade(p PedidoEmissao) (modelo, serie, numero string) {
	mod := p.InfMDFe.Ide.Mod
	if mod == 0 {
		mod = 58
	}
	return strconv.Itoa(mod), strconv.Itoa(p.InfMDFe.Ide.Serie), strconv.Itoa(p.InfMDFe.Ide.NMDF)
}
