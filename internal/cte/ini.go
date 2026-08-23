package cte

import (
	"strconv"
	"strings"
	"time"

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
	b.loc = inifmt.LocalDoCodigoUF(inf.Ide.CUF)

	b.section("infCTe")
	b.kv("versao", defaultStr(inf.Versao, "4.00"))

	b.identificacao(inf.Ide, p.Ambiente)
	b.complemento(inf.Compl)
	b.emitente(inf.Emit)

	if r := inf.Rem; r != nil {
		b.section("rem")
		b.pessoa(r.CNPJ, r.CPF, r.IE, r.XNome, r.Fone, r.Email, r.EnderReme)
	}
	if e := inf.Exped; e != nil {
		b.section("exped")
		b.pessoa(e.CNPJ, e.CPF, e.IE, e.XNome, e.Fone, e.Email, e.EnderExped)
	}
	if r := inf.Receb; r != nil {
		b.section("receb")
		b.pessoa(r.CNPJ, r.CPF, r.IE, r.XNome, r.Fone, r.Email, r.EnderReceb)
	}
	if d := inf.Dest; d != nil {
		b.section("Dest")
		b.pessoa(d.CNPJ, d.CPF, d.IE, d.XNome, d.Fone, d.Email, d.EnderDest)
		b.kvOpt("ISUF", d.ISUF)
	}

	b.section("vPrest")
	b.kv("vTPrest", money(inf.VPrest.VTPrest))
	b.kv("vRec", money(inf.VPrest.VRec))
	for i, c := range inf.VPrest.Comp {
		b.section("Comp" + seq(i+1))
		b.kv("xNome", c.XNome)
		b.kv("vComp", money(c.VComp))
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
			b.section("infGlobalizado")
			b.kv("xObs", g.XObs)
		}
		b.subst(n.InfCteSub)
		b.modalRodo(n.InfModal.Rodo)
		for i, vn := range n.VeicNovos {
			b.section("veicNovos" + seq(i+1))
			b.kv("chassi", vn.Chassi)
			b.kvOpt("cCor", vn.CCor)
			b.kvOpt("xCor", vn.XCor)
			b.kvOpt("cMod", vn.CMod)
			b.kvOpt("vUnit", moneyOpt(vn.VUnit))
			b.kvOpt("vFrete", moneyOpt(vn.VFrete))
		}
	}

	b.respTec(inf.InfRespTec)
	return b.String()
}

// identificacao emite a seção [ide] + [toma4] (tomador externo do CT-e Normal).
func (b *iniBuilder) identificacao(id Ide, ambiente string) {
	b.section("ide")
	b.kvIntOpt("cUF", id.CUF)
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
	b.kv("tpAmb", fiscal.TpAmb(ambiente))
	b.kvOpt("CFOP", id.CFOP)
	b.kv("natOp", id.NatOp)
	b.kv("mod", strconv.Itoa(defaultInt(id.Mod, 57)))
	b.kv("serie", strconv.Itoa(id.Serie))
	b.kv("nCT", strconv.Itoa(id.NCT))
	b.kv("dhEmi", b.dataHora(id.DhEmi))
	b.kv("tpImp", strconv.Itoa(defaultInt(id.TpImp, 1)))
	b.kv("tpEmis", strconv.Itoa(defaultInt(id.TpEmis, 1)))
	b.kv("tpCTe", strconv.Itoa(id.TpCTe))
	b.kv("procEmi", strconv.Itoa(id.ProcEmi))
	b.kv("verProc", defaultStr(id.VerProc, versao.Emissor()))
	b.kvIntOpt("indGlobalizado", id.IndGlobalizado)
	b.kvOpt("cMunEnv", id.CMunEnv)
	b.kvOpt("xMunEnv", id.XMunEnv)
	b.kvOpt("UFEnv", id.UFEnv)
	b.kv("modal", defaultStr(id.Modal, "01")) // 01 = rodoviário
	b.kv("tpServ", strconv.Itoa(id.TpServ))
	b.kv("cMunIni", id.CMunIni)
	b.kv("xMunIni", id.XMunIni)
	b.kv("UFIni", id.UFIni)
	b.kv("cMunFim", id.CMunFim)
	b.kv("xMunFim", id.XMunFim)
	b.kv("UFFim", id.UFFim)
	b.kv("retira", strconv.Itoa(id.Retira))
	b.kvOpt("xDetRetira", id.XDetRetira)
	b.kv("indIEToma", strconv.Itoa(id.IndIEToma))
	b.kvOpt("dhCont", b.dataHoraOpt(id.DhCont))
	b.kvOpt("xJust", id.XJust)
	// O indicador do tomador vai na seção PRÓPRIA, não em [ide]: o leitor da lib
	// faz Ide.toma03.Toma := ReadString('toma3','toma') e toma4.toma :=
	// ReadString('toma4','toma'). Escrito em [ide], o valor era simplesmente
	// ignorado e o CT-e saía com <toma>0</toma>, ou seja, tomador = Remetente,
	// qualquer que fosse o pedido. Achado pelo teste de lockstep.
	if t3 := id.Toma3; t3 != nil {
		b.section("toma3")
		b.kv("toma", strconv.Itoa(t3.Toma))
	}
	if t4 := id.Toma4; t4 != nil {
		b.section("toma4")
		b.kv("toma", strconv.Itoa(defaultInt(t4.Toma, 4)))
		b.kvOpt("CNPJCPF", firstNonEmpty(t4.CNPJ, t4.CPF))
		b.kvOpt("IE", t4.IE)
		b.kvOpt("xNome", t4.XNome)
		b.kvOpt("fone", t4.Fone)
		b.kvOpt("email", t4.Email)
		b.endereco(t4.EnderToma)
	}
}

