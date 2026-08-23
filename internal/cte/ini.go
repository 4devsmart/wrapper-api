package cte

import (
	"cmp"
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// ToINI traduz o pedido (JSON ACBr.API) para o INI do CT-e consumido pela
// ACBrLibCTe (CTE_CarregarINI). Seções/chaves seguem o modelo oficial
// (acbr.sourceforge.io/ACBrLib/ModeloCTeINI.html).
//
// Cobertura: CT-e Normal no MODAL RODOVIÁRIO + grupos comuns (compl/obs/Entrega,
// componentes, todas as variantes de ICMS, IBSCBS da Reforma Tributária,
// infNF/infNFe/infOutros/infDCe, docAnt, occ, unidCarga/unidTransp, cobrança,
// respTec). Para o CT-e Simplificado (tpCTe=5) ver ToINISimp.
//
// Os demais modais estão fora do contrato, e o handler recusa antes de chegar
// aqui (ver modalSuportado, http.go). O recorte fechou três lacunas que a
// cobertura anterior carregava: o peri aéreo e o tarifa.CL divergiam entre o
// modelo e o INI, e o multimodal (COTM) nunca foi lido por INI pela lib. Os
// três eram aceitos e descartados na montagem.
func ToINI(p PedidoEmissao) string {
	inf := p.InfCte
	var b iniBuilder
	b.EmUF(inf.Ide.CUF)

	b.Secao("infCTe")
	b.KV("versao", cmp.Or(inf.Versao, "4.00"))

	b.identificacao(inf.Ide, p.Ambiente)
	b.complemento(inf.Compl)
	b.emitente(inf.Emit)

	if r := inf.Rem; r != nil {
		b.Secao("rem")
		b.pessoa(r.CNPJ, r.CPF, r.IE, r.XNome, r.Fone, r.Email, r.EnderReme)
		// Só rem e toma4 têm xFant no layout: exped, receb e dest não.
		b.KVOpt("xFant", r.XFant)
	}
	if e := inf.Exped; e != nil {
		b.Secao("exped")
		b.pessoa(e.CNPJ, e.CPF, e.IE, e.XNome, e.Fone, e.Email, e.EnderExped)
	}
	if r := inf.Receb; r != nil {
		b.Secao("receb")
		b.pessoa(r.CNPJ, r.CPF, r.IE, r.XNome, r.Fone, r.Email, r.EnderReceb)
	}
	if d := inf.Dest; d != nil {
		b.Secao("Dest")
		b.pessoa(d.CNPJ, d.CPF, d.IE, d.XNome, d.Fone, d.Email, d.EnderDest)
		b.KVOpt("ISUF", d.ISUF)
	}

	b.Secao("vPrest")
	b.KV("vTPrest", inifmt.Money(inf.VPrest.VTPrest))
	b.KV("vRec", inifmt.Money(inf.VPrest.VRec))
	for i, c := range inf.VPrest.Comp {
		b.Secao("Comp" + inifmt.Seq3(i+1))
		b.KV("xNome", c.XNome)
		b.KV("vComp", inifmt.Money(c.VComp))
	}

	b.imposto(inf.Imp)

	if n := inf.InfCTeNorm; n != nil {
		b.infCargaQ(n.InfCarga)
		if d := n.InfDoc; d != nil {
			b.infDoc(d)
		}
		if da := n.DocAnt; da != nil {
			b.docAnt(da)
		}
		b.cobranca(n.Cobr)
		if g := n.InfGlobalizado; g != nil {
			b.Secao("infGlobalizado")
			b.KV("xObs", g.XObs)
		}
		b.subst(n.InfCteSub)
		b.modalRodo(n.InfModal.Rodo)
		for i, vn := range n.VeicNovos {
			b.Secao("veicNovos" + inifmt.Seq3(i+1))
			b.KV("chassi", vn.Chassi)
			b.KVOpt("cCor", vn.CCor)
			b.KVOpt("xCor", vn.XCor)
			b.KVOpt("cMod", vn.CMod)
			b.KVOpt("vUnit", inifmt.MoneyOpt(vn.VUnit))
			b.KVOpt("vFrete", inifmt.MoneyOpt(vn.VFrete))
		}
	}

	// infCteComp e infCTeNorm são ESCOLHA no layout: o CT-e Complementar
	// (tpCTe=1) traz um, o Normal traz o outro. Escrevemos o que vier, e quem
	// decide qual vale é o tpCTe.
	//
	// A indexação aqui é de DUAS casas, não das três que o resto do arquivo
	// usa: verificado contra a biblioteca, [infCteComp001] é ignorado em
	// silêncio e [infCteComp01] é lido. O mesmo vale para autXML abaixo.
	for i, c := range inf.InfCteComp {
		b.Secao("infCteComp" + inifmt.Seq2(i+1))
		b.KV("chCte", c.ChCTe)
	}
	for i, a := range inf.AutXML {
		b.Secao("autXML" + inifmt.Seq2(i+1))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(a.CNPJ, a.CPF))
	}

	b.respTec(inf.InfRespTec)
	return b.String()
}

