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
// antes de chegar aqui (ver fiscal.ModalSuportado).
//
// Esta função é um ÍNDICE: cada grupo do documento é um método logo abaixo, na
// ordem em que sai no arquivo. Era um bloco de 300 linhas com cinco níveis de
// aninhamento, onde achar o que escreve uma chave custava rolagem, e mudar uma
// delas obrigava a ler o resto para saber o que mais estava naquele escopo.
func ToINI(p PedidoEmissao) string {
	inf := p.InfMDFe
	var b iniBuilder
	b.EmUF(inf.Ide.CUF)

	b.Secao("infMDFe")
	b.KV("versao", cmp.Or(inf.Versao, "3.00"))

	b.identificacao(inf.Ide, p.Ambiente)
	b.emitente(inf.Emit)
	b.modalRodo(inf.InfModal.Rodo)
	b.descargas(inf.InfDoc.InfMunDescarga)
	b.produtoPredominante(inf.ProdPred)
	b.totais(inf.Tot)
	b.lacres(inf.Lacres)
	b.adicionais(inf.InfAdic)
	b.seguros(inf.Seg)
	b.autorizados(inf.AutXML)
	b.respTec(inf.InfRespTec)
	return b.String()
}

// identificacao emite [ide] + os percursos e municípios de carregamento.
func (b *iniBuilder) identificacao(id Ide, ambiente string) {
	b.Secao("ide")
	b.KVIntOpt("cUF", id.CUF)
	// tpAmb sai SEMPRE do ambiente do pedido, o mesmo que configura a sessão
	// nativa. O tpAmb explícito de infMDFe.ide é conferido no handler e
	// recusado quando contradiz: eleger um vencedor aqui seria a divergência
	// que ninguém vê, com um documento de teste indo para produção.
	b.KV("tpAmb", fiscal.TpAmb(ambiente))
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
}

// emitente emite [emit], com o endereço inline (ele não tem seção própria).
func (b *iniBuilder) emitente(e Emit) {
	b.Secao("emit")
	b.KVOpt("CNPJCPF", fiscal.Primeiro(e.CNPJ, e.CPF))
	b.KVOpt("IE", e.IE)
	b.KVOpt("xNome", e.XNome)
	b.KVOpt("xFant", e.XFant)
	if a := e.EnderEmit; a != nil {
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
}

// --- modal rodoviário -------------------------------------------------------

// modalRodo emite [Rodo] e tudo que pende dele: a ANTT, os veículos e os
// lacres. É o único modal do contrato (ver Escopo, no README).
func (b *iniBuilder) modalRodo(rodo *Rodo) {
	if rodo == nil {
		return
	}
	b.Secao("Rodo")
	b.KVOpt("codAgPorto", rodo.CodAgPorto)
	b.infANTT(rodo.InfANTT)

	b.veicTracao(rodo.VeicTracao)
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
		b.proprietario(rb.Prop)
	}
	for i, l := range rodo.LacRodo {
		b.Secao("lacRodo" + inifmt.Seq3(i+1))
		b.KV("nLacre", l.NLacre)
	}
}

// infANTT emite [infANTT] e os grupos da ANTT: CIOT, vale-pedágio,
// contratantes e pagamentos.
func (b *iniBuilder) infANTT(antt *InfANTT) {
	if antt == nil {
		return
	}
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
			// A lib lê UM pagador, em CNPJPg: o CPF entra ali quando é o que
			// veio.
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
		// infContrato não tem seção própria: a lib lê as duas chaves dentro de
		// [infContratantennn].
		if k := ct.InfContrato; k != nil {
			b.KVOpt("NroContrato", k.NroContrato)
			b.KVOpt("vContratoGlobal", inifmt.MoneyOpt(k.VContratoGlobal))
		}
	}
	for i, pg := range antt.InfPag {
		b.pagamento(inifmt.Seq3(i+1), pg)
	}
}

