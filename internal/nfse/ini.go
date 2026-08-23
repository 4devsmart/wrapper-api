package nfse

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
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

	b.Secao("IdentificacaoNFSe")
	b.KV("TipoXML", "RPS")

	b.Secao("IdentificacaoRps")
	b.KV("Numero", inf.NDPS)
	b.KV("Serie", inf.Serie)
	b.KV("DataEmissao", inifmt.DataBR(inf.DhEmi, b.Local()))
	b.KV("Competencia", inifmt.DataBR(inf.DCompet, b.Local()))
	b.KV("verAplic", cmp.Or(inf.VerAplic, versao.Emissor()))
	b.KV("tpEmit", strconv.Itoa(cmp.Or(inf.TpEmit, 1)))
	// Local de emissão (município emissor). Default = município do prestador.
	b.KV("cLocEmi", cmp.Or(inf.CLocEmi, inf.Prest.CMun))
	// NaturezaOperacao (ABRASF): 1=tributação no município (default). Nacional ignora.
	b.KV("NaturezaOperacao", "1")

	b.Secao("Prestador")
	b.pessoaCommon(inf.Prest)
	if rt := inf.Prest.RegTrib; rt != nil {
		b.KV("opSimpNac", strconv.Itoa(cmp.Or(rt.OpSimpNac, 1)))
		b.KVOpt("RegimeApuracaoSN", optInt(rt.RegApTribSN))
		// regEspTrib é obrigatório no XSD (TCRegTrib): escrever sempre (0 = nenhum).
		b.KV("Regime", strconv.Itoa(rt.RegEspTrib))
	}

	if inf.Toma != nil {
		b.Secao("Tomador")
		b.pessoaCommon(*inf.Toma)
	}

	if inf.Interm != nil {
		b.Secao("Intermediario")
		b.pessoaCommon(*inf.Interm)
	}

	s := inf.Serv
	b.Secao("Servico")
	b.KV("CodigoMunicipio", s.CMunPrestacao)
	b.KV("ItemListaServico", s.CServ)
	b.KV("CodigoTributacaoMunicipio", s.CTribMun)
	b.KV("Discriminacao", s.XDescServ)
	b.KVOpt("CodigoNBS", nbs(s.CNBS))
	b.KVOpt("CodigoCnae", s.CodigoCnae)
	b.KVIntOpt("ExigibilidadeISS", s.ExigibISS)
	b.KVOpt("MunicipioIncidencia", s.MunIncidencia)
	b.KVOpt("xMunicipioIncidencia", s.XMunIncidencia)
	b.KVOpt("NumeroProcesso", s.NumeroProcesso)
	b.KVOpt("CodigoPais", s.CodigoPais)
	b.KVOpt("xPais", s.XPais)

	if c := s.ComExt; c != nil {
		b.Secao("ComercioExterior")
		b.KVOpt("mdPrestacao", c.MdPrestacao)
		b.KVOpt("vincPrest", c.VincPrest)
		b.KVIntOpt("tpMoeda", c.TpMoeda)
		b.KVOpt("vServMoeda", inifmt.MoneyOpt(c.VServMoeda))
		b.KVOpt("mecAFComexP", c.MecAFComexP)
		b.KVOpt("mecAFComexT", c.MecAFComexT)
		b.KVOpt("movTempBens", c.MovTempBens)
		b.KVOpt("nDI", c.NDI)
		b.KVOpt("nRE", c.NRE)
		b.KVIntOpt("mdic", c.Mdic)
	}

	if ic := s.InfoCompl; ic != nil {
		b.Secao("InformacoesComplementares")
		b.KVOpt("idDocTec", ic.IdDocTec)
		b.KVOpt("docRef", ic.DocRef)
		b.KVOpt("xPed", ic.XPed)
		b.KVOpt("xInfComp", ic.XInfComp)
		for i, it := range ic.GItemPed {
			b.Secao("gItemPed" + inifmt.Seq2(i+1))
			b.KV("xItemPed", it)
		}
	}

	v := inf.Valores
	b.Secao("Valores")
	b.KV("ValorServicos", inifmt.Money(v.VServ))
	// IssRetido (ABRASF) é Sim/Não obrigatório; default 2 (não retido).
	b.KV("IssRetido", strconv.Itoa(cmp.Or(v.IssRetido, 2)))
	b.KVOpt("Aliquota", inifmt.MoneyOpt(v.PAliq))
	b.KVOpt("ValorRecebido", inifmt.MoneyOpt(v.VReceb))
	b.KVOpt("DescontoIncondicionado", inifmt.MoneyOpt(v.VDescIncond))
	b.KVOpt("DescontoCondicionado", inifmt.MoneyOpt(v.VDescCond))
	b.KVOpt("ValorDeducoes", inifmt.MoneyOpt(v.VDeducoes))
	b.KVOpt("AliquotaDeducoes", inifmt.MoneyOpt(v.PDeducoes))

	// [tribMun]: usa o grupo detalhado quando informado; senão o atalho simples.
	b.Secao("tribMun")
	if t := v.TribMun; t != nil {
		b.KV("tribISSQN", strconv.Itoa(cmp.Or(t.TribISSQN, 1)))
		b.KVIntOpt("tpRetISSQN", t.TpRetISSQN)
		b.KVOpt("pAliq", inifmt.MoneyOpt(t.PAliq))
		b.KVIntOpt("tpImunidade", t.TpImunidade)
		b.KVIntOpt("tpSusp", t.TpSusp)
		b.KVOpt("nProcesso", t.NProcesso)
		b.KVOpt("vRedBCBM", inifmt.MoneyOpt(t.VRedBCBM))
		b.KVOpt("pRedBCBM", inifmt.MoneyOpt(t.PRedBCBM))
		b.KVOpt("nBM", t.NBM)
	} else {
		b.KV("tribISSQN", strconv.Itoa(cmp.Or(v.TribISSQN, 1)))
		b.KVOpt("pAliq", inifmt.MoneyOpt(v.PAliq))
	}

	if t := v.TribFed; t != nil {
		b.Secao("tribFed")
		b.KVOpt("CST", t.CST)
		b.KVOpt("vBCPisCofins", inifmt.MoneyOpt(t.VBCPisCofins))
		b.KVOpt("pAliqPis", inifmt.MoneyOpt(t.PAliqPis))
		b.KVOpt("pAliqCofins", inifmt.MoneyOpt(t.PAliqCofins))
		b.KVOpt("vPis", inifmt.MoneyOpt(t.VPis))
		b.KVOpt("vCofins", inifmt.MoneyOpt(t.VCofins))
		b.KVIntOpt("tpRetPisCofins", t.TpRetPisCofins)
		b.KVOpt("vRetCP", inifmt.MoneyOpt(t.VRetCP))
		b.KVOpt("vRetIRRF", inifmt.MoneyOpt(t.VRetIRRF))
		b.KVOpt("vRetCSLL", inifmt.MoneyOpt(t.VRetCSLL))
	}

	if t := v.TotTrib; t != nil {
		b.Secao("totTrib")
		b.KVIntOpt("indTotTrib", t.IndTotTrib)
		b.KVOpt("pTotTribSN", inifmt.MoneyOpt(t.PTotTribSN))
		b.KVOpt("vTotTribFed", inifmt.MoneyOpt(t.VTotTribFed))
		b.KVOpt("vTotTribEst", inifmt.MoneyOpt(t.VTotTribEst))
		b.KVOpt("vTotTribMun", inifmt.MoneyOpt(t.VTotTribMun))
		b.KVOpt("pTotTribFed", inifmt.MoneyOpt(t.PTotTribFed))
		b.KVOpt("pTotTribEst", inifmt.MoneyOpt(t.PTotTribEst))
		b.KVOpt("pTotTribMun", inifmt.MoneyOpt(t.PTotTribMun))
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
	b.Secao("IBSCBSDPS")
	// finNFSe e indDest são obrigatórios quando o grupo existe: o leitor do
	// ACBrNFSeX estoura no vazio (diferente de indFinal/tpOper, que aceitam ''):
	// emitimos defaults neutros (0=regular / 0=tomador=adquirente=destinatário).
	b.KV("finNFSe", cmp.Or(g.FinNFSe, "0"))
	b.KV("indDest", cmp.Or(g.IndDest, "0"))
	b.KVOpt("indFinal", g.IndFinal)
	b.KVOpt("cIndOp", g.CIndOp)
	b.KVOpt("tpOper", g.TpOper)
	b.KVOpt("tpEnteGov", g.TpEnteGov)
	if d := g.Dest; d != nil {
		b.Secao("Destinatario")
		b.KVOpt("CNPJCPF", fiscal.Primeiro(d.CNPJ, d.CPF))
		b.KVOpt("NIF", d.NIF)
		b.KVOpt("RazaoSocial", d.XNome)
		b.KVOpt("InscricaoMunicipal", d.IM)
		b.KVOpt("Logradouro", d.Logradouro)
		b.KVOpt("Numero", d.Numero)
		b.KVOpt("Complemento", d.Complemento)
		b.KVOpt("Bairro", d.Bairro)
		b.KVOpt("CodigoMunicipio", d.CMun)
		b.KVOpt("xMunicipio", d.XMun)
		b.KVOpt("UF", d.UF)
		b.KVOpt("CEP", d.CEP)
		b.KVOpt("CodigoPais", d.CPais)
		b.KVOpt("Email", d.Email)
		b.KVOpt("Telefone", d.Telefone)
	}
	if m := g.Imovel; m != nil {
		b.Secao("Imovel")
		b.KVOpt("inscImobFisc", m.InscImobFisc)
		b.KVOpt("cCIB", m.CCIB)
		b.KVOpt("Logradouro", m.Logradouro)
		b.KVOpt("Numero", m.Numero)
		b.KVOpt("Complemento", m.Complemento)
		b.KVOpt("Bairro", m.Bairro)
		b.KVOpt("CEP", m.CEP)
	}
	for i, d := range g.Documentos {
		b.Secao("Documentos" + inifmt.Seq4(i+1))
		// tipoChaveDFe é lido sempre e estoura no vazio (1=NFSe,2=NFe,3=CTe,
		// 4=Outro) → default "4" (Outro) quando o consumidor não informa.
		b.KV("tipoChaveDFe", cmp.Or(d.TipoChaveDFe, "4"))
		b.KVOpt("xTipoChaveDFe", d.XTipoChaveDFe)
		b.KVOpt("chaveDFe", d.ChaveDFe)
		b.KVOpt("cMunDocFiscal", d.CMunDocFiscal)
		b.KVOpt("nDocFiscal", d.NDocFiscal)
		b.KVOpt("xDocFiscal", d.XDocFiscal)
		b.KVOpt("nDoc", d.NDoc)
		b.KVOpt("xDoc", d.XDoc)
		b.KVOpt("CNPJCPF", fiscal.Primeiro(d.FornecCNPJ, d.FornecCPF))
		b.KVOpt("RazaoSocial", d.FornecNome)
		b.KVOpt("dtEmiDoc", inifmt.DataBR(d.DtEmiDoc, b.Local()))
		b.KVOpt("dtCompDoc", inifmt.DataBROpt(d.DtCompDoc, b.Local()))
		b.KVOpt("tpReeRepRes", d.TpReeRepRes)
		b.KVOpt("xTpReeRepRes", d.XTpReeRepRes)
		b.KVOpt("vlrReeRepRes", inifmt.MoneyOpt(d.VlrReeRepRes))
	}
	if c := g.GIBSCBS; c != nil {
		b.Secao("gIBSCBS")
		b.KV("CST", c.CST)
		b.KVOpt("cClassTrib", c.CClassTrib)
		b.KVOpt("cCredPres", c.CCredPres)
		if r := c.GTribRegular; r != nil {
			b.Secao("gTribRegular")
			b.KV("CSTReg", r.CSTReg)
			b.KVOpt("cClassTribReg", r.CClassTribReg)
		}
		if d := c.GDif; d != nil {
			b.Secao("gDif")
			b.KVOpt("pDifUF", inifmt.MoneyOpt(d.PDifUF))
			b.KVOpt("pDifMun", inifmt.MoneyOpt(d.PDifMun))
			b.KVOpt("pDifCBS", inifmt.MoneyOpt(d.PDifCBS))
		}
	}
}