// identificacao emite a seção [ide] + [toma4] (tomador externo do CT-e Normal).
func (b *iniBuilder) identificacao(id Ide, ambiente string) {
	b.Secao("ide")
	b.KVIntOpt("cUF", id.CUF)
	b.KVOpt("cCT", id.CCT)
	// tpAmb precisa sair no INI. Sem ele a lib escreve o default (produção) no
	// XML enquanto a sessão está configurada para o ambiente pedido, e o próprio
	// ValidarRegrasdeNegocios acusa "252-Rejeição: Ambiente informado diverge do
	// Ambiente de recebimento". Pior: como a transmissão deriva o ambiente do tpAmb do
	// XML, um documento pedido em homologação apontaria para produção.
	//
	// A fonte é SEMPRE o ambiente do pedido, o mesmo que configura a sessão
	// nativa (fiscal.TpAmb e fiscal.AmbienteOrdinal saem da mesma normalização).
	// O tpAmb explícito de infCte.ide é conferido no handler e recusado quando
	// contradiz: eleger um vencedor aqui era a divergência que ninguém via.
	b.KV("tpAmb", fiscal.TpAmb(ambiente))
	b.KVOpt("CFOP", id.CFOP)
	b.KV("natOp", id.NatOp)
	b.KV("mod", strconv.Itoa(cmp.Or(id.Mod, 57)))
	b.KV("serie", strconv.Itoa(id.Serie))
	b.KV("nCT", strconv.Itoa(id.NCT))
	b.KV("dhEmi", b.DataHora(id.DhEmi))
	b.KV("tpImp", strconv.Itoa(cmp.Or(id.TpImp, 1)))
	b.KV("tpEmis", strconv.Itoa(cmp.Or(id.TpEmis, 1)))
	b.KV("tpCTe", strconv.Itoa(id.TpCTe))
	b.KV("procEmi", strconv.Itoa(id.ProcEmi))
	b.KV("verProc", cmp.Or(id.VerProc, versao.Emissor()))
	b.KVIntOpt("indGlobalizado", id.IndGlobalizado)
	b.KVOpt("cMunEnv", id.CMunEnv)
	b.KVOpt("xMunEnv", id.XMunEnv)
	b.KVOpt("UFEnv", id.UFEnv)
	b.KV("modal", cmp.Or(id.Modal, "01")) // 01 = rodoviário
	b.KV("tpServ", strconv.Itoa(id.TpServ))
	b.KV("cMunIni", id.CMunIni)
	b.KV("xMunIni", id.XMunIni)
	b.KV("UFIni", id.UFIni)
	b.KV("cMunFim", id.CMunFim)
	b.KV("xMunFim", id.XMunFim)
	b.KV("UFFim", id.UFFim)
	b.KV("retira", strconv.Itoa(id.Retira))
	b.KVOpt("xDetRetira", id.XDetRetira)
	b.KV("indIEToma", strconv.Itoa(id.IndIEToma))
	b.KVOpt("dhCont", b.DataHoraOpt(id.DhCont))
	b.KVOpt("xJust", id.XJust)
	// gCompraGov não tem seção própria: a lib lê tpEnteGov e pRedutor de [ide].
	if g := id.GCompraGov; g != nil {
		b.KVIntOpt("tpEnteGov", g.TpEnteGov)
		b.KVOpt("pRedutor", inifmt.MoneyOpt(g.PRedutor))
	}
	// O indicador do tomador vai na seção PRÓPRIA, não em [ide]: o leitor da lib
	// faz Ide.toma03.Toma := ReadString('toma3','toma') e toma4.toma :=
	// ReadString('toma4','toma'). Escrito em [ide], o valor era simplesmente
	// ignorado e o CT-e saía com <toma>0</toma>, ou seja, tomador = Remetente,
	// qualquer que fosse o pedido. Achado pelo teste de lockstep.
	if t3 := id.Toma3; t3 != nil {
		b.Secao("toma3")
		b.KV("toma", strconv.Itoa(t3.Toma))
	}
	if t4 := id.Toma4; t4 != nil {
		b.Secao("toma4")
		b.KV("toma", strconv.Itoa(cmp.Or(t4.Toma, 4)))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(t4.CNPJ, t4.CPF))
		b.KVOpt("IE", t4.IE)
		b.KVOpt("xNome", t4.XNome)
		b.KVOpt("xFant", t4.XFant)
		b.KVOpt("fone", t4.Fone)
		b.KVOpt("email", t4.Email)
		b.endereco(t4.EnderToma)
	}
}

