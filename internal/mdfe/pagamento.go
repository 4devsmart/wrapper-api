package mdfe

import (
	"cmp"
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoPagamentoOperacao é o corpo de POST /v1/mdfe/eventos/pagamento-operacao
// (evento 110116). Registra o pagamento da operação de transporte. Reusa os
// tipos da emissão (InfPag/Comp/InfPrazo/InfBanc).
type PedidoPagamentoOperacao struct {
	InfViagens *InfViagens `json:"infViagens,omitempty"`
	Pagamentos []InfPag    `json:"pagamentos"`
	NSeqEvento int         `json:"nSeqEvento,omitempty"`
}

// InfViagens é a seção [infViagens] (quantidade/numeração de viagens).
type InfViagens struct {
	QtdViagens int `json:"qtdViagens,omitempty"`
	NroViagem  int `json:"nroViagem,omitempty"`
}

// ToINIPagamentoOperacao monta o INI do evento 110116. Seções (todas top-level):
// [EVENTO]/[EVENTO001] + [infViagens] + [infPagNNN] (+ [CompNNNKKK],
// [infPrazoNNNKKK] quando a prazo, [infBancNNN]). Índices de 3 dígitos; os de
// Comp/infPrazo são compostos (infPag+item). Ver
// .claude/skills/acbr-especialista/referencia/mdfe-eventos.md.
func ToINIPagamentoOperacao(chave, cnpj, dhEvento string, p PedidoPagamentoOperacao) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110116", p.NSeqEvento)
	if v := p.InfViagens; v != nil {
		b.Secao("infViagens")
		b.KVIntOpt("qtdViagens", v.QtdViagens)
		b.KVIntOpt("nroViagem", v.NroViagem)
	}
	for i, pag := range p.Pagamentos {
		j := inifmt.Seq3(i + 1)
		b.Secao("infPag" + j)
		b.KVOpt("xNome", pag.XNome)
		b.KVOpt("CNPJCPF", fiscal.Primeiro(pag.CNPJ, pag.CPF))
		b.KVOpt("idEstrangeiro", pag.IdEstrangeiro)
		b.KV("vContrato", inifmt.Money(pag.VContrato))
		b.KV("indPag", strconv.Itoa(pag.IndPag))
		b.KVOpt("vAdiant", inifmt.MoneyOpt(pag.VAdiant))
		b.KVIntOpt("indAntecipaAdiant", pag.IndAntecipaAdiant)
		b.KVIntOpt("tpAntecip", pag.TpAntecip)

		for k, c := range pag.Comp {
			b.Secao("Comp" + j + inifmt.Seq3(k+1))
			b.KV("tpComp", cmp.Or(c.TpComp, "01"))
			b.KV("vComp", inifmt.Money(c.VComp))
			b.KVOpt("xComp", c.XComp)
		}
		if pag.IndPag == 1 {
			for k, par := range pag.InfPrazo {
				b.Secao("infPrazo" + j + inifmt.Seq3(k+1))
				b.KVIntOpt("nParcela", par.NParcela)
				b.KVOpt("dVenc", par.DVenc)
				b.KV("vParcela", inifmt.Money(par.VParcela))
			}
		}
		if bc := pag.InfBanc; bc.PIX != "" || bc.CNPJIPEF != "" || bc.CodBanco != "" {
			b.Secao("infBanc" + j)
			switch {
			case bc.PIX != "":
				b.KV("PIX", bc.PIX)
			case bc.CNPJIPEF != "":
				b.KV("CNPJIPEF", bc.CNPJIPEF)
			default:
				b.KVOpt("codBanco", bc.CodBanco)
				b.KVOpt("codAgencia", bc.CodAgencia)
			}
		}
	}
	return b.String()
}
