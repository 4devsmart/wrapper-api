package mdfe

import "github.com/4devsmart/wrapper-api/internal/fiscal"

type (
	// Emissao é o resultado de uma emissão de MDF-e.
	Emissao = fiscal.Emissao
	// Mensagem é um erro ou alerta com código e descrição.
	Mensagem = fiscal.Mensagem
)

// ParseEnvio interpreta a resposta INI do MDFE_Enviar.
func ParseEnvio(resp string) Emissao { return fiscal.ParseEnvio(resp, "chMDFe", "100", "132") }

// StatusEmissao traduz a Emissao no status devolvido ao cliente.
func StatusEmissao(e Emissao) string { return fiscal.StatusEmissao(e) }