// complemento emite a seção [compl] + fluxo/passagem/obsCont/obsFisco.
func (b *iniBuilder) complemento(c *Compl) {
	if c == nil {
		return
	}
	b.Secao("compl")
	b.KVOpt("xCaracAd", c.XCaracAd)
	b.KVOpt("xCaracSer", c.XCaracSer)
	b.KVOpt("xEmi", c.XEmi)
	// fluxo (xOrig/xDest/xRota) vai dentro de [compl]; pass em [PASSnnn].
	if f := c.Fluxo; f != nil {
		b.KVOpt("xOrig", f.XOrig)
		b.KVOpt("xDest", f.XDest)
		b.KVOpt("xRota", f.XRota)
	}
	b.entrega(c.Entrega) // ainda dentro de [compl]
	b.KVOpt("origCalc", c.OrigCalc)
	b.KVOpt("destCalc", c.DestCalc)
	b.KVOpt("xObs", c.XObs)
	if f := c.Fluxo; f != nil {
		for i, ps := range f.Pass {
			b.Secao("PASS" + inifmt.Seq3(i+1))
			b.KV("xPass", ps.XPass)
		}
	}
	for i, o := range c.ObsCont {
		b.Secao("obsCont" + inifmt.Seq3(i+1))
		b.KV("xCampo", o.XCampo)
		b.KV("xTexto", o.XTexto)
	}
	for i, o := range c.ObsFisco {
		b.Secao("obsFisco" + inifmt.Seq3(i+1))
		b.KV("xCampo", o.XCampo)
		b.KV("xTexto", o.XTexto)
	}
}

// entrega emite o agendamento de entrega (grupo Entrega), TUDO dentro de
// [compl]: discriminadores TipoData/TipoHora (0=sem,1/2/3=na/até/apartir,
// 4=período) + dProg/dIni/dFim e tpHor/hProg/hIni/hFim. Ver Ler_Complemento.
func (b *iniBuilder) entrega(e *Entrega) {
	if e == nil {
		return
	}
	switch {
	case e.SemData != nil:
		b.KV("TipoData", "0")
	case e.ComData != nil:
		b.KV("TipoData", strconv.Itoa(e.ComData.TpPer)) // 1/2/3
		b.KV("tpPer", strconv.Itoa(e.ComData.TpPer))
		b.KVOpt("dProg", b.Data(e.ComData.DProg))
	case e.NoPeriodo != nil:
		b.KV("TipoData", "4")
		b.KV("tpPer", strconv.Itoa(e.NoPeriodo.TpPer))
		b.KVOpt("dIni", b.Data(e.NoPeriodo.DIni))
		b.KVOpt("dFim", b.Data(e.NoPeriodo.DFim))
	}
	switch {
	case e.SemHora != nil:
		b.KV("TipoHora", "0")
	case e.ComHora != nil:
		b.KV("TipoHora", strconv.Itoa(e.ComHora.TpHor)) // 1/2/3
		b.KV("tpHor", strconv.Itoa(e.ComHora.TpHor))
		b.KVOpt("hProg", e.ComHora.HProg)
	case e.NoInter != nil:
		b.KV("TipoHora", "4")
		b.KV("tpHor", strconv.Itoa(e.NoInter.TpHor))
		b.KVOpt("hIni", e.NoInter.HIni)
		b.KVOpt("hFim", e.NoInter.HFim)
	}
}

