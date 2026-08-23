package mdfe

import "strconv"

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
		b.section("infViagens")
		b.kvIntOpt("qtdViagens", v.QtdViagens)
		b.kvIntOpt("nroViagem", v.NroViagem)
	}
	for i, pag := range p.Pagamentos {
		j := seq(i + 1)
		b.section("infPag" + j)
		b.kvOpt("xNome", pag.XNome)
		b.kvOpt("CNPJCPF", firstNonEmpty(pag.CNPJ, pag.CPF))
		b.kvOpt("idEstrangeiro", pag.IdEstrangeiro)
		b.kv("vContrato", money(pag.VContrato))
		b.kv("indPag", strconv.Itoa(pag.IndPag))
		b.kvOpt("vAdiant", moneyOpt(pag.VAdiant))
		b.kvIntOpt("indAntecipaAdiant", pag.IndAntecipaAdiant)
		b.kvIntOpt("tpAntecip", pag.TpAntecip)

		for k, c := range pag.Comp {
			b.section("Comp" + j + seq(k+1))
			b.kv("tpComp", defaultStr(c.TpComp, "01"))
			b.kv("vComp", money(c.VComp))
			b.kvOpt("xComp", c.XComp)
		}
		if pag.IndPag == 1 {
			for k, par := range pag.InfPrazo {
				b.section("infPrazo" + j + seq(k+1))
				b.kvIntOpt("nParcela", par.NParcela)
				b.kvOpt("dVenc", par.DVenc)
				b.kv("vParcela", money(par.VParcela))
			}
		}
		if bc := pag.InfBanc; bc.PIX != "" || bc.CNPJIPEF != "" || bc.CodBanco != "" {
			b.section("infBanc" + j)
			switch {
			case bc.PIX != "":
				b.kv("PIX", bc.PIX)
			case bc.CNPJIPEF != "":
				b.kv("CNPJIPEF", bc.CNPJIPEF)
			default:
				b.kvOpt("codBanco", bc.CodBanco)
				b.kvOpt("codAgencia", bc.CodAgencia)
			}
		}
	}
	return b.String()
}
