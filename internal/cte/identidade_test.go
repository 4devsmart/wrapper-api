package cte

import "testing"

func TestIdentidade(t *testing.T) {
	var p PedidoEmissao
	p.InfCte.Ide.Serie = 1
	p.InfCte.Ide.NCT = 100
	if mod, serie, numero := Identidade(p); mod != "57" || serie != "1" || numero != "100" {
		t.Fatalf("Identidade = %s/%s/%s; queria 57/1/100", mod, serie, numero)
	}
	// modelo explícito (CT-e OS) é preservado.
	p.InfCte.Ide.Mod = 67
	if mod, _, _ := Identidade(p); mod != "67" {
		t.Fatalf("modelo CT-e OS = %s; queria 67", mod)
	}
}

func TestIdentidadeSimp(t *testing.T) {
	var p PedidoSimp
	p.InfCte.Ide.Serie = 2
	p.InfCte.Ide.NCT = 55
	if mod, serie, numero := IdentidadeSimp(p); mod != "57" || serie != "2" || numero != "55" {
		t.Fatalf("IdentidadeSimp = %s/%s/%s; queria 57/2/55", mod, serie, numero)
	}
}
