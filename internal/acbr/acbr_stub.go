//go:build !acbrlib

package acbr

import "github.com/4devsmart/wrapper-api/internal/platform/config"

// newServicos (stub) devolve serviços indisponíveis. Mantém a API rodando em
// ambientes sem os .so (dev, CI, healthcheck): o contrato JSON e a tradução
// INI funcionam; operações que exigem a lib nativa falham explicitamente.
func newServicos(_ config.ACBr) *Servicos {
	return &Servicos{
		NFSe:   indisponivel{nome: "nfse"},
		CTe:    indisponivel{nome: "cte"},
		MDFe:   indisponivel{nome: "mdfe"},
		NFe:    indisponivel{nome: "nfe"},
		Boleto: indisponivel{nome: "boleto"},
	}
}
