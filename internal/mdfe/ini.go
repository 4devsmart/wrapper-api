package mdfe

import (
	"cmp"
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// ToINI traduz o pedido (JSON ACBr.API) para o INI do MDF-e consumido pela
// ACBrLibMDFe (MDFE_CarregarINI). Seções/chaves seguem o modelo oficial
// (acbr.sourceforge.io/ACBrLib/ModeloMDFeINI.html).
//
// Cobertura do MODAL RODOVIÁRIO, que é o escopo do serviço: infANTT
// (CIOT/valePed/contratante/infPag), veicTracao/veicReboque/condutor/lacRodo,
// descarga (infNFe/infCTe/infMDFeTransp com peri + unidCarga/unidTransp
// aninhados), seg, prodPred/infLotacao, infAdic, infRespTec.
//
// Aéreo, aquaviário e ferroviário estão fora do contrato, e o handler recusa
// antes de chegar aqui (ver modalSuportado, http.go).
func ToINI(p PedidoEmissao) string {
	inf := p.InfMDFe
	id := inf.Ide
	var b iniBuilder
	b.EmUF(inf.Ide.CUF)

	b.Secao("infMDFe")
	b.KV("versao", cmp.Or(inf.Versao, "3.00"))

	b.Secao("ide")
	b.KVIntOpt("cUF", id.CUF)
	b.KV("tpAmb", fiscal.TpAmb(p.Ambiente))
	b.KV("tpEmit", strconv.Itoa(cmp.Or(id.TpEmit, 1)))
	b.KVIntOpt("tpTransp", id.TpTransp)
	b.KV("mod", strconv.Itoa(cmp.Or(id.Mod, 58)))
	b.KV("serie", strconv.Itoa(id.Serie))
	b.KV("nMDF", strconv.Itoa(id.NMDF))
	b.KVOpt("cMDF", id.CMDF)
	b.KV("modal", strconv.Itoa(cmp.Or(id.Modal, 1))) // 1 = rodoviário
	b.KV("dhemi", b.DataHora(id.DhEmi))
	b.KV("tpEmis", strconv.Itoa(cmp.Or(id.TpEmis, 1)))
	b.KV("procEmi", cmp.Or(id.ProcEmi, "0"))
	b.KV("verProc", cmp.Or(id.VerProc, versao.Emissor()))
	b.KV("UFIni", id.UFIni)
	b.KV("UFFim", id.UFFim)
	b.KVIntOpt("indCanalVerde", id.IndCanalVerde)
	b.KVIntOpt("indCarregaPosterior", id.IndCarregaPosterior)
	// dhIniViagem pertence a [ide]: o leitor da lib faz ReadString(<ide>,
	// 'dhIniViagem'). Era escrito dentro de [CARR001] e simplesmente ignorado:
	// o MDF-e saía sem data/hora de início da viagem. Achado pelo lockstep.
	if id.DhIniViagem != "" {
		b.KV("dhIniViagem", b.DataHora(id.DhIniViagem))
	}

	for i, perc := range id.InfPercurso {
		b.Secao("perc" + inifmt.Seq3(i+1))
		b.KV("UFPer", perc.UFPer)
	}
	for i, c := range id.InfMunCarrega {
		b.Secao("CARR" + inifmt.Seq3(i+1))
		b.KV("cMunCarrega", c.CMunCarrega)
		b.KV("xMunCarrega", c.XMunCarrega)
	}

	b.Secao("emit")
	b.KVOpt("CNPJCPF", fiscal.Primeiro(inf.Emit.CNPJ, inf.Emit.CPF))
	b.KVOpt("IE", inf.Emit.IE)
	b.KVOpt("xNome", inf.Emit.XNome)
	b.KVOpt("xFant", inf.Emit.XFant)
	if a := inf.Emit.EnderEmit; a != nil {
		b.KVOpt("xLgr", a.XLgr)
		b.KVOpt("nro", a.Nro)
		b.KVOpt("xCpl", a.XCpl)
		b.KVOpt("xBairro", a.XBairro)
		b.KVOpt("cMun", a.CMun)
		b.KVOpt("xMun", a.XMun)
		b.KVOpt("CEP", a.CEP)
		b.KVOpt("UF", a.UF)
		b.KVOpt("fone", a.Fone)
		b.KVOpt("email", a.Email)
	}

	if rodo := inf.InfModal.Rodo; rodo != nil {
		b.Secao("Rodo")
		b.KVOpt("codAgPorto", rodo.CodAgPorto)
		if antt := rodo.InfANTT; antt != nil {
			b.Secao("infANTT")
			b.KVOpt("RNTRC", antt.RNTRC)
			for i, c := range antt.InfCIOT {
				b.Secao("infCIOT" + inifmt.Seq3(i+1))
				b.KV("CIOT", c.CIOT)
				b.KVOpt("CNPJCPF", fiscal.Primeiro(c.CNPJ, c.CPF))
			}
			if vp := antt.ValePed; vp != nil {
				b.Secao("valePed")
				b.KVOpt("CategCombVeic", vp.CategCombVeic)
				for i, d := range vp.Disp {
					b.Secao("disp" + inifmt.Seq3(i+1))
					b.KVOpt("CNPJForn", d.CNPJForn)
					b.KVOpt("CNPJPg", fiscal.Primeiro(d.CNPJPg, d.CPFPg))
					b.KVOpt("nCompra", d.NCompra)
					b.KV("vValePed", inifmt.Money(d.VValePed))
					b.KVOpt("tpValePed", d.TpValePed)
				}
			}
			for i, ct := range antt.InfContratante {
				b.Secao("infContratante" + inifmt.Seq3(i+1))
				b.KVOpt("CNPJCPF", fiscal.Primeiro(ct.CNPJ, ct.CPF))
				b.KVOpt("idEstrangeiro", ct.IdEstrangeiro)
				b.KVOpt("xNome", ct.XNome)
				// infContrato não tem seção própria: a lib lê as duas chaves
				// dentro de [infContratantennn].
				if k := ct.InfContrato; k != nil {
					b.KVOpt("NroContrato", k.NroContrato)
					b.KVOpt("vContratoGlobal", inifmt.MoneyOpt(k.VContratoGlobal))
				}
			}
			for pi, pg := range antt.InfPag {
				ps := inifmt.Seq3(pi + 1)
				b.Secao("infPag" + ps)
				b.KVOpt("CNPJCPF", fiscal.Primeiro(pg.CNPJ, pg.CPF))
				b.KVOpt("idEstrangeiro", pg.IdEstrangeiro)
				b.KVOpt("xNome", pg.XNome)
				b.KV("vContrato", inifmt.Money(pg.VContrato))
				b.KV("indPag", strconv.Itoa(pg.IndPag))
				b.KVOpt("vAdiant", inifmt.MoneyOpt(pg.VAdiant))
				b.KVIntOpt("indAltoDesemp", pg.IndAltoDesemp)
				b.KVIntOpt("indAntecipaAdiant", pg.IndAntecipaAdiant)
				b.KVIntOpt("tpAntecip", pg.TpAntecip)
				for ci, c := range pg.Comp {
					b.Secao("Comp" + ps + inifmt.Seq3(ci+1))
					b.KV("tpComp", c.TpComp)
					b.KV("vComp", inifmt.Money(c.VComp))
					b.KVOpt("xComp", c.XComp)
				}
				for zi, z := range pg.InfPrazo {
					b.Secao("infPrazo" + ps + inifmt.Seq3(zi+1))
					b.KV("nParcela", strconv.Itoa(z.NParcela))
					b.KVOpt("dVenc", b.Data(z.DVenc))
					b.KV("vParcela", inifmt.Money(z.VParcela))
				}
				if ib := pg.InfBanc; ib.CodBanco != "" || ib.CNPJIPEF != "" || ib.PIX != "" {
					b.Secao("infBanc" + ps)
					b.KVOpt("codBanco", ib.CodBanco)
					b.KVOpt("codAgencia", ib.CodAgencia)
					b.KVOpt("CNPJIPEF", ib.CNPJIPEF)
					b.KVOpt("PIX", ib.PIX)
				}
			}
		}
		v := rodo.VeicTracao
		b.Secao("veicTracao")
		b.KVOpt("cInt", v.CInt)
		b.KV("placa", v.Placa)
		b.KVOpt("UF", v.UF)
		b.KVOpt("RENAVAM", v.RENAVAM)
		b.KV("tara", strconv.Itoa(v.Tara))
		b.KVIntOpt("capKG", v.CapKG)
		b.KVIntOpt("capM3", v.CapM3)
		b.KV("tpRod", v.TpRod)
		b.KV("tpCar", v.TpCar)
		if pr := v.Prop; pr != nil {
			b.KVOpt("CNPJCPF", fiscal.Primeiro(pr.CNPJ, pr.CPF))
			b.KVOpt("RNTRC", pr.RNTRC)
			b.KVOpt("xNome", pr.XNome)
			b.KVOpt("IE", pr.IE)
			b.KVOpt("UFProp", pr.UF)
			b.KVIntOpt("tpProp", pr.TpProp)
		}
		for i, c := range v.Condutor {
			b.Secao("moto" + inifmt.Seq3(i+1))
			b.KV("xNome", c.XNome)
			b.KV("CPF", c.CPF)
		}
		for i, rb := range rodo.VeicReboque {
			b.Secao("reboque" + inifmt.Seq2(i+1)) // [reboqueNN]: 2 dígitos
			b.KVOpt("cInt", rb.CInt)
			b.KV("placa", rb.Placa)
			b.KVOpt("RENAVAM", rb.RENAVAM)
			b.KV("tara", strconv.Itoa(rb.Tara))
			b.KVIntOpt("capKG", rb.CapKG)
			b.KVIntOpt("capM3", rb.CapM3)
			b.KV("tpCar", rb.TpCar)
			b.KVOpt("UF", rb.UF)
			if pr := rb.Prop; pr != nil {
				b.KVOpt("CNPJCPF", fiscal.Primeiro(pr.CNPJ, pr.CPF))
				b.KVOpt("RNTRC", pr.RNTRC)
				b.KVOpt("xNome", pr.XNome)
				b.KVOpt("IE", pr.IE)
				b.KVOpt("UFProp", pr.UF)
				b.KVIntOpt("tpProp", pr.TpProp)
			}
		}
		for i, l := range rodo.LacRodo {
			b.Secao("lacRodo" + inifmt.Seq3(i+1))
			b.KV("nLacre", l.NLacre)
		}
	}

	for di, desc := range inf.InfDoc.InfMunDescarga {
		b.Secao("DESC" + inifmt.Seq3(di+1))
		b.KV("cMunDescarga", desc.CMunDescarga)
		b.KV("xMunDescarga", desc.XMunDescarga)
		for ni, nfe := range desc.InfNFe {
			dn := inifmt.Seq3(di+1) + inifmt.Seq3(ni+1)
			b.Secao("infNFe" + dn)
			b.KV("chNFe", nfe.ChNFe)
			b.KVOpt("SegCodBarra", nfe.SegCodBarra)
			b.KVIntOpt("indReentrega", nfe.IndReentrega)
			for pi, p := range nfe.Peri {
				b.peri(dn+inifmt.Seq3(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, nfe.InfUnidTransp)
		}
		for ci, c := range desc.InfCTe {
			dn := inifmt.Seq3(di+1) + inifmt.Seq3(ci+1)
			b.Secao("infCTe" + dn)
			b.KV("chCTe", c.ChCTe)
			b.KVOpt("SegCodBarra", c.SegCodBarra)
			b.KVIntOpt("indReentrega", c.IndReentrega)
			b.KVIntOpt("indPrestacaoParcial", c.IndPrestacaoParcial)
			if ep := c.InfEntregaParcial; ep != nil {
				b.Secao("infEntregaParcial" + dn)
				b.KV("qtdTotal", inifmt.Money(ep.QtdTotal))
				b.KV("qtdParcial", inifmt.Money(ep.QtdParcial))
			}
			for ni, nfp := range c.InfNFePrestParcial {
				b.Secao("infNFePrestParcial" + dn + inifmt.Seq3(ni+1))
				b.KV("chNFe", nfp.ChNFe)
			}
			for pi, p := range c.Peri {
				b.peri(dn+inifmt.Seq3(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, c.InfUnidTransp)
		}
		for mi, m := range desc.InfMDFeTransp {
			dn := inifmt.Seq3(di+1) + inifmt.Seq3(mi+1)
			b.Secao("infMDFeTransp" + dn)
			b.KV("chMDFe", m.ChMDFe)
			b.KVIntOpt("indReentrega", m.IndReentrega)
			for pi, p := range m.Peri {
				b.peri(dn+inifmt.Seq3(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, m.InfUnidTransp)
		}
	}

	if pp := inf.ProdPred; pp != nil {
		b.Secao("prodPred")
		b.KVOpt("tpCarga", pp.TpCarga)
		b.KVOpt("xProd", pp.XProd)
		b.KVOpt("cEAN", pp.CEAN)
		b.KVOpt("NCM", pp.NCM)
		if lot := pp.InfLotacao; lot != nil {
			c := lot.InfLocalCarrega
			b.Secao("infLocalCarrega")
			b.KVOpt("CEP", c.CEP)
			b.KVOpt("latitude", c.Latitude)
			b.KVOpt("longitude", c.Longitude)
			d := lot.InfLocalDescarrega
			b.Secao("infLocalDescarrega")
			b.KVOpt("CEP", d.CEP)
			b.KVOpt("latitude", d.Latitude)
			b.KVOpt("longitude", d.Longitude)
		}
	}

	t := inf.Tot
	b.Secao("tot")
	b.KVIntOpt("qCTe", t.QCTe)
	b.KVIntOpt("qNFe", t.QNFe)
	b.KVIntOpt("qMDFe", t.QMDFe)
	b.KV("vCarga", inifmt.Money(t.VCarga))
	b.KV("cUnid", cmp.Or(t.CUnid, "01"))
	b.KV("qCarga", inifmt.Money(t.QCarga))

	for i, l := range inf.Lacres {
		b.Secao("lacres" + inifmt.Seq3(i+1))
		b.KV("nLacre", l.NLacre)
	}

	if ia := inf.InfAdic; ia != nil {
		b.Secao("infAdic")
		b.KVOpt("infAdFisco", ia.InfAdFisco)
		b.KVOpt("infCpl", ia.InfCpl)
	}

	for i, s := range inf.Seg {
		b.Secao("seg" + inifmt.Seq3(i+1))
		b.KV("respSeg", strconv.Itoa(s.InfResp.RespSeg))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(s.InfResp.CNPJ, s.InfResp.CPF))
		if is := s.InfSeg; is != nil {
			b.KVOpt("xSeg", is.XSeg)
			b.KVOpt("CNPJ", is.CNPJ)
		}
		b.KVOpt("nApol", s.NApol)
		for j, av := range s.NAver {
			b.Secao("aver" + inifmt.Seq3(i+1) + inifmt.Seq3(j+1))
			b.KV("nAver", av)
		}
	}

	// Indexação de DUAS casas, não das três que o resto do arquivo usa:
	// verificado contra a biblioteca, [autXML001] é ignorado em silêncio e
	// [autXML01] é lido.
	for i, a := range inf.AutXML {
		b.Secao("autXML" + inifmt.Seq2(i+1))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(a.CNPJ, a.CPF))
	}

	if rt := inf.InfRespTec; rt != nil {
		b.Secao("infRespTec")
		b.KV("CNPJ", rt.CNPJ)
		b.KV("xContato", rt.XContato)
		b.KV("email", rt.Email)
		b.KV("fone", rt.Fone)
		b.KVIntOpt("idCSRT", rt.IdCSRT)
		b.KVOpt("hashCSRT", rt.HashCSRT)
	}
	return b.String()
}

// --- builder de INI (local ao pacote mdfe) ----------------------------------

// iniBuilder acumula o INI. Carrega o fuso do documento porque data-hora só faz
// sentido com ele: o cliente informa um instante (RFC 3339) e o documento fiscal
// precisa desse instante no relógio do emitente.
// iniBuilder é o construtor compartilhado (internal/platform/inifmt) mais os
// métodos de DOMÍNIO deste documento, logo abaixo. O núcleo (seção, par
// chave=valor, datas no fuso do emitente) vive num lugar só: era o mesmo código
// nos quatro documentos, e um ajuste na sanitização precisava ser feito quatro
// vezes.
type iniBuilder struct{ inifmt.Builder }

// peri emite [periIDX] (produto perigoso) do documento, com o índice já pronto.
func (b *iniBuilder) peri(idx, nONU, xNomeAE, xClaRisco, grEmb, qTotProd, qVolTipo string) {
	b.Secao("peri" + idx)
	b.KV("nONU", nONU)
	b.KVOpt("xNomeAE", xNomeAE)
	b.KVOpt("xClaRisco", xClaRisco)
	b.KVOpt("grEmb", grEmb)
	b.KV("qTotProd", qTotProd)
	b.KVOpt("qVolTipo", qVolTipo)
}

// unidTransp emite infUnidTransp/infUnidCarga (+lacres) de um documento da
// descarga, com o prefixo {descarga}{doc}.
func (b *iniBuilder) unidTransp(prefix string, transps []UnidadeTransp) {
	for ti, tp := range transps {
		tIdx := prefix + inifmt.Seq3(ti+1)
		b.Secao("infUnidTransp" + tIdx)
		b.KV("tpUnidTransp", strconv.Itoa(tp.TpUnidTransp))
		b.KV("idUnidTransp", tp.IdUnidTransp)
		b.KVOpt("qtdRat", inifmt.MoneyOpt(tp.QtdRat))
		for li, l := range tp.LacUnidTransp {
			b.Secao("lacUnidTransp" + tIdx + inifmt.Seq3(li+1))
			b.KV("nLacre", l.NLacre)
		}
		for ci, c := range tp.InfUnidCarga {
			cIdx := tIdx + inifmt.Seq3(ci+1)
			b.Secao("infUnidCarga" + cIdx)
			b.KV("tpUnidCarga", strconv.Itoa(c.TpUnidCarga))
			b.KV("idUnidCarga", c.IdUnidCarga)
			b.KVOpt("qtdRat", inifmt.MoneyOpt(c.QtdRat))
			for li, l := range c.LacUnidCarga {
				b.Secao("lacUnidCarga" + cIdx + inifmt.Seq3(li+1))
				b.KV("nLacre", l.NLacre)
			}
		}
	}
}

// --- helpers ----------------------------------------------------------------
