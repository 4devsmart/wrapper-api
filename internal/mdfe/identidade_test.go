package mdfe

import "testing"

func TestIdentidade(t *testing.T) {
	var p PedidoEmissao
	p.InfMDFe.Ide.Serie = 1
	p.InfMDFe.Ide.NMDF = 100
	if mod, serie, numero := Identidade(p); mod != "58" || serie != "1" || numero != "100" {
		t.Fatalf("Identidade = %s/%s/%s; queria 58/1/100", mod, serie, numero)
	}
}