// pagamento emite [infPagNNN] e os grupos que pendem dele: componentes,
// parcelas e dados bancários. O índice do pagamento entra no nome das seções
// filhas, que é como a lib as associa ao pagamento certo.
func (b *iniBuilder) pagamento(idx string, pg InfPag) {
	b.Secao("infPag" + idx)
	b.KVOpt("CNPJCPF", fiscal.Primeiro(pg.CNPJ, pg.CPF))
	b.KVOpt("idEstrangeiro", pg.IdEstrangeiro)
	b.KVOpt("xNome", pg.XNome)
	b.KV("vContrato", inifmt.Money(pg.VContrato))
	b.KV("indPag", strconv.Itoa(pg.IndPag))
	b.KVOpt("vAdiant", inifmt.MoneyOpt(pg.VAdiant))
	b.KVIntOpt("indAltoDesemp", pg.IndAltoDesemp)
	b.KVIntOpt("indAntecipaAdiant", pg.IndAntecipaAdiant)
	b.KVIntOpt("tpAntecip", pg.TpAntecip)
	for i, c := range pg.Comp {
		b.Secao("Comp" + idx + inifmt.Seq3(i+1))
		b.KV("tpComp", c.TpComp)
		b.KV("vComp", inifmt.Money(c.VComp))
		b.KVOpt("xComp", c.XComp)
	}
	for i, z := range pg.InfPrazo {
		b.Secao("infPrazo" + idx + inifmt.Seq3(i+1))
		b.KV("nParcela", strconv.Itoa(z.NParcela))
		b.KVOpt("dVenc", b.Data(z.DVenc))
		b.KV("vParcela", inifmt.Money(z.VParcela))
	}
	// A seção só sai com algum dado bancário: vazia, ela faria a lib gravar um
	// grupo de pagamento sem meio de pagamento nenhum.
	if ib := pg.InfBanc; ib.CodBanco != "" || ib.CNPJIPEF != "" || ib.PIX != "" {
		b.Secao("infBanc" + idx)
		b.KVOpt("codBanco", ib.CodBanco)
		b.KVOpt("codAgencia", ib.CodAgencia)
		b.KVOpt("CNPJIPEF", ib.CNPJIPEF)
		b.KVOpt("PIX", ib.PIX)
	}
}

// veicTracao emite [veicTracao] + [motoNNN] (condutores).
func (b *iniBuilder) veicTracao(v VeicTracao) {
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
	b.proprietario(v.Prop)
	for i, c := range v.Condutor {
		b.Secao("moto" + inifmt.Seq3(i+1))
		b.KV("xNome", c.XNome)
		b.KV("CPF", c.CPF)
	}
}

// proprietario escreve o proprietário do veículo na seção do CHAMADOR: no
// layout ele não tem seção própria, e vale tanto para a tração quanto para o
// reboque. A UF dele vai em UFProp, não em UF, que é a do veículo.
func (b *iniBuilder) proprietario(pr *Prop) {
	if pr == nil {
		return
	}
	b.KVOpt("CNPJCPF", fiscal.Primeiro(pr.CNPJ, pr.CPF))
	b.KVOpt("RNTRC", pr.RNTRC)
	b.KVOpt("xNome", pr.XNome)
	b.KVOpt("IE", pr.IE)
	b.KVOpt("UFProp", pr.UF)
	b.KVIntOpt("tpProp", pr.TpProp)
}

// --- documentos transportados -----------------------------------------------

// descargas emite [DESCNNN] e os documentos de cada município de descarga.
//
// O índice das seções filhas é {descarga}{documento}: é ele que diz à lib em
// qual município aquele documento é descarregado.
func (b *iniBuilder) descargas(descargas []InfMunDescarga) {
	for di, desc := range descargas {
		dIdx := inifmt.Seq3(di + 1)
		b.Secao("DESC" + dIdx)
		b.KV("cMunDescarga", desc.CMunDescarga)
		b.KV("xMunDescarga", desc.XMunDescarga)

		for i, nfe := range desc.InfNFe {
			idx := dIdx + inifmt.Seq3(i+1)
			b.Secao("infNFe" + idx)
			b.KV("chNFe", nfe.ChNFe)
			b.KVOpt("SegCodBarra", nfe.SegCodBarra)
			b.KVIntOpt("indReentrega", nfe.IndReentrega)
			b.perigosos(idx, nfe.Peri)
			b.unidTransp(idx, nfe.InfUnidTransp)
		}
		for i, c := range desc.InfCTe {
			idx := dIdx + inifmt.Seq3(i+1)
			b.Secao("infCTe" + idx)
			b.KV("chCTe", c.ChCTe)
			b.KVOpt("SegCodBarra", c.SegCodBarra)
			b.KVIntOpt("indReentrega", c.IndReentrega)
			b.KVIntOpt("indPrestacaoParcial", c.IndPrestacaoParcial)
			if ep := c.InfEntregaParcial; ep != nil {
				b.Secao("infEntregaParcial" + idx)
				b.KV("qtdTotal", inifmt.Money(ep.QtdTotal))
				b.KV("qtdParcial", inifmt.Money(ep.QtdParcial))
			}
			for j, nfp := range c.InfNFePrestParcial {
				b.Secao("infNFePrestParcial" + idx + inifmt.Seq3(j+1))
				b.KV("chNFe", nfp.ChNFe)
			}
			b.perigosos(idx, c.Peri)
			b.unidTransp(idx, c.InfUnidTransp)
		}
		for i, m := range desc.InfMDFeTransp {
			idx := dIdx + inifmt.Seq3(i+1)
			b.Secao("infMDFeTransp" + idx)
			b.KV("chMDFe", m.ChMDFe)
			b.KVIntOpt("indReentrega", m.IndReentrega)
			b.perigosos(idx, m.Peri)
			b.unidTransp(idx, m.InfUnidTransp)
		}
	}
}