// --- builder de INI ---------------------------------------------------------

// iniBuilder é o construtor compartilhado (internal/platform/inifmt) mais os
// métodos de DOMÍNIO deste documento, logo abaixo. O núcleo (seção, par
// chave=valor, datas no fuso do emitente) vive num lugar só: era o mesmo código
// nos quatro documentos, e um ajuste na sanitização precisava ser feito quatro
// vezes.
type iniBuilder struct{ inifmt.Builder }

// pessoaCommon escreve os campos comuns de prestador/tomador.
//
// O documento vai na chave CNPJCPF: o leitor do ACBrNFSeX lê o Tomador SÓ por
// CNPJCPF (sem fallback para CNPJ): usar só CNPJ fazia o tomador sair como
// cNaoNIF=0. O Prestador aceita ambas (CNPJCPF tem prioridade), então CNPJCPF
// serve para os dois. Mantemos CNPJ/CPF também por compatibilidade.
func (b *iniBuilder) pessoaCommon(p Pessoa) {
	if doc := p.CNPJ; doc != "" {
		b.KV("CNPJCPF", doc)
		b.KV("CNPJ", doc)
	} else if doc := p.CPF; doc != "" {
		b.KV("CNPJCPF", doc)
		b.KV("CPF", doc)
	}
	b.KVOpt("InscricaoMunicipal", p.IM)
	b.KVOpt("RazaoSocial", p.XNome)
	b.KVOpt("Logradouro", p.Logradouro)
	b.KVOpt("Numero", p.Numero)
	b.KVOpt("Complemento", p.Complemento)
	b.KVOpt("Bairro", p.Bairro)
	b.KVOpt("CodigoMunicipio", p.CMun)
	b.KVOpt("UF", p.UF)
	b.KVOpt("CEP", p.CEP)
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
		b.KVIntOpt("CodigoPais", p.CPais) // exterior (ou nacional explícito)
	} else if p.CMun != "" {
		b.KVIntOpt("CodigoPais", codigoPaisBrasil)
	}
	b.KVOpt("xPais", p.XPais)
	b.KVOpt("Telefone", p.Telefone)
	b.KVOpt("Email", p.Email)
}

// --- helpers ----------------------------------------------------------------

// dateBR converte "YYYY-MM-DD" (ou RFC3339) para "DD/MM/YYYY" (formato do INI).
// Vazio vira a data de hoje.

// money formata com 2 casas e separador decimal VÍRGULA: o ACBr lê floats do
// INI no padrão brasileiro (vírgula); "1.00" seria interpretado como 0.

func optInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

// dateBROpt converte para DD/MM/YYYY; vazio continua vazio (sem virar "hoje").
