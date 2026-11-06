package cte

import "github.com/4devsmart/wrapper-api/internal/fiscal"

type (
	// Emissao é o resultado de uma emissão de CT-e.
	Emissao = fiscal.Emissao
	// Mensagem é um erro ou alerta com código e descrição.
	Mensagem = fiscal.Mensagem
)

// ParseEnvio interpreta a resposta INI do CTE_Enviar.
func ParseEnvio(resp string) Emissao { return fiscal.ParseEnvio(resp, "chCTe", "100", "104") }

// StatusEmissao traduz a Emissao no status devolvido ao cliente.
func StatusEmissao(e Emissao) string { return fiscal.StatusEmissao(e) }