// perigosos emite os produtos perigosos de um documento transportado.
func (b *iniBuilder) perigosos(idx string, peris []Peri) {
	for i, p := range peris {
		b.peri(idx+inifmt.Seq3(i+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
	}
}

// --- carga, totais e fechamento ---------------------------------------------

// produtoPredominante emite [prodPred] e, na lotação, os locais de carga e
// descarga.
func (b *iniBuilder) produtoPredominante(pp *ProdPred) {
	if pp == nil {
		return
	}
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

// totais emite [tot]. O default de cUnid é 01 (quilograma), que é o que o
// layout espera quando o cliente não informa.
func (b *iniBuilder) totais(t Tot) {
	b.Secao("tot")
	b.KVIntOpt("qCTe", t.QCTe)
	b.KVIntOpt("qNFe", t.QNFe)
	b.KVIntOpt("qMDFe", t.QMDFe)
	b.KV("vCarga", inifmt.Money(t.VCarga))
	b.KV("cUnid", cmp.Or(t.CUnid, "01"))
	b.KV("qCarga", inifmt.Money(t.QCarga))
}

// lacres emite [lacresNNN] (lacres do MDF-e, distintos dos lacres do modal).
func (b *iniBuilder) lacres(ls []Lacres) {
	for i, l := range ls {
		b.Secao("lacres" + inifmt.Seq3(i+1))
		b.KV("nLacre", l.NLacre)
	}
}

// adicionais emite [infAdic] (informações complementares).
func (b *iniBuilder) adicionais(ia *InfAdic) {
	if ia == nil {
		return
	}
	b.Secao("infAdic")
	b.KVOpt("infAdFisco", ia.InfAdFisco)
	b.KVOpt("infCpl", ia.InfCpl)
}

// seguros emite [segNNN] + as averbações de cada seguro.
func (b *iniBuilder) seguros(segs []Seg) {
	for i, s := range segs {
		idx := inifmt.Seq3(i + 1)
		b.Secao("seg" + idx)
		b.KV("respSeg", strconv.Itoa(s.InfResp.RespSeg))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(s.InfResp.CNPJ, s.InfResp.CPF))
		// A seguradora não tem seção própria: vai dentro de [segNNN].
		if is := s.InfSeg; is != nil {
			b.KVOpt("xSeg", is.XSeg)
			b.KVOpt("CNPJ", is.CNPJ)
		}
		b.KVOpt("nApol", s.NApol)
		for j, av := range s.NAver {
			b.Secao("aver" + idx + inifmt.Seq3(j+1))
			b.KV("nAver", av)
		}
	}
}

// autorizados emite [autXMLNN] (quem pode baixar o XML).
//
// Indexação de DUAS casas, não das três que o resto do arquivo usa: verificado
// contra a biblioteca, [autXML001] é ignorado em silêncio e [autXML01] é lido.
func (b *iniBuilder) autorizados(auts []AutXML) {
	for i, a := range auts {
		b.Secao("autXML" + inifmt.Seq2(i+1))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(a.CNPJ, a.CPF))
	}
}

// respTec emite [infRespTec] (responsável técnico).
func (b *iniBuilder) respTec(rt *RespTec) {
	if rt == nil {
		return
	}
	b.Secao("infRespTec")
	b.KV("CNPJ", rt.CNPJ)
	b.KV("xContato", rt.XContato)
	b.KV("email", rt.Email)
	b.KV("fone", rt.Fone)
	b.KVIntOpt("idCSRT", rt.IdCSRT)
	b.KVOpt("hashCSRT", rt.HashCSRT)
}

// --- builder de INI (local ao pacote mdfe) ----------------------------------

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