// emitente emite a seção [emit].
func (b *iniBuilder) emitente(e Emit) {
	b.Secao("emit")
	b.KVOpt("CNPJ", fiscal.Primeiro(e.CNPJ, e.CPF))
	b.KVOpt("IE", e.IE)
	b.KVOpt("IEST", e.IEST)
	b.KVOpt("xNome", e.XNome)
	b.KVOpt("xFant", e.XFant)
	b.KVIntOpt("CRT", e.CRT)
	b.endeEmi(e.EnderEmit)
}

// imposto emite [Imp] + a variante de ICMS + ICMSUFFim + IBSCBS (Reforma).
func (b *iniBuilder) imposto(imp Imp) {
	b.Secao("Imp")
	b.KVOpt("vTotTrib", inifmt.MoneyOpt(imp.VTotTrib))
	b.KVOpt("vTotDFe", inifmt.MoneyOpt(imp.VTotDFe))
	b.KVOpt("infAdFisco", imp.InfAdFisco)
	b.icms(imp.ICMS)
	if u := imp.ICMSUFFim; u != nil {
		b.Secao("ICMSUFFim")
		b.KV("vBCUFFim", inifmt.Money(u.VBCUFFim))
		b.KV("pFCPUFFim", inifmt.Money(u.PFCPUFFim))
		b.KV("pICMSUFFim", inifmt.Money(u.PICMSUFFim))
		b.KV("pICMSInter", inifmt.Money(u.PICMSInter))
		b.KV("vFCPUFFim", inifmt.Money(u.VFCPUFFim))
		b.KV("vICMSUFFim", inifmt.Money(u.VICMSUFFim))
		b.KV("vICMSUFIni", inifmt.Money(u.VICMSUFIni))
	}
	if t := imp.IBSCBS; t != nil {
		b.Secao("IBSCBS")
		b.KV("CST", t.CST)
		b.KVOpt("cClassTrib", t.CClassTrib)
		b.KVIntOpt("indDoacao", t.IndDoacao)
		if c := t.GIBSCBS; c != nil {
			b.Secao("gIBSCBS")
			b.KV("vBC", inifmt.Money(c.VBC))
			b.KV("vIBS", inifmt.Money(c.VIBS))
			b.Secao("gIBSUF")
			b.KV("pIBSUF", inifmt.Money(c.GIBSUF.PIBSUF))
			b.KV("vIBSUF", inifmt.Money(c.GIBSUF.VIBSUF))
			b.gDifDevRed(c.GIBSUF.GDif, c.GIBSUF.GDevTrib, c.GIBSUF.GRed)
			b.Secao("gIBSMun")
			b.KV("pIBSMun", inifmt.Money(c.GIBSMun.PIBSMun))
			b.KV("vIBSMun", inifmt.Money(c.GIBSMun.VIBSMun))
			b.gDifDevRed(c.GIBSMun.GDif, c.GIBSMun.GDevTrib, c.GIBSMun.GRed)
			b.Secao("gCBS")
			b.KV("pCBS", inifmt.Money(c.GCBS.PCBS))
			b.KV("vCBS", inifmt.Money(c.GCBS.VCBS))
			b.gDifDevRed(c.GCBS.GDif, c.GCBS.GDevTrib, c.GCBS.GRed)
			if r := c.GTribRegular; r != nil {
				b.Secao("gTribRegular")
				b.KV("CSTReg", r.CSTReg)
				b.KV("cClassTribReg", r.CClassTribReg)
				b.KV("pAliqEfetRegIBSUF", inifmt.Money(r.PAliqEfetRegIBSUF))
				b.KV("vTribRegIBSUF", inifmt.Money(r.VTribRegIBSUF))
				b.KV("pAliqEfetRegIBSMun", inifmt.Money(r.PAliqEfetRegIBSMun))
				b.KV("vTribRegIBSMun", inifmt.Money(r.VTribRegIBSMun))
				b.KV("pAliqEfetRegCBS", inifmt.Money(r.PAliqEfetRegCBS))
				b.KV("vTribRegCBS", inifmt.Money(r.VTribRegCBS))
			}
			if cg := c.GTribCompraGov; cg != nil {
				b.Secao("gTribCompraGov")
				b.KVOpt("pAliqIBSUF", inifmt.MoneyOpt(cg.PAliqIBSUF))
				b.KV("vTribIBSUF", inifmt.Money(cg.VTribIBSUF))
				b.KVOpt("pAliqIBSMun", inifmt.MoneyOpt(cg.PAliqIBSMun))
				b.KV("vTribIBSMun", inifmt.Money(cg.VTribIBSMun))
				b.KVOpt("pAliqCBS", inifmt.MoneyOpt(cg.PAliqCBS))
				b.KV("vTribCBS", inifmt.Money(cg.VTribCBS))
			}
		}
		if ec := t.GEstornoCred; ec != nil {
			b.Secao("gEstornoCred")
			b.KV("vIBSEstCred", inifmt.Money(ec.VIBSEstCred))
			b.KV("vCBSEstCred", inifmt.Money(ec.VCBSEstCred))
		}
	}
}

