package boleto

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestSemCredenciaisNaoVazaSegredo garante que o payload persistido (auditoria) do
// boleto NÃO contém os segredos de WS — eles ficam só em memória durante a operação.
func TestSemCredenciaisNaoVazaSegredo(t *testing.T) {
	p := Pedido{
		Conta: Conta{
			Banco: "237",
			WS: &ContaWS{
				ClientID:     "cli-id",
				ClientSecret: "SEGREDO-OAUTH",
				CertCRT:      "BASE64-CERT-CRT",
				CertKEY:      "BASE64-CHAVE-PRIVADA-MTLS",
			},
		},
		Titulos: []Titulo{{NumeroDocumento: "1", ValorDocumento: 10, Vencimento: "2026-01-01"}},
	}

	payload, err := json.Marshal(p.SemCredenciais())
	if err != nil {
		t.Fatal(err)
	}
	for _, proibido := range []string{
		"SEGREDO-OAUTH", "BASE64-CHAVE-PRIVADA-MTLS", "BASE64-CERT-CRT",
		"clientSecret", "certKEY", "certCRT", "cli-id",
	} {
		if bytes.Contains(payload, []byte(proibido)) {
			t.Errorf("payload persistido vazou %q: %s", proibido, payload)
		}
	}

	// O Pedido original NÃO pode ter sido mutado (a operação em memória ainda
	// precisa das credenciais).
	if p.Conta.WS == nil || p.Conta.WS.ClientSecret != "SEGREDO-OAUTH" {
		t.Fatal("SemCredenciais mutou o Pedido original (esperado: cópia)")
	}
}
