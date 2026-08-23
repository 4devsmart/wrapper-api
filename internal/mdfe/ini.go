package mdfe

import (
	"strconv"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// ToINI traduz o pedido (JSON ACBr.API) para o INI do MDF-e consumido pela
// ACBrLibMDFe (MDFE_CarregarINI). Seções/chaves seguem o modelo oficial
// (acbr.sourceforge.io/ACBrLib/ModeloMDFeINI.html).
//
// Cobertura do rodoviário completo: infANTT (CIOT/valePed/contratante/infPag),
// veicTracao/veicReboque/condutor/lacRodo, descarga (infNFe/infCTe/infMDFeTransp
// com peri + unidCarga/unidTransp aninhados), seg, prodPred/infLotacao, infAdic,
// infRespTec. Pendente: modais aquaviário/ferroviário/aéreo (não-rodoviário).
func ToINI(p PedidoEmissao) string {
	inf := p.InfMDFe
	id := inf.Ide
	var b iniBuilder
	b.loc = inifmt.LocalDoCodigoUF(inf.Ide.CUF)

	b.section("ide")
	b.kvIntOpt("cUF", id.CUF)
	b.kv("tpAmb", fiscal.TpAmb(p.Ambiente))
	b.kv("tpEmit", strconv.Itoa(defaultInt(id.TpEmit, 1)))
	b.kvIntOpt("tpTransp", id.TpTransp)
	b.kv("mod", strconv.Itoa(defaultInt(id.Mod, 58)))
	b.kv("serie", strconv.Itoa(id.Serie))
	b.kv("nMDF", strconv.Itoa(id.NMDF))
	b.kvOpt("cMDF", id.CMDF)
	b.kv("modal", strconv.Itoa(defaultInt(id.Modal, 1))) // 1 = rodoviário
	b.kv("dhemi", b.dataHora(id.DhEmi))
	b.kv("tpEmis", strconv.Itoa(defaultInt(id.TpEmis, 1)))
	b.kv("procEmi", defaultStr(id.ProcEmi, "0"))
	b.kv("verProc", defaultStr(id.VerProc, "my-nuvem-fiscal"))
	b.kv("UFIni", id.UFIni)
	b.kv("UFFim", id.UFFim)
	// dhIniViagem pertence a [ide]: o leitor da lib faz ReadString(<ide>,
	// 'dhIniViagem'). Era escrito dentro de [CARR001] e simplesmente ignorado:
	// o MDF-e saía sem data/hora de início da viagem. Achado pelo lockstep.
	if id.DhIniViagem != "" {
		b.kv("dhIniViagem", b.dataHora(id.DhIniViagem))
	}

	for i, perc := range id.InfPercurso {
		b.section("perc" + seq(i+1))
		b.kv("UFPer", perc.UFPer)
	}
	for i, c := range id.InfMunCarrega {
		b.section("CARR" + seq(i+1))
		b.kv("cMunCarrega", c.CMunCarrega)
		b.kv("xMunCarrega", c.XMunCarrega)
	}

	b.section("emit")
	b.kvOpt("CNPJCPF", firstNonEmpty(inf.Emit.CNPJ, inf.Emit.CPF))
	b.kvOpt("IE", inf.Emit.IE)
	b.kvOpt("xNome", inf.Emit.XNome)
	b.kvOpt("xFant", inf.Emit.XFant)
	if a := inf.Emit.EnderEmit; a != nil {
		b.kvOpt("xLgr", a.XLgr)
		b.kvOpt("nro", a.Nro)
		b.kvOpt("xCpl", a.XCpl)
		b.kvOpt("xBairro", a.XBairro)
		b.kvOpt("cMun", a.CMun)
		b.kvOpt("xMun", a.XMun)
		b.kvOpt("CEP", a.CEP)
		b.kvOpt("UF", a.UF)
		b.kvOpt("fone", a.Fone)
		b.kvOpt("email", a.Email)
	}

	if rodo := inf.InfModal.Rodo; rodo != nil {
		b.section("Rodo")
		b.kvOpt("codAgPorto", rodo.CodAgPorto)
		if antt := rodo.InfANTT; antt != nil {
			b.section("infANTT")
			b.kvOpt("RNTRC", antt.RNTRC)
			for i, c := range antt.InfCIOT {
				b.section("infCIOT" + seq(i+1))
				b.kv("CIOT", c.CIOT)
				b.kvOpt("CNPJCPF", firstNonEmpty(c.CNPJ, c.CPF))
			}
			if vp := antt.ValePed; vp != nil {
				b.section("valePed")
				b.kvOpt("CategCombVeic", vp.CategCombVeic)
				for i, d := range vp.Disp {
					b.section("disp" + seq(i+1))
					b.kvOpt("CNPJForn", d.CNPJForn)
					b.kvOpt("CNPJPg", firstNonEmpty(d.CNPJPg, d.CPFPg))
					b.kvOpt("nCompra", d.NCompra)
					b.kv("vValePed", money(d.VValePed))
					b.kvOpt("tpValePed", d.TpValePed)
				}
			}
			for i, ct := range antt.InfContratante {
				b.section("infContratante" + seq(i+1))
				b.kvOpt("CNPJCPF", firstNonEmpty(ct.CNPJ, ct.CPF))
				b.kvOpt("idEstrangeiro", ct.IdEstrangeiro)
				b.kvOpt("xNome", ct.XNome)
			}
			for pi, pg := range antt.InfPag {
				ps := seq(pi + 1)
				b.section("infPag" + ps)
				b.kvOpt("CNPJCPF", firstNonEmpty(pg.CNPJ, pg.CPF))
				b.kvOpt("idEstrangeiro", pg.IdEstrangeiro)
				b.kvOpt("xNome", pg.XNome)
				b.kv("vContrato", money(pg.VContrato))
				b.kv("indPag", strconv.Itoa(pg.IndPag))
				b.kvOpt("vAdiant", moneyOpt(pg.VAdiant))
				for ci, c := range pg.Comp {
					b.section("Comp" + ps + seq(ci+1))
					b.kv("tpComp", c.TpComp)
					b.kv("vComp", money(c.VComp))
					b.kvOpt("xComp", c.XComp)
				}
				for zi, z := range pg.InfPrazo {
					b.section("infPrazo" + ps + seq(zi+1))
					b.kv("nParcela", strconv.Itoa(z.NParcela))
					b.kvOpt("dVenc", b.dataOpt(z.DVenc))
					b.kv("vParcela", money(z.VParcela))
				}
				if ib := pg.InfBanc; ib.CodBanco != "" || ib.CNPJIPEF != "" || ib.PIX != "" {
					b.section("infBanc" + ps)
					b.kvOpt("codBanco", ib.CodBanco)
					b.kvOpt("codAgencia", ib.CodAgencia)
					b.kvOpt("CNPJIPEF", ib.CNPJIPEF)
					b.kvOpt("PIX", ib.PIX)
				}
			}
		}
		v := rodo.VeicTracao
		b.section("veicTracao")
		b.kvOpt("cInt", v.CInt)
		b.kv("placa", v.Placa)
		b.kvOpt("UF", v.UF)
		b.kvOpt("RENAVAM", v.RENAVAM)
		b.kv("tara", strconv.Itoa(v.Tara))
		b.kvIntOpt("capKG", v.CapKG)
		b.kvIntOpt("capM3", v.CapM3)
		b.kv("tpRod", v.TpRod)
		b.kv("tpCar", v.TpCar)
		if pr := v.Prop; pr != nil {
			b.kvOpt("CNPJCPF", firstNonEmpty(pr.CNPJ, pr.CPF))
			b.kvOpt("RNTRC", pr.RNTRC)
			b.kvOpt("xNome", pr.XNome)
			b.kvOpt("IE", pr.IE)
			b.kvOpt("UFProp", pr.UF)
			b.kvIntOpt("tpProp", pr.TpProp)
		}
		for i, c := range v.Condutor {
			b.section("moto" + seq(i+1))
			b.kv("xNome", c.XNome)
			b.kv("CPF", c.CPF)
		}
		for i, rb := range rodo.VeicReboque {
			b.section("reboque" + seq2(i+1)) // [reboqueNN]: 2 dígitos
			b.kvOpt("cInt", rb.CInt)
			b.kv("placa", rb.Placa)
			b.kvOpt("RENAVAM", rb.RENAVAM)
			b.kv("tara", strconv.Itoa(rb.Tara))
			b.kvIntOpt("capKG", rb.CapKG)
			b.kvIntOpt("capM3", rb.CapM3)
			b.kv("tpCar", rb.TpCar)
			b.kvOpt("UF", rb.UF)
			if pr := rb.Prop; pr != nil {
				b.kvOpt("CNPJCPF", firstNonEmpty(pr.CNPJ, pr.CPF))
				b.kvOpt("RNTRC", pr.RNTRC)
				b.kvOpt("xNome", pr.XNome)
				b.kvOpt("IE", pr.IE)
				b.kvOpt("UFProp", pr.UF)
				b.kvIntOpt("tpProp", pr.TpProp)
			}
		}
		for i, l := range rodo.LacRodo {
			b.section("lacRodo" + seq(i+1))
			b.kv("nLacre", l.NLacre)
		}
	}

	if a := inf.InfModal.Aereo; a != nil {
		b.section("aereo")
		b.kv("nac", a.Nac) // gate do reader
		b.kvOpt("matr", a.Matr)
		b.kvOpt("nVoo", a.NVoo)
		b.kvOpt("cAerEmb", a.CAerEmb)
		b.kvOpt("cAerDes", a.CAerDes)
		b.kvOpt("dVoo", b.dataOpt(a.DVoo))
	}

	if a := inf.InfModal.Aquav; a != nil {
		b.section("aquav")
		b.kv("irin", a.Irin) // gate do reader (CNPJAgeNav ou irin)
		b.kvOpt("tpEmb", a.TpEmb)
		b.kvOpt("cEmbar", a.CEmbar)
		b.kvOpt("xEmbar", a.XEmbar)
		b.kvOpt("nViag", a.NViag)
		b.kvOpt("cPrtEmb", a.CPrtEmb)
		b.kvOpt("cPrtDest", a.CPrtDest)
		b.kvOpt("MMSI", a.MMSI)
		b.kvOpt("prtTrans", a.PrtTrans)
		b.kvIntOpt("tpNav", a.TpNav)
		// Nota: coleções infTermCarreg/Descarreg/infEmbComb/infUnidCargaVazia
		// do aquaviário ficam para uma iteração futura.
	}

	if f := inf.InfModal.Ferrov; f != nil {
		b.section("ferrov")
		b.kv("xPref", f.Trem.XPref) // gate do reader
		b.kvOpt("dhTrem", b.dataOpt(f.Trem.DhTrem))
		b.kvOpt("xOri", f.Trem.XOri)
		b.kvOpt("xDest", f.Trem.XDest)
		b.kvIntOpt("qVag", f.Trem.QVag)
		for i, v := range f.Vag {
			b.section("vag" + seq(i+1))
			b.kv("serie", v.Serie)
			b.kvIntOpt("nVag", v.NVag)
			b.kvIntOpt("nSeq", v.NSeq)
			b.kvOpt("TU", moneyOpt(v.TU))
			b.kvOpt("pesoBC", moneyOpt(v.PesoBC))
			b.kvOpt("pesoR", moneyOpt(v.PesoR))
			b.kvOpt("tpVag", v.TpVag)
		}
	}

	for di, desc := range inf.InfDoc.InfMunDescarga {
		b.section("DESC" + seq(di+1))
		b.kv("cMunDescarga", desc.CMunDescarga)
		b.kv("xMunDescarga", desc.XMunDescarga)
		for ni, nfe := range desc.InfNFe {
			dn := seq(di+1) + seq(ni+1)
			b.section("infNFe" + dn)
			b.kv("chNFe", nfe.ChNFe)
			b.kvOpt("SegCodBarra", nfe.SegCodBarra)
			b.kvIntOpt("indReentrega", nfe.IndReentrega)
			for pi, p := range nfe.Peri {
				b.peri(dn+seq(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, nfe.InfUnidTransp)
		}
		for ci, c := range desc.InfCTe {
			dn := seq(di+1) + seq(ci+1)
			b.section("infCTe" + dn)
			b.kv("chCTe", c.ChCTe)
			b.kvOpt("SegCodBarra", c.SegCodBarra)
			b.kvIntOpt("indReentrega", c.IndReentrega)
			for pi, p := range c.Peri {
				b.peri(dn+seq(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, c.InfUnidTransp)
		}
		for mi, m := range desc.InfMDFeTransp {
			dn := seq(di+1) + seq(mi+1)
			b.section("infMDFeTransp" + dn)
			b.kv("chMDFe", m.ChMDFe)
			b.kvIntOpt("indReentrega", m.IndReentrega)
			for pi, p := range m.Peri {
				b.peri(dn+seq(pi+1), p.NONU, p.XNomeAE, p.XClaRisco, p.GrEmb, p.QTotProd, p.QVolTipo)
			}
			b.unidTransp(dn, m.InfUnidTransp)
		}
	}

	if pp := inf.ProdPred; pp != nil {
		b.section("prodPred")
		b.kvOpt("tpCarga", pp.TpCarga)
		b.kvOpt("xProd", pp.XProd)
		b.kvOpt("cEAN", pp.CEAN)
		b.kvOpt("NCM", pp.NCM)
		if lot := pp.InfLotacao; lot != nil {
			c := lot.InfLocalCarrega
			b.section("infLocalCarrega")
			b.kvOpt("CEP", c.CEP)
			b.kvOpt("latitude", c.Latitude)
			b.kvOpt("longitude", c.Longitude)
			d := lot.InfLocalDescarrega
			b.section("infLocalDescarrega")
			b.kvOpt("CEP", d.CEP)
			b.kvOpt("latitude", d.Latitude)
			b.kvOpt("longitude", d.Longitude)
		}
	}

	t := inf.Tot
	b.section("tot")
	b.kvIntOpt("qCTe", t.QCTe)
	b.kvIntOpt("qNFe", t.QNFe)
	b.kvIntOpt("qMDFe", t.QMDFe)
	b.kv("vCarga", money(t.VCarga))
	b.kv("cUnid", defaultStr(t.CUnid, "01"))
	b.kv("qCarga", money(t.QCarga))

	for i, l := range inf.Lacres {
		b.section("lacres" + seq(i+1))
		b.kv("nLacre", l.NLacre)
	}

	if ia := inf.InfAdic; ia != nil {
		b.section("infAdic")
		b.kvOpt("infAdFisco", ia.InfAdFisco)
		b.kvOpt("infCpl", ia.InfCpl)
	}

	for i, s := range inf.Seg {
		b.section("seg" + seq(i+1))
		b.kv("respSeg", strconv.Itoa(s.InfResp.RespSeg))
		b.kvOpt("CNPJCPF", firstNonEmpty(s.InfResp.CNPJ, s.InfResp.CPF))
		if is := s.InfSeg; is != nil {
			b.kvOpt("xSeg", is.XSeg)
			b.kvOpt("CNPJ", is.CNPJ)
		}
		b.kvOpt("nApol", s.NApol)
		for j, av := range s.NAver {
			b.section("aver" + seq(i+1) + seq(j+1))
			b.kv("nAver", av)
		}
	}

	if rt := inf.InfRespTec; rt != nil {
		b.section("infRespTec")
		b.kv("CNPJ", rt.CNPJ)
		b.kv("xContato", rt.XContato)
		b.kv("email", rt.Email)
		b.kv("fone", rt.Fone)
		b.kvIntOpt("idCSRT", rt.IdCSRT)
		b.kvOpt("hashCSRT", rt.HashCSRT)
	}
	// TODO: modais aquaviário/ferroviário/aéreo, unidCarga/unidTransp + peri,
	// infANTT.infPag (pagamento): indexação aninhada profunda, a confirmar.

	return b.String()
}

// --- builder de INI (local ao pacote mdfe) ----------------------------------

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

// peri emite [periIDX] (produto perigoso) do documento, com o índice já pronto.
func (b *iniBuilder) peri(idx, nONU, xNomeAE, xClaRisco, grEmb, qTotProd, qVolTipo string) {
	b.section("peri" + idx)
	b.kv("nONU", nONU)
	b.kvOpt("xNomeAE", xNomeAE)
	b.kvOpt("xClaRisco", xClaRisco)
	b.kvOpt("grEmb", grEmb)
	b.kv("qTotProd", qTotProd)
	b.kvOpt("qVolTipo", qVolTipo)
}

// unidTransp emite infUnidTransp/infUnidCarga (+lacres) de um documento da
// descarga, com o prefixo {descarga}{doc}.
func (b *iniBuilder) unidTransp(prefix string, transps []UnidadeTransp) {
	for ti, tp := range transps {
		tIdx := prefix + seq(ti+1)
		b.section("infUnidTransp" + tIdx)
		b.kv("tpUnidTransp", strconv.Itoa(tp.TpUnidTransp))
		b.kv("idUnidTransp", tp.IdUnidTransp)
		b.kvOpt("qtdRat", moneyOpt(tp.QtdRat))
		for li, l := range tp.LacUnidTransp {
			b.section("lacUnidTransp" + tIdx + seq(li+1))
			b.kv("nLacre", l.NLacre)
		}
		for ci, c := range tp.InfUnidCarga {
			cIdx := tIdx + seq(ci+1)
			b.section("infUnidCarga" + cIdx)
			b.kv("tpUnidCarga", strconv.Itoa(c.TpUnidCarga))
			b.kv("idUnidCarga", c.IdUnidCarga)
			b.kvOpt("qtdRat", moneyOpt(c.QtdRat))
			for li, l := range c.LacUnidCarga {
				b.section("lacUnidCarga" + cIdx + seq(li+1))
				b.kv("nLacre", l.NLacre)
			}
		}
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

// seq2 formata índice com 2 dígitos (ex.: [reboque01]).
func seq2(n int) string { return inifmt.Seq2(n) }

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

// dataOpt é a data (sem hora) no fuso do documento; vazio permanece vazio.
func (b *iniBuilder) dataOpt(s string) string {
	if dt := b.dataHoraOpt(s); len(dt) >= 10 {
		return dt[:10]
	}
	return ""
}

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
