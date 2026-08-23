package nfse

// Identidade extrai a identidade fiscal (série, número/nDPS) do pedido — base da
// unicidade (emitente, série, nDPS, ambiente) que previne duplicidade. NFS-e PN
// não tem "modelo" (fica vazio na chave de unicidade).
func Identidade(p DPSPedido) (serie, numero string) {
	return p.InfDPS.Serie, p.InfDPS.NDPS
}