// infCargaQ emite [infCarga] + [infQnnn].
func (b *iniBuilder) infCargaQ(c InfCarga) {
	b.Secao("infCarga")
	b.KVOpt("vCarga", inifmt.MoneyOpt(c.VCarga))
	b.KV("proPred", c.ProPred)
	b.KVOpt("xOutCat", c.XOutCat)
	b.KVOpt("vCargaAverb", inifmt.MoneyOpt(c.VCargaAverb))
	for i, q := range c.InfQ {
		b.Secao("infQ" + inifmt.Seq3(i+1))
		b.KV("cUnid", q.CUnid)
		b.KV("tpMed", q.TpMed)
		b.KV("qCarga", inifmt.Money(q.QCarga))
	}
}

// infDoc emite os documentos transportados (infNF/infNFe/infOutros/infDCe).
func (b *iniBuilder) infDoc(d *InfDoc) {
	for i, nf := range d.InfNF {
		b.Secao("infNF" + inifmt.Seq3(i+1))
		b.KVOpt("nRoma", nf.NRoma)
		b.KVOpt("nPed", nf.NPed)
		b.KV("mod", nf.Mod)
		b.KV("serie", nf.Serie)
		b.KV("nDoc", nf.NDoc)
		b.KVOpt("dEmi", b.Data(nf.DEmi))
		b.KV("vBC", inifmt.Money(nf.VBC))
		b.KV("vICMS", inifmt.Money(nf.VICMS))
		b.KV("vBCST", inifmt.Money(nf.VBCST))
		b.KV("vST", inifmt.Money(nf.VST))
		b.KV("vProd", inifmt.Money(nf.VProd))
		b.KV("vNF", inifmt.Money(nf.VNF))
		b.KV("nCFOP", nf.NCFOP)
		b.KVOpt("nPeso", inifmt.MoneyOpt(nf.NPeso))
		b.KVOpt("PIN", nf.PIN)
		b.KVOpt("dPrev", b.Data(nf.DPrev))
		b.unidades(inifmt.Seq3(i+1), nf.InfUnidCarga, nf.InfUnidTransp)
	}
	for i, nfe := range d.InfNFe {
		b.Secao("infNFe" + inifmt.Seq3(i+1))
		b.KV("chave", nfe.Chave)
		b.KVOpt("PIN", nfe.PIN)
		b.KVOpt("dPrev", b.Data(nfe.DPrev))
		b.unidades(inifmt.Seq3(i+1), nfe.InfUnidCarga, nfe.InfUnidTransp)
	}
	for i, o := range d.InfOutros {
		b.Secao("infOutros" + inifmt.Seq3(i+1))
		b.KV("tpDoc", o.TpDoc)
		b.KVOpt("descOutros", o.DescOutros)
		b.KVOpt("nDoc", o.NDoc)
		b.KVOpt("dEmi", b.Data(o.DEmi))
		b.KVOpt("vDocFisc", inifmt.MoneyOpt(o.VDocFisc))
		b.KVOpt("dPrev", b.Data(o.DPrev))
		b.unidades(inifmt.Seq3(i+1), o.InfUnidCarga, o.InfUnidTransp)
	}
	for i, dce := range d.InfDCe {
		b.Secao("infDCe" + inifmt.Seq3(i+1))
		b.KV("chave", dce.Chave)
	}
}