// complemento emite a seção [compl] + fluxo/passagem/obsCont/obsFisco.
func (b *iniBuilder) complemento(c *Compl) {
	if c == nil {
		return
	}
	b.section("compl")
	b.kvOpt("xCaracAd", c.XCaracAd)
	b.kvOpt("xCaracSer", c.XCaracSer)
	b.kvOpt("xEmi", c.XEmi)
	// fluxo (xOrig/xDest/xRota) vai dentro de [compl]; pass em [PASSnnn].
	if f := c.Fluxo; f != nil {
		b.kvOpt("xOrig", f.XOrig)
		b.kvOpt("xDest", f.XDest)
		b.kvOpt("xRota", f.XRota)
	}
	b.entrega(c.Entrega) // ainda dentro de [compl]
	b.kvOpt("origCalc", c.OrigCalc)
	b.kvOpt("destCalc", c.DestCalc)
	b.kvOpt("xObs", c.XObs)
	if f := c.Fluxo; f != nil {
		for i, ps := range f.Pass {
			b.section("PASS" + seq(i+1))
			b.kv("xPass", ps.XPass)
		}
	}
	for i, o := range c.ObsCont {
		b.section("obsCont" + seq(i+1))
		b.kv("xCampo", o.XCampo)
		b.kv("xTexto", o.XTexto)
	}
	for i, o := range c.ObsFisco {
		b.section("obsFisco" + seq(i+1))
		b.kv("xCampo", o.XCampo)
		b.kv("xTexto", o.XTexto)
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
		b.kv("TipoData", "0")
	case e.ComData != nil:
		b.kv("TipoData", strconv.Itoa(e.ComData.TpPer)) // 1/2/3
		b.kv("tpPer", strconv.Itoa(e.ComData.TpPer))
		b.kvOpt("dProg", b.dataOpt(e.ComData.DProg))
	case e.NoPeriodo != nil:
		b.kv("TipoData", "4")
		b.kv("tpPer", strconv.Itoa(e.NoPeriodo.TpPer))
		b.kvOpt("dIni", b.dataOpt(e.NoPeriodo.DIni))
		b.kvOpt("dFim", b.dataOpt(e.NoPeriodo.DFim))
	}
	switch {
	case e.SemHora != nil:
		b.kv("TipoHora", "0")
	case e.ComHora != nil:
		b.kv("TipoHora", strconv.Itoa(e.ComHora.TpHor)) // 1/2/3
		b.kv("tpHor", strconv.Itoa(e.ComHora.TpHor))
		b.kvOpt("hProg", e.ComHora.HProg)
	case e.NoInter != nil:
		b.kv("TipoHora", "4")
		b.kv("tpHor", strconv.Itoa(e.NoInter.TpHor))
		b.kvOpt("hIni", e.NoInter.HIni)
		b.kvOpt("hFim", e.NoInter.HFim)
	}
}

