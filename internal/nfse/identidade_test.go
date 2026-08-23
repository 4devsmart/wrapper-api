package nfse

import "testing"

func TestIdentidade(t *testing.T) {
	var p DPSPedido
	p.InfDPS.Serie = "1"
	p.InfDPS.NDPS = "100"
	if serie, numero := Identidade(p); serie != "1" || numero != "100" {
		t.Fatalf("Identidade = %s/%s; queria 1/100", serie, numero)
	}
}