// docAnt emite os documentos anteriores ([emiDocAnt]/[idDocAntPap]/[idDocAntEle]).
func (b *iniBuilder) docAnt(da *DocAnt) {
	for ei, emi := range da.EmiDocAnt {
		b.Secao("emiDocAnt" + inifmt.Seq3(ei+1))
		b.KVOpt("CNPJCPF", fiscal.Primeiro(emi.CNPJ, emi.CPF))
		b.KVOpt("IE", emi.IE)
		b.KVOpt("UF", emi.UF)
		b.KVOpt("xNome", emi.XNome)
		for _, idd := range emi.IdDocAnt {
			for pi, pap := range idd.IdDocAntPap {
				b.Secao("idDocAntPap" + inifmt.Seq3(ei+1) + inifmt.Seq3(pi+1))
				b.KV("tpDoc", pap.TpDoc)
				b.KV("serie", pap.Serie)
				b.KVOpt("subser", pap.Subser)
				b.KV("nDoc", pap.NDoc)
				b.KVOpt("dEmi", b.Data(pap.DEmi))
			}
			for ki, ele := range idd.IdDocAntEle {
				b.Secao("idDocAntEle" + inifmt.Seq3(ei+1) + inifmt.Seq3(ki+1))
				b.KV("chCTe", ele.ChCTe)
			}
		}
	}
}

// cobranca emite [cobr] + [dupnnn].
func (b *iniBuilder) cobranca(c *Cobr) {
	if c == nil {
		return
	}
	b.Secao("cobr")
	if f := c.Fat; f != nil {
		b.KVOpt("nFat", f.NFat)
		b.KVOpt("vOrig", inifmt.MoneyOpt(f.VOrig))
		b.KVOpt("vDesc", inifmt.MoneyOpt(f.VDesc))
		b.KVOpt("vLiq", inifmt.MoneyOpt(f.VLiq))
	}
	for i, d := range c.Dup {
		b.Secao("dup" + inifmt.Seq3(i+1))
		b.KVOpt("nDup", d.NDup)
		b.KVOpt("dVenc", b.Data(d.DVenc))
		b.KV("vDup", inifmt.Money(d.VDup))
	}
}

// subst emite [infCteSub] (CT-e de substituição).
func (b *iniBuilder) subst(s *InfCteSub) {
	if s == nil {
		return
	}
	b.Secao("infCteSub")
	b.KV("chCte", s.ChCte)
	b.KVIntOpt("indAlteraToma", s.IndAlteraToma)
}