// emitente emite a seção [emit].
func (b *iniBuilder) emitente(e Emit) {
	b.section("emit")
	b.kvOpt("CNPJ", firstNonEmpty(e.CNPJ, e.CPF))
	b.kvOpt("IE", e.IE)
	b.kvOpt("IEST", e.IEST)
	b.kvOpt("xNome", e.XNome)
	b.kvOpt("xFant", e.XFant)
	b.kvIntOpt("CRT", e.CRT)
	b.endeEmi(e.EnderEmit)
}

// imposto emite [Imp] + a variante de ICMS + ICMSUFFim + IBSCBS (Reforma).
func (b *iniBuilder) imposto(imp Imp) {
	b.section("Imp")
	b.kvOpt("vTotTrib", moneyOpt(imp.VTotTrib))
	b.kvOpt("infAdFisco", imp.InfAdFisco)
	b.icms(imp.ICMS)
	if u := imp.ICMSUFFim; u != nil {
		b.section("ICMSUFFim")
		b.kv("vBCUFFim", money(u.VBCUFFim))
		b.kv("pFCPUFFim", money(u.PFCPUFFim))
		b.kv("pICMSUFFim", money(u.PICMSUFFim))
		b.kv("pICMSInter", money(u.PICMSInter))
		b.kv("vFCPUFFim", money(u.VFCPUFFim))
		b.kv("vICMSUFFim", money(u.VICMSUFFim))
		b.kv("vICMSUFIni", money(u.VICMSUFIni))
	}
	if t := imp.IBSCBS; t != nil {
		b.section("IBSCBS")
		b.kv("CST", t.CST)
		b.kvOpt("cClassTrib", t.CClassTrib)
		b.kvIntOpt("indDoacao", t.IndDoacao)
		if c := t.GIBSCBS; c != nil {
			b.section("gIBSCBS")
			b.kv("vBC", money(c.VBC))
			b.kv("vIBS", money(c.VIBS))
			b.section("gIBSUF")
			b.kv("pIBSUF", money(c.GIBSUF.PIBSUF))
			b.kv("vIBSUF", money(c.GIBSUF.VIBSUF))
			b.gDifDevRed(c.GIBSUF.GDif, c.GIBSUF.GDevTrib, c.GIBSUF.GRed)
			b.section("gIBSMun")
			b.kv("pIBSMun", money(c.GIBSMun.PIBSMun))
			b.kv("vIBSMun", money(c.GIBSMun.VIBSMun))
			b.gDifDevRed(c.GIBSMun.GDif, c.GIBSMun.GDevTrib, c.GIBSMun.GRed)
			b.section("gCBS")
			b.kv("pCBS", money(c.GCBS.PCBS))
			b.kv("vCBS", money(c.GCBS.VCBS))
			b.gDifDevRed(c.GCBS.GDif, c.GCBS.GDevTrib, c.GCBS.GRed)
			if r := c.GTribRegular; r != nil {
				b.section("gTribRegular")
				b.kv("CSTReg", r.CSTReg)
				b.kv("cClassTribReg", r.CClassTribReg)
				b.kv("pAliqEfetRegIBSUF", money(r.PAliqEfetRegIBSUF))
				b.kv("vTribRegIBSUF", money(r.VTribRegIBSUF))
				b.kv("pAliqEfetRegIBSMun", money(r.PAliqEfetRegIBSMun))
				b.kv("vTribRegIBSMun", money(r.VTribRegIBSMun))
				b.kv("pAliqEfetRegCBS", money(r.PAliqEfetRegCBS))
				b.kv("vTribRegCBS", money(r.VTribRegCBS))
			}
			if cg := c.GTribCompraGov; cg != nil {
				b.section("gTribCompraGov")
				b.kvOpt("pAliqIBSUF", moneyOpt(cg.PAliqIBSUF))
				b.kv("vTribIBSUF", money(cg.VTribIBSUF))
				b.kvOpt("pAliqIBSMun", moneyOpt(cg.PAliqIBSMun))
				b.kv("vTribIBSMun", money(cg.VTribIBSMun))
				b.kvOpt("pAliqCBS", moneyOpt(cg.PAliqCBS))
				b.kv("vTribCBS", money(cg.VTribCBS))
			}
		}
		if ec := t.GEstornoCred; ec != nil {
			b.section("gEstornoCred")
			b.kv("vIBSEstCred", money(ec.VIBSEstCred))
			b.kv("vCBSEstCred", money(ec.VCBSEstCred))
		}
	}
}

