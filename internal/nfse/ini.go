package nfse

import (
	"strconv"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// codigoPaisBrasil é o código BACEN do Brasil, usado como default do endereço
// nacional. O layout ABRASF 2.04 compara exatamente com este valor para decidir
// entre endereço nacional e do exterior.
const codigoPaisBrasil = 1058

// nbs normaliza o código NBS para só dígitos. O campo tem 9 caracteres no
// layout e a lib TRUNCA no tamanho máximo, então um "1.1103.22.00" formatado
// viraria "1.1103.22" (lixo) em vez do código. Tirar a pontuação aqui evita
// esse corte silencioso.
func nbs(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToINI traduz o pedido (JSON estilo Nuvem Fiscal) para o INI da NFS-e Padrão
// Nacional consumido pela ACBrLibNFSe (NFSE_CarregarINI).
//
// Seções/chaves seguem o modelo oficial do ACBr
// (acbr.sourceforge.io/ACBrLib/ModeloNFSeINI-PadraoNacional.html).
func ToINI(p DPSPedido) string {
	inf := p.InfDPS
	var b iniBuilder

	b.section("IdentificacaoNFSe")
	b.kv("TipoXML", "RPS")

	b.section("IdentificacaoRps")
	b.kv("Numero", inf.NDPS)
	b.kv("Serie", inf.Serie)
	b.kv("DataEmissao", dateBR(inf.DhEmi))
	b.kv("Competencia", dateBR(inf.DCompet))
	b.kv("verAplic", defaultStr(inf.VerAplic, "my-nuvem-fiscal"))
	b.kv("tpEmit", strconv.Itoa(defaultInt(inf.TpEmit, 1)))
	// Local de emissão (município emissor). Default = município do prestador.
	b.kv("cLocEmi", defaultStr(inf.CLocEmi, inf.Prest.CMun))
	// NaturezaOperacao (ABRASF): 1=tributação no município (default). Nacional ignora.
	b.kv("NaturezaOperacao", "1")

	b.section("Prestador")
	b.pessoaCommon(inf.Prest)
	if rt := inf.Prest.RegTrib; rt != nil {
		b.kv("opSimpNac", strconv.Itoa(defaultInt(rt.OpSimpNac, 1)))
		b.kvOpt("RegimeApuracaoSN", optInt(rt.RegApTribSN))
		// regEspTrib é obrigatório no XSD (TCRegTrib): escrever sempre (0 = nenhum).
		b.kv("Regime", strconv.Itoa(rt.RegEspTrib))
	}

	if inf.Toma != nil {
		b.section("Tomador")
		b.pessoaCommon(*inf.Toma)
	}

	if inf.Interm != nil {
		b.section("Intermediario")
		b.pessoaCommon(*inf.Interm)
	}

	s := inf.Serv
	b.section("Servico")
	b.kv("CodigoMunicipio", s.CMunPrestacao)
	b.kv("ItemListaServico", s.CServ)
	b.kv("CodigoTributacaoMunicipio", s.CTribMun)
	b.kv("Discriminacao", s.XDescServ)
	b.kvOpt("CodigoNBS", nbs(s.CNBS))
	b.kvOpt("CodigoCnae", s.CodigoCnae)
	b.kvIntOpt("ExigibilidadeISS", s.ExigibISS)
	b.kvOpt("MunicipioIncidencia", s.MunIncidencia)
	b.kvOpt("xMunicipioIncidencia", s.XMunIncidencia)
	b.kvOpt("NumeroProcesso", s.NumeroProcesso)
	b.kvOpt("CodigoPais", s.CodigoPais)
	b.kvOpt("xPais", s.XPais)

	if c := s.ComExt; c != nil {
		b.section("ComercioExterior")
		b.kvOpt("mdPrestacao", c.MdPrestacao)
		b.kvOpt("vincPrest", c.VincPrest)
		b.kvIntOpt("tpMoeda", c.TpMoeda)
		b.kvOpt("vServMoeda", moneyOpt(c.VServMoeda))
		b.kvOpt("mecAFComexP", c.MecAFComexP)
		b.kvOpt("mecAFComexT", c.MecAFComexT)
		b.kvOpt("movTempBens", c.MovTempBens)
		b.kvOpt("nDI", c.NDI)
		b.kvOpt("nRE", c.NRE)
		b.kvIntOpt("mdic", c.Mdic)
	}

	if ic := s.InfoCompl; ic != nil {
		b.section("InformacoesComplementares")
		b.kvOpt("idDocTec", ic.IdDocTec)
		b.kvOpt("docRef", ic.DocRef)
		b.kvOpt("xPed", ic.XPed)
		b.kvOpt("xInfComp", ic.XInfComp)
		for i, it := range ic.GItemPed {
			b.section("gItemPed" + seq2(i+1))
			b.kv("xItemPed", it)
		}
	}

	v := inf.Valores
	b.section("Valores")
	b.kv("ValorServicos", money(v.VServ))
	// IssRetido (ABRASF) é Sim/Não obrigatório; default 2 (não retido).
	b.kv("IssRetido", strconv.Itoa(defaultInt(v.IssRetido, 2)))
	b.kvOpt("Aliquota", moneyOpt(v.PAliq))
	b.kvOpt("ValorRecebido", moneyOpt(v.VReceb))
	b.kvOpt("DescontoIncondicionado", moneyOpt(v.VDescIncond))
	b.kvOpt("DescontoCondicionado", moneyOpt(v.VDescCond))
	b.kvOpt("ValorDeducoes", moneyOpt(v.VDeducoes))
	b.kvOpt("AliquotaDeducoes", moneyOpt(v.PDeducoes))

	// [tribMun]: usa o grupo detalhado quando informado; senão o atalho simples.
	b.section("tribMun")
	if t := v.TribMun; t != nil {
		b.kv("tribISSQN", strconv.Itoa(defaultInt(t.TribISSQN, 1)))
		b.kvIntOpt("tpRetISSQN", t.TpRetISSQN)
		b.kvOpt("pAliq", moneyOpt(t.PAliq))
		b.kvIntOpt("tpImunidade", t.TpImunidade)
		b.kvIntOpt("tpSusp", t.TpSusp)
		b.kvOpt("nProcesso", t.NProcesso)
		b.kvOpt("vRedBCBM", moneyOpt(t.VRedBCBM))
		b.kvOpt("pRedBCBM", moneyOpt(t.PRedBCBM))
		b.kvOpt("nBM", t.NBM)
	} else {
		b.kv("tribISSQN", strconv.Itoa(defaultInt(v.TribISSQN, 1)))
		b.kvOpt("pAliq", moneyOpt(v.PAliq))
	}

	if t := v.TribFed; t != nil {
		b.section("tribFed")
		b.kvOpt("CST", t.CST)
		b.kvOpt("vBCPisCofins", moneyOpt(t.VBCPisCofins))
		b.kvOpt("pAliqPis", moneyOpt(t.PAliqPis))
		b.kvOpt("pAliqCofins", moneyOpt(t.PAliqCofins))
		b.kvOpt("vPis", moneyOpt(t.VPis))
		b.kvOpt("vCofins", moneyOpt(t.VCofins))
		b.kvIntOpt("tpRetPisCofins", t.TpRetPisCofins)
		b.kvOpt("vRetCP", moneyOpt(t.VRetCP))
		b.kvOpt("vRetIRRF", moneyOpt(t.VRetIRRF))
		b.kvOpt("vRetCSLL", moneyOpt(t.VRetCSLL))
	}

	if t := v.TotTrib; t != nil {
		b.section("totTrib")
		b.kvIntOpt("indTotTrib", t.IndTotTrib)
		b.kvOpt("pTotTribSN", moneyOpt(t.PTotTribSN))
		b.kvOpt("vTotTribFed", moneyOpt(t.VTotTribFed))
		b.kvOpt("vTotTribEst", moneyOpt(t.VTotTribEst))
		b.kvOpt("vTotTribMun", moneyOpt(t.VTotTribMun))
		b.kvOpt("pTotTribFed", moneyOpt(t.PTotTribFed))
		b.kvOpt("pTotTribEst", moneyOpt(t.PTotTribEst))
		b.kvOpt("pTotTribMun", moneyOpt(t.PTotTribMun))
	}

	b.ibscbs(inf.IBSCBS)
	return b.String()
}

// ibscbs emite o grupo da Reforma Tributária: [IBSCBSDPS] + [gIBSCBS] +
// [gTribRegular] + [gDif]. Ver LerINIIBSCBS do ACBrNFSeX.
func (b *iniBuilder) ibscbs(g *IBSCBSDPS) {
	if g == nil {
		return
	}
	b.section("IBSCBSDPS")
	// finNFSe e indDest são obrigatórios quando o grupo existe: o leitor do
	// ACBrNFSeX estoura no vazio (diferente de indFinal/tpOper, que aceitam ''):
	// emitimos defaults neutros (0=regular / 0=tomador=adquirente=destinatário).
	b.kv("finNFSe", defaultStr(g.FinNFSe, "0"))
	b.kv("indDest", defaultStr(g.IndDest, "0"))
	b.kvOpt("indFinal", g.IndFinal)
	b.kvOpt("cIndOp", g.CIndOp)
	b.kvOpt("tpOper", g.TpOper)
	b.kvOpt("tpEnteGov", g.TpEnteGov)
	if d := g.Dest; d != nil {
		b.section("Destinatario")
		b.kvOpt("CNPJCPF", firstNonEmpty(d.CNPJ, d.CPF))
		b.kvOpt("NIF", d.NIF)
		b.kvOpt("RazaoSocial", d.XNome)
		b.kvOpt("InscricaoMunicipal", d.IM)
		b.kvOpt("Logradouro", d.Logradouro)
		b.kvOpt("Numero", d.Numero)
		b.kvOpt("Complemento", d.Complemento)
		b.kvOpt("Bairro", d.Bairro)
		b.kvOpt("CodigoMunicipio", d.CMun)
		b.kvOpt("xMunicipio", d.XMun)
		b.kvOpt("UF", d.UF)
		b.kvOpt("CEP", d.CEP)
		b.kvOpt("CodigoPais", d.CPais)
		b.kvOpt("Email", d.Email)
		b.kvOpt("Telefone", d.Telefone)
	}
	if m := g.Imovel; m != nil {
		b.section("Imovel")
		b.kvOpt("inscImobFisc", m.InscImobFisc)
		b.kvOpt("cCIB", m.CCIB)
		b.kvOpt("Logradouro", m.Logradouro)
		b.kvOpt("Numero", m.Numero)
		b.kvOpt("Complemento", m.Complemento)
		b.kvOpt("Bairro", m.Bairro)
		b.kvOpt("CEP", m.CEP)
	}
	for i, d := range g.Documentos {
		b.section("Documentos" + seq4(i+1))
		// tipoChaveDFe é lido sempre e estoura no vazio (1=NFSe,2=NFe,3=CTe,
		// 4=Outro) → default "4" (Outro) quando o consumidor não informa.
		b.kv("tipoChaveDFe", defaultStr(d.TipoChaveDFe, "4"))
		b.kvOpt("xTipoChaveDFe", d.XTipoChaveDFe)
		b.kvOpt("chaveDFe", d.ChaveDFe)
		b.kvOpt("cMunDocFiscal", d.CMunDocFiscal)
		b.kvOpt("nDocFiscal", d.NDocFiscal)
		b.kvOpt("xDocFiscal", d.XDocFiscal)
		b.kvOpt("nDoc", d.NDoc)
		b.kvOpt("xDoc", d.XDoc)
		b.kvOpt("CNPJCPF", firstNonEmpty(d.FornecCNPJ, d.FornecCPF))
		b.kvOpt("RazaoSocial", d.FornecNome)
		b.kvOpt("dtEmiDoc", dateBR(d.DtEmiDoc))
		b.kvOpt("dtCompDoc", dateBROpt(d.DtCompDoc))
		b.kvOpt("tpReeRepRes", d.TpReeRepRes)
		b.kvOpt("xTpReeRepRes", d.XTpReeRepRes)
		b.kvOpt("vlrReeRepRes", moneyOpt(d.VlrReeRepRes))
	}
	if c := g.GIBSCBS; c != nil {
		b.section("gIBSCBS")
		b.kv("CST", c.CST)
		b.kvOpt("cClassTrib", c.CClassTrib)
		b.kvOpt("cCredPres", c.CCredPres)
		if r := c.GTribRegular; r != nil {
			b.section("gTribRegular")
			b.kv("CSTReg", r.CSTReg)
			b.kvOpt("cClassTribReg", r.CClassTribReg)
		}
		if d := c.GDif; d != nil {
			b.section("gDif")
			b.kvOpt("pDifUF", moneyOpt(d.PDifUF))
			b.kvOpt("pDifMun", moneyOpt(d.PDifMun))
			b.kvOpt("pDifCBS", moneyOpt(d.PDifCBS))
		}
	}
}

// --- builder de INI ---------------------------------------------------------

type iniBuilder struct{ sb strings.Builder }

func (b *iniBuilder) section(name string) {
	if b.sb.Len() > 0 {
		b.sb.WriteByte('\n')
	}
	b.sb.WriteByte('[')
	b.sb.WriteString(name)
	b.sb.WriteString("]\n")
}

// sanitizeINIVal neutraliza CR/LF no valor: o INI é "chave=valor" por linha, então
// um \n vindo de campo de texto livre do cliente injetaria nova chave/seção que a
// ACBrLib consumiria (forjar Ambiente, Emitente, etc.). Trocamos por espaço.
func sanitizeINIVal(s string) string { return inifmt.Sanitize(s) }

// kv escreve sempre a chave (mesmo vazia): útil para campos esperados pelo ACBr.
func (b *iniBuilder) kv(key, val string) {
	b.sb.WriteString(key)
	b.sb.WriteByte('=')
	b.sb.WriteString(sanitizeINIVal(val))
	b.sb.WriteByte('\n')
}

// kvOpt escreve a chave apenas se o valor não for vazio.
func (b *iniBuilder) kvOpt(key, val string) {
	if val != "" {
		b.kv(key, val)
	}
}

// kvIntOpt escreve a chave apenas se o inteiro não for zero.
func (b *iniBuilder) kvIntOpt(key string, v int) {
	if v != 0 {
		b.kv(key, strconv.Itoa(v))
	}
}

// pessoaCommon escreve os campos comuns de prestador/tomador.
//
// O documento vai na chave CNPJCPF: o leitor do ACBrNFSeX lê o Tomador SÓ por
// CNPJCPF (sem fallback para CNPJ): usar só CNPJ fazia o tomador sair como
// cNaoNIF=0. O Prestador aceita ambas (CNPJCPF tem prioridade), então CNPJCPF
// serve para os dois. Mantemos CNPJ/CPF também por compatibilidade.
func (b *iniBuilder) pessoaCommon(p Pessoa) {
	if doc := p.CNPJ; doc != "" {
		b.kv("CNPJCPF", doc)
		b.kv("CNPJ", doc)
	} else if doc := p.CPF; doc != "" {
		b.kv("CNPJCPF", doc)
		b.kv("CPF", doc)
	}
	b.kvOpt("InscricaoMunicipal", p.IM)
	b.kvOpt("RazaoSocial", p.XNome)
	b.kvOpt("Logradouro", p.Logradouro)
	b.kvOpt("Numero", p.Numero)
	b.kvOpt("Complemento", p.Complemento)
	b.kvOpt("Bairro", p.Bairro)
	b.kvOpt("CodigoMunicipio", p.CMun)
	b.kvOpt("UF", p.UF)
	b.kvOpt("CEP", p.CEP)
	// CodigoPais é OBRIGATÓRIO mesmo em endereço nacional. No layout ABRASF 2.04
	// o gravador escolhe nacional × exterior por este campo:
	//
	//   if GerarEnderecoExterior and (Tomador.Endereco.CodigoPais <> 1058) then
	//     GerarEnderecoExteriorTomador   // <EnderecoExterior>
	//
	// e o LerIni assume 0 quando a chave falta (ACBrNFSeX.LerIni.pas). Como
	// 0 <> 1058, omitir a chave fazia TODO endereço nacional virar exterior,
	// colapsando número/bairro/município/UF/CEP em um único
	// <EnderecoCompletoExterior> com o logradouro. Provedores afetados: os de
	// layout 2.04 (ex.: EloTech). O Padrão Nacional decide por CodigoMunicipio e
	// não sofria disso.
	if p.CPais > 0 {
		b.kvIntOpt("CodigoPais", p.CPais) // exterior (ou nacional explícito)
	} else if p.CMun != "" {
		b.kvIntOpt("CodigoPais", codigoPaisBrasil)
	}
	b.kvOpt("xPais", p.XPais)
	b.kvOpt("Telefone", p.Telefone)
	b.kvOpt("Email", p.Email)
}

func (b *iniBuilder) String() string { return b.sb.String() }

// --- helpers ----------------------------------------------------------------

// dateBR converte "YYYY-MM-DD" (ou RFC3339) para "DD/MM/YYYY" (formato do INI).
// Vazio vira a data de hoje.
func dateBR(s string) string {
	if s == "" {
		return time.Now().Format("02/01/2006")
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02/01/2006")
		}
	}
	return s
}

// money formata com 2 casas e separador decimal VÍRGULA: o ACBr lê floats do
// INI no padrão brasileiro (vírgula); "1.00" seria interpretado como 0.
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

func optInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

// seq2 formata um índice com 2 dígitos (ex.: gItemPed01).
func seq2(n int) string { return inifmt.Seq2(n) }

// seq4 formata um índice com 4 dígitos (ex.: Documentos0001).
func seq4(n int) string { return inifmt.Seq4(n) }

// dateBROpt converte para DD/MM/YYYY; vazio continua vazio (sem virar "hoje").
func dateBROpt(s string) string {
	if s == "" {
		return ""
	}
	return dateBR(s)
}

// firstNonEmpty devolve o primeiro valor não vazio.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