// modalRodo emite [Rodo] + [occnnn] (modal rodoviário).
func (b *iniBuilder) modalRodo(rodo *Rodo) {
	if rodo == nil {
		return
	}
	b.Secao("Rodo")
	b.KV("RNTRC", rodo.RNTRC)
	for i, o := range rodo.Occ {
		b.Secao("occ" + inifmt.Seq3(i+1))
		b.KVOpt("serie", o.Serie)
		b.KV("nOcc", strconv.Itoa(o.NOcc))
		b.KVOpt("dEmi", b.Data(o.DEmi))
		if e := o.EmiOcc; e != nil {
			b.KVOpt("CNPJ", e.CNPJ)
			b.KVOpt("cInt", e.CInt)
			b.KVOpt("IE", e.IE)
			b.KVOpt("UF", e.UF)
			b.KVOpt("fone", e.Fone)
		}
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

// icms emite a seção da variante de ICMS preenchida (apenas uma deve vir).
func (b *iniBuilder) icms(ic ICMS) {
	switch {
	case ic.ICMS00 != nil:
		b.Secao("ICMS00")
		b.KV("CST", cmp.Or(ic.ICMS00.CST, "00"))
		b.KV("vBC", inifmt.Money(ic.ICMS00.VBC))
		b.KV("pICMS", inifmt.Money(ic.ICMS00.PICMS))
		b.KV("vICMS", inifmt.Money(ic.ICMS00.VICMS))
	case ic.ICMS20 != nil:
		b.Secao("ICMS20")
		b.KV("CST", cmp.Or(ic.ICMS20.CST, "20"))
		b.KV("pRedBC", inifmt.Money(ic.ICMS20.PRedBC))
		b.KV("vBC", inifmt.Money(ic.ICMS20.VBC))
		b.KV("pICMS", inifmt.Money(ic.ICMS20.PICMS))
		b.KV("vICMS", inifmt.Money(ic.ICMS20.VICMS))
		b.KVOpt("vICMSDeson", inifmt.MoneyOpt(ic.ICMS20.VICMSDeson))
		b.KVOpt("cBenef", ic.ICMS20.CBenef)
	case ic.ICMS45 != nil:
		b.Secao("ICMS45")
		b.KV("CST", cmp.Or(ic.ICMS45.CST, "40"))
		b.KVOpt("vICMSDeson", inifmt.MoneyOpt(ic.ICMS45.VICMSDeson))
		b.KVOpt("cBenef", ic.ICMS45.CBenef)
	case ic.ICMS60 != nil:
		b.Secao("ICMS60")
		b.KV("CST", cmp.Or(ic.ICMS60.CST, "60"))
		b.KV("vBCSTRet", inifmt.Money(ic.ICMS60.VBCSTRet))
		b.KV("vICMSSTRet", inifmt.Money(ic.ICMS60.VICMSSTRet))
		b.KV("pICMSSTRet", inifmt.Money(ic.ICMS60.PICMSSTRet))
		b.KVOpt("vCred", inifmt.MoneyOpt(ic.ICMS60.VCred))
		b.KVOpt("vICMSDeson", inifmt.MoneyOpt(ic.ICMS60.VICMSDeson))
		b.KVOpt("cBenef", ic.ICMS60.CBenef)
	case ic.ICMS90 != nil:
		b.Secao("ICMS90")
		b.KV("CST", cmp.Or(ic.ICMS90.CST, "90"))
		b.KVOpt("pRedBC", inifmt.MoneyOpt(ic.ICMS90.PRedBC))
		b.KV("vBC", inifmt.Money(ic.ICMS90.VBC))
		b.KV("pICMS", inifmt.Money(ic.ICMS90.PICMS))
		b.KV("vICMS", inifmt.Money(ic.ICMS90.VICMS))
		b.KVOpt("vCred", inifmt.MoneyOpt(ic.ICMS90.VCred))
		b.KVOpt("vICMSDeson", inifmt.MoneyOpt(ic.ICMS90.VICMSDeson))
		b.KVOpt("cBenef", ic.ICMS90.CBenef)
	case ic.ICMSOutraUF != nil:
		b.Secao("ICMSOutraUF")
		b.KV("CST", cmp.Or(ic.ICMSOutraUF.CST, "90"))
		b.KVOpt("pRedBCOutraUF", inifmt.MoneyOpt(ic.ICMSOutraUF.PRedBCOutraUF))
		b.KV("vBCOutraUF", inifmt.Money(ic.ICMSOutraUF.VBCOutraUF))
		b.KV("pICMSOutraUF", inifmt.Money(ic.ICMSOutraUF.PICMSOutraUF))
		b.KV("vICMSOutraUF", inifmt.Money(ic.ICMSOutraUF.VICMSOutraUF))
		b.KVOpt("vICMSDeson", inifmt.MoneyOpt(ic.ICMSOutraUF.VICMSDeson))
		b.KVOpt("cBenef", ic.ICMSOutraUF.CBenef)
	case ic.ICMSSN != nil:
		b.Secao("ICMSSN")
		b.KV("CST", cmp.Or(ic.ICMSSN.CST, "90"))
		b.KV("indSN", strconv.Itoa(cmp.Or(ic.ICMSSN.IndSN, 1)))
	}
}

// --- builder de INI (local ao pacote cte) -----------------------------------

// iniBuilder acumula o INI. Carrega o fuso do documento porque data-hora só faz
// sentido com ele: o cliente informa um instante (RFC 3339) e o documento fiscal
// precisa desse instante no relógio do emitente.
// iniBuilder é o construtor compartilhado (internal/platform/inifmt) mais os
// métodos de DOMÍNIO deste documento, logo abaixo. O núcleo (seção, par
// chave=valor, datas no fuso do emitente) vive num lugar só: era o mesmo código
// nos quatro documentos, e um ajuste na sanitização precisava ser feito quatro
// vezes.
type iniBuilder struct{ inifmt.Builder }

// kvInt emite sempre (inclusive 0). Para campos obrigatórios cujo zero é um valor
// válido: ex.: tpNav=0 (navegação interior), obrigatório no aquaviário v4.00.

// pessoa escreve rem/exped/receb/Dest (CNPJCPF único + endereço inline).
func (b *iniBuilder) pessoa(cnpj, cpf, ie, xNome, fone, email string, e Endereco) {
	b.KVOpt("CNPJCPF", fiscal.Primeiro(cnpj, cpf))
	b.KVOpt("IE", ie)
	b.KVOpt("xNome", xNome)
	b.KVOpt("fone", fone)
	b.KVOpt("email", email)
	b.endereco(e)
}

func (b *iniBuilder) endereco(e Endereco) {
	b.KVOpt("xLgr", e.XLgr)
	b.KVOpt("nro", e.Nro)
	b.KVOpt("xCpl", e.XCpl)
	b.KVOpt("xBairro", e.XBairro)
	b.KVOpt("cMun", e.CMun)
	b.KVOpt("xMun", e.XMun)
	b.KVOpt("CEP", e.CEP)
	b.KVOpt("UF", e.UF)
	b.KVOpt("cPais", e.CPais)
	b.KVOpt("xPais", e.XPais)
}

func (b *iniBuilder) endeEmi(e *EndeEmi) {
	if e == nil {
		return
	}
	b.KVOpt("xLgr", e.XLgr)
	b.KVOpt("nro", e.Nro)
	b.KVOpt("xCpl", e.XCpl)
	b.KVOpt("xBairro", e.XBairro)
	b.KVOpt("cMun", e.CMun)
	b.KVOpt("xMun", e.XMun)
	b.KVOpt("CEP", e.CEP)
	b.KVOpt("UF", e.UF)
	b.KVOpt("fone", e.Fone)
}

// unidades emite infUnidCarga/infUnidTransp (+lacres) de um documento, com o
// prefixo de índice do doc (ex.: "001"). Cargas no nível do doc usam {doc}{c};
// dentro de uma unidade de transporte usam {doc}{transp}{c}.
func (b *iniBuilder) unidades(docIdx string, cargas []UnidCarga, transps []UnidadeTransp) {
	for ci, c := range cargas {
		b.unidCarga(docIdx+inifmt.Seq3(ci+1), c)
	}
	for ti, tp := range transps {
		tIdx := docIdx + inifmt.Seq3(ti+1)
		b.Secao("infUnidTransp" + tIdx)
		b.KV("tpUnidTransp", strconv.Itoa(tp.TpUnidTransp))
		b.KV("idUnidTransp", tp.IdUnidTransp)
		b.KVOpt("qtdRat", inifmt.MoneyOpt(tp.QtdRat))
		for li, l := range tp.LacUnidTransp {
			b.Secao("lacUnidTransp" + tIdx + inifmt.Seq3(li+1))
			b.KV("nLacre", l.NLacre)
		}
		for ci, c := range tp.InfUnidCarga {
			b.unidCarga(tIdx+inifmt.Seq3(ci+1), c)
		}
	}
}

// gDifDevRed achata gDif/gDevTrib/gRed dentro da seção corrente (gIBSUF/
// gIBSMun/gCBS), conforme o modelo INI da Reforma Tributária.
func (b *iniBuilder) gDifDevRed(dif *Dif, dev *DevTrib, red *Red) {
	if dif != nil {
		b.KVOpt("pDif", inifmt.MoneyOpt(dif.PDif))
		b.KVOpt("vDif", inifmt.MoneyOpt(dif.VDif))
	}
	if dev != nil {
		b.KVOpt("vDevTrib", inifmt.MoneyOpt(dev.VDevTrib))
	}
	if red != nil {
		b.KVOpt("pRedAliq", inifmt.MoneyOpt(red.PRedAliq))
		b.KVOpt("pAliqEfet", inifmt.MoneyOpt(red.PAliqEfet))
	}
}

func (b *iniBuilder) unidCarga(idx string, c UnidCarga) {
	b.Secao("infUnidCarga" + idx)
	b.KV("tpUnidCarga", strconv.Itoa(c.TpUnidCarga))
	b.KV("idUnidCarga", c.IdUnidCarga)
	b.KVOpt("qtdRat", inifmt.MoneyOpt(c.QtdRat))
	for li, l := range c.LacUnidCarga {
		b.Secao("lacUnidCarga" + idx + inifmt.Seq3(li+1))
		b.KV("nLacre", l.NLacre)
	}
}

// --- helpers ----------------------------------------------------------------