// infCargaQ emite [infCarga] + [infQnnn].
func (b *iniBuilder) infCargaQ(c InfCarga) {
	b.section("infCarga")
	b.kvOpt("vCarga", moneyOpt(c.VCarga))
	b.kv("proPred", c.ProPred)
	b.kvOpt("xOutCat", c.XOutCat)
	b.kvOpt("vCargaAverb", moneyOpt(c.VCargaAverb))
	for i, q := range c.InfQ {
		b.section("infQ" + seq(i+1))
		b.kv("cUnid", q.CUnid)
		b.kv("tpMed", q.TpMed)
		b.kv("qCarga", money(q.QCarga))
	}
}

// infDoc emite os documentos transportados (infNF/infNFe/infOutros/infDCe).
func (b *iniBuilder) infDoc(d *InfDoc) {
	for i, nf := range d.InfNF {
		b.section("infNF" + seq(i+1))
		b.kvOpt("nRoma", nf.NRoma)
		b.kvOpt("nPed", nf.NPed)
		b.kv("mod", nf.Mod)
		b.kv("serie", nf.Serie)
		b.kv("nDoc", nf.NDoc)
		b.kvOpt("dEmi", b.dataOpt(nf.DEmi))
		b.kv("vBC", money(nf.VBC))
		b.kv("vICMS", money(nf.VICMS))
		b.kv("vBCST", money(nf.VBCST))
		b.kv("vST", money(nf.VST))
		b.kv("vProd", money(nf.VProd))
		b.kv("vNF", money(nf.VNF))
		b.kv("nCFOP", nf.NCFOP)
		b.kvOpt("nPeso", moneyOpt(nf.NPeso))
		b.kvOpt("PIN", nf.PIN)
		b.kvOpt("dPrev", b.dataOpt(nf.DPrev))
		b.unidades(seq(i+1), nf.InfUnidCarga, nf.InfUnidTransp)
	}
	for i, nfe := range d.InfNFe {
		b.section("infNFe" + seq(i+1))
		b.kv("chave", nfe.Chave)
		b.kvOpt("PIN", nfe.PIN)
		b.kvOpt("dPrev", b.dataOpt(nfe.DPrev))
		b.unidades(seq(i+1), nfe.InfUnidCarga, nfe.InfUnidTransp)
	}
	for i, o := range d.InfOutros {
		b.section("infOutros" + seq(i+1))
		b.kv("tpDoc", o.TpDoc)
		b.kvOpt("descOutros", o.DescOutros)
		b.kvOpt("nDoc", o.NDoc)
		b.kvOpt("dEmi", b.dataOpt(o.DEmi))
		b.kvOpt("vDocFisc", moneyOpt(o.VDocFisc))
		b.kvOpt("dPrev", b.dataOpt(o.DPrev))
		b.unidades(seq(i+1), o.InfUnidCarga, o.InfUnidTransp)
	}
	for i, dce := range d.InfDCe {
		b.section("infDCe" + seq(i+1))
		b.kv("chave", dce.Chave)
	}
}

// docAnt emite os documentos anteriores ([emiDocAnt]/[idDocAntPap]/[idDocAntEle]).
func (b *iniBuilder) docAnt(da *DocAnt) {
	for ei, emi := range da.EmiDocAnt {
		b.section("emiDocAnt" + seq(ei+1))
		b.kvOpt("CNPJCPF", firstNonEmpty(emi.CNPJ, emi.CPF))
		b.kvOpt("IE", emi.IE)
		b.kvOpt("UF", emi.UF)
		b.kvOpt("xNome", emi.XNome)
		for _, idd := range emi.IdDocAnt {
			for pi, pap := range idd.IdDocAntPap {
				b.section("idDocAntPap" + seq(ei+1) + seq(pi+1))
				b.kv("tpDoc", pap.TpDoc)
				b.kv("serie", pap.Serie)
				b.kvOpt("subser", pap.Subser)
				b.kv("nDoc", pap.NDoc)
				b.kvOpt("dEmi", b.dataOpt(pap.DEmi))
			}
			for ki, ele := range idd.IdDocAntEle {
				b.section("idDocAntEle" + seq(ei+1) + seq(ki+1))
				b.kv("chCTe", ele.ChCTe)
			}
		}
	}
}

// cobranca emite [cobr] + [dupnnn].
func (b *iniBuilder) cobranca(c *Cobr) {
	if c == nil {
		return
	}
	b.section("cobr")
	if f := c.Fat; f != nil {
		b.kvOpt("nFat", f.NFat)
		b.kvOpt("vOrig", moneyOpt(f.VOrig))
		b.kvOpt("vDesc", moneyOpt(f.VDesc))
		b.kvOpt("vLiq", moneyOpt(f.VLiq))
	}
	for i, d := range c.Dup {
		b.section("dup" + seq(i+1))
		b.kvOpt("nDup", d.NDup)
		b.kvOpt("dVenc", b.dataOpt(d.DVenc))
		b.kv("vDup", money(d.VDup))
	}
}

// subst emite [infCteSub] (CT-e de substituição).
func (b *iniBuilder) subst(s *InfCteSub) {
	if s == nil {
		return
	}
	b.section("infCteSub")
	b.kv("chCte", s.ChCte)
	b.kvIntOpt("indAlteraToma", s.IndAlteraToma)
}

// modalRodo emite [Rodo] + [occnnn] (modal rodoviário).
func (b *iniBuilder) modalRodo(rodo *Rodo) {
	if rodo == nil {
		return
	}
	b.section("Rodo")
	b.kv("RNTRC", rodo.RNTRC)
	for i, o := range rodo.Occ {
		b.section("occ" + seq(i+1))
		b.kvOpt("serie", o.Serie)
		b.kv("nOcc", strconv.Itoa(o.NOcc))
		b.kvOpt("dEmi", b.dataOpt(o.DEmi))
		if e := o.EmiOcc; e != nil {
			b.kvOpt("CNPJ", e.CNPJ)
			b.kvOpt("cInt", e.CInt)
			b.kvOpt("IE", e.IE)
			b.kvOpt("UF", e.UF)
			b.kvOpt("fone", e.Fone)
		}
	}
}

// respTec emite [infRespTec] (responsável técnico).
func (b *iniBuilder) respTec(rt *RespTec) {
	if rt == nil {
		return
	}
	b.section("infRespTec")
	b.kv("CNPJ", rt.CNPJ)
	b.kv("xContato", rt.XContato)
	b.kv("email", rt.Email)
	b.kv("fone", rt.Fone)
	b.kvIntOpt("idCSRT", rt.IdCSRT)
	b.kvOpt("hashCSRT", rt.HashCSRT)
}

// icms emite a seção da variante de ICMS preenchida (apenas uma deve vir).
func (b *iniBuilder) icms(ic ICMS) {
	switch {
	case ic.ICMS00 != nil:
		b.section("ICMS00")
		b.kv("CST", defaultStr(ic.ICMS00.CST, "00"))
		b.kv("vBC", money(ic.ICMS00.VBC))
		b.kv("pICMS", money(ic.ICMS00.PICMS))
		b.kv("vICMS", money(ic.ICMS00.VICMS))
	case ic.ICMS20 != nil:
		b.section("ICMS20")
		b.kv("CST", defaultStr(ic.ICMS20.CST, "20"))
		b.kv("pRedBC", money(ic.ICMS20.PRedBC))
		b.kv("vBC", money(ic.ICMS20.VBC))
		b.kv("pICMS", money(ic.ICMS20.PICMS))
		b.kv("vICMS", money(ic.ICMS20.VICMS))
		b.kvOpt("vICMSDeson", moneyOpt(ic.ICMS20.VICMSDeson))
		b.kvOpt("cBenef", ic.ICMS20.CBenef)
	case ic.ICMS45 != nil:
		b.section("ICMS45")
		b.kv("CST", defaultStr(ic.ICMS45.CST, "40"))
		b.kvOpt("vICMSDeson", moneyOpt(ic.ICMS45.VICMSDeson))
		b.kvOpt("cBenef", ic.ICMS45.CBenef)
	case ic.ICMS60 != nil:
		b.section("ICMS60")
		b.kv("CST", defaultStr(ic.ICMS60.CST, "60"))
		b.kv("vBCSTRet", money(ic.ICMS60.VBCSTRet))
		b.kv("vICMSSTRet", money(ic.ICMS60.VICMSSTRet))
		b.kv("pICMSSTRet", money(ic.ICMS60.PICMSSTRet))
		b.kvOpt("vCred", moneyOpt(ic.ICMS60.VCred))
	case ic.ICMS90 != nil:
		b.section("ICMS90")
		b.kv("CST", defaultStr(ic.ICMS90.CST, "90"))
		b.kvOpt("pRedBC", moneyOpt(ic.ICMS90.PRedBC))
		b.kv("vBC", money(ic.ICMS90.VBC))
		b.kv("pICMS", money(ic.ICMS90.PICMS))
		b.kv("vICMS", money(ic.ICMS90.VICMS))
		b.kvOpt("vCred", moneyOpt(ic.ICMS90.VCred))
	case ic.ICMSOutraUF != nil:
		b.section("ICMSOutraUF")
		b.kv("CST", defaultStr(ic.ICMSOutraUF.CST, "90"))
		b.kvOpt("pRedBCOutraUF", moneyOpt(ic.ICMSOutraUF.PRedBCOutraUF))
		b.kv("vBCOutraUF", money(ic.ICMSOutraUF.VBCOutraUF))
		b.kv("pICMSOutraUF", money(ic.ICMSOutraUF.PICMSOutraUF))
		b.kv("vICMSOutraUF", money(ic.ICMSOutraUF.VICMSOutraUF))
	case ic.ICMSSN != nil:
		b.section("ICMSSN")
		b.kv("CST", defaultStr(ic.ICMSSN.CST, "90"))
		b.kv("indSN", strconv.Itoa(defaultInt(ic.ICMSSN.IndSN, 1)))
	}
}

// --- builder de INI (local ao pacote cte) -----------------------------------

// iniBuilder acumula o INI. Carrega o fuso do documento porque data-hora só faz
// sentido com ele: o cliente informa um instante (RFC 3339) e o documento fiscal
// precisa desse instante no relógio do emitente.
type iniBuilder struct {
	sb  strings.Builder
	loc *time.Location
}

func (b *iniBuilder) section(name string) {
	if b.sb.Len() > 0 {
		b.sb.WriteByte('\n')
	}
	b.sb.WriteString("[" + name + "]\n")
}

// sanitizeINIVal neutraliza CR/LF no valor (evita injeção de chave/seção INI via
// campo de texto livre do cliente que a ACBrLib consumiria).
func sanitizeINIVal(s string) string { return inifmt.Sanitize(s) }

func (b *iniBuilder) kv(key, val string) { b.sb.WriteString(key + "=" + sanitizeINIVal(val) + "\n") }
func (b *iniBuilder) kvOpt(key, val string) {
	if val != "" {
		b.kv(key, val)
	}
}
func (b *iniBuilder) kvIntOpt(key string, v int) {
	if v != 0 {
		b.kv(key, strconv.Itoa(v))
	}
}

// kvInt emite sempre (inclusive 0). Para campos obrigatórios cujo zero é um valor
// válido: ex.: tpNav=0 (navegação interior), obrigatório no aquaviário v4.00.
func (b *iniBuilder) kvInt(key string, v int) { b.kv(key, strconv.Itoa(v)) }

// pessoa escreve rem/exped/receb/Dest (CNPJCPF único + endereço inline).
func (b *iniBuilder) pessoa(cnpj, cpf, ie, xNome, fone, email string, e Endereco) {
	b.kvOpt("CNPJCPF", firstNonEmpty(cnpj, cpf))
	b.kvOpt("IE", ie)
	b.kvOpt("xNome", xNome)
	b.kvOpt("fone", fone)
	b.kvOpt("email", email)
	b.endereco(e)
}

func (b *iniBuilder) endereco(e Endereco) {
	b.kvOpt("xLgr", e.XLgr)
	b.kvOpt("nro", e.Nro)
	b.kvOpt("xCpl", e.XCpl)
	b.kvOpt("xBairro", e.XBairro)
	b.kvOpt("cMun", e.CMun)
	b.kvOpt("xMun", e.XMun)
	b.kvOpt("CEP", e.CEP)
	b.kvOpt("UF", e.UF)
	b.kvOpt("cPais", e.CPais)
	b.kvOpt("xPais", e.XPais)
}

func (b *iniBuilder) endeEmi(e *EndeEmi) {
	if e == nil {
		return
	}
	b.kvOpt("xLgr", e.XLgr)
	b.kvOpt("nro", e.Nro)
	b.kvOpt("xCpl", e.XCpl)
	b.kvOpt("xBairro", e.XBairro)
	b.kvOpt("cMun", e.CMun)
	b.kvOpt("xMun", e.XMun)
	b.kvOpt("CEP", e.CEP)
	b.kvOpt("UF", e.UF)
	b.kvOpt("fone", e.Fone)
}

// unidades emite infUnidCarga/infUnidTransp (+lacres) de um documento, com o
// prefixo de índice do doc (ex.: "001"). Cargas no nível do doc usam {doc}{c};
// dentro de uma unidade de transporte usam {doc}{transp}{c}.
func (b *iniBuilder) unidades(docIdx string, cargas []UnidCarga, transps []UnidadeTransp) {
	for ci, c := range cargas {
		b.unidCarga(docIdx+seq(ci+1), c)
	}
	for ti, tp := range transps {
		tIdx := docIdx + seq(ti+1)
		b.section("infUnidTransp" + tIdx)
		b.kv("tpUnidTransp", strconv.Itoa(tp.TpUnidTransp))
		b.kv("idUnidTransp", tp.IdUnidTransp)
		b.kvOpt("qtdRat", moneyOpt(tp.QtdRat))
		for li, l := range tp.LacUnidTransp {
			b.section("lacUnidTransp" + tIdx + seq(li+1))
			b.kv("nLacre", l.NLacre)
		}
		for ci, c := range tp.InfUnidCarga {
			b.unidCarga(tIdx+seq(ci+1), c)
		}
	}
}

// gDifDevRed achata gDif/gDevTrib/gRed dentro da seção corrente (gIBSUF/
// gIBSMun/gCBS), conforme o modelo INI da Reforma Tributária.
func (b *iniBuilder) gDifDevRed(dif *Dif, dev *DevTrib, red *Red) {
	if dif != nil {
		b.kvOpt("pDif", moneyOpt(dif.PDif))
		b.kvOpt("vDif", moneyOpt(dif.VDif))
	}
	if dev != nil {
		b.kvOpt("vDevTrib", moneyOpt(dev.VDevTrib))
	}
	if red != nil {
		b.kvOpt("pRedAliq", moneyOpt(red.PRedAliq))
		b.kvOpt("pAliqEfet", moneyOpt(red.PAliqEfet))
	}
}

func (b *iniBuilder) unidCarga(idx string, c UnidCarga) {
	b.section("infUnidCarga" + idx)
	b.kv("tpUnidCarga", strconv.Itoa(c.TpUnidCarga))
	b.kv("idUnidCarga", c.IdUnidCarga)
	b.kvOpt("qtdRat", moneyOpt(c.QtdRat))
	for li, l := range c.LacUnidCarga {
		b.section("lacUnidCarga" + idx + seq(li+1))
		b.kv("nLacre", l.NLacre)
	}
}

func (b *iniBuilder) String() string { return b.sb.String() }

// --- helpers ----------------------------------------------------------------

func seq(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

// dataOpt é a data (sem hora) no fuso do documento; vazio permanece vazio.
func (b *iniBuilder) dataOpt(s string) string {
	if dt := b.dataHoraOpt(s); len(dt) >= 10 {
		return dt[:10]
	}
	return ""
}

// dataHora formata data-hora no fuso do documento. Substituiu um dateTimeBR que
// DESCARTAVA o offset do cliente: "11:00Z" virava "11:00" e a lib carimbava o
// fuso do estado, adiantando o documento em três horas sem erro nenhum.
func (b *iniBuilder) dataHora(s string) string {
	if s == "" {
		return time.Now().In(b.fuso()).Format("02/01/2006 15:04:05")
	}
	return inifmt.DataHoraNoFuso(s, b.fuso())
}

// dataHoraOpt é como dataHora, mas vazio devolve vazio (chave omitida).
func (b *iniBuilder) dataHoraOpt(s string) string {
	if s == "" {
		return ""
	}
	return inifmt.DataHoraNoFuso(s, b.fuso())
}

// fuso devolve o fuso do documento; Brasília quando não definido.
func (b *iniBuilder) fuso() *time.Location {
	if b.loc == nil {
		b.loc = inifmt.LocalDaUF("")
	}
	return b.loc
}

func money(v float64) string    { return inifmt.Money(v) }
func moneyOpt(v float64) string { return inifmt.MoneyOpt(v) }
func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
