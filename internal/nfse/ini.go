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

// codigoMunicipioExterior é o CodigoMunicipio que o ABRASF usa para endereço fora
// do Brasil. Não é município brasileiro, então não implica o país 1058.
const codigoMunicipioExterior = "9999999"

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
	b.identificacaoRps(inf)

	b.Secao("Prestador")
	b.pessoaCommon(inf.Prest.Pessoa)
	b.pessoaPrestador(inf.Prest)
	if rt := inf.Prest.RegTrib; rt != nil {
		b.KV("opSimpNac", strconv.Itoa(cmp.Or(rt.OpSimpNac, 1)))
		b.KVOpt("RegimeApuracaoSN", optInt(rt.RegApTribSN))
		// regEspTrib é obrigatório no XSD (TCRegTrib): escrever sempre (0 = nenhum).
		b.KV("Regime", strconv.Itoa(rt.RegEspTrib))
	}

	if inf.Toma != nil {
		b.Secao("Tomador")
		b.pessoaCommon(inf.Toma.Pessoa)
		b.pessoaTomador(*inf.Toma)
	}

	if inf.Interm != nil {
		b.Secao("Intermediario")
		b.pessoaCommon(inf.Interm.Pessoa)
		b.pessoaIntermediario(*inf.Interm)
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
	b.servicoProvedor(s)
	b.complementosServico(s)

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
	b.valoresProvedor(v)

	b.tributacao(v)
	b.ibscbs(inf.IBSCBS)
	b.gruposDoServico(s)
	b.gruposDoDocumento(inf)
	return b.String()
}

// servicoProvedor emite o resto da seção [Servico]: as chaves que a biblioteca
// aceita e que os layouts municipais leem, incluindo o endereço do local da
// prestação, que a lib lê achatado nesta mesma seção.
//
// Vale o mesmo de valoresProvedor: o leitor de INI é um só, e o contrato cobre
// o INI inteiro, não um subconjunto dele.
func (b *iniBuilder) servicoProvedor(s Servico) {
	b.KVOpt("Descricao", s.Descricao)
	b.KVOpt("CodigoNCM", s.CodigoNCM)
	b.KVOpt("CFPS", s.CFPS)
	b.KVOpt("CodigoInterContr", s.CodigoInterContr)
	b.KVOpt("cClassTrib", s.CClassTrib)
	b.KVOpt("INDOP", s.INDOP)
	b.KVOpt("IdentifNaoExigibilidade", s.IdentifNaoExigib)
	b.KVOpt("InfAdicional", s.InfAdicional)
	b.KVOpt("TipoLancamento", s.TipoLancamento)
	b.KVOpt("Operacao", s.Operacao)
	b.KVOpt("Tributacao", s.Tributacao)
	b.KVOpt("LocalPrestacao", s.LocalPrestacao)
	if v := s.PrestadoViasPublic; v != nil {
		// A lib assume true quando a chave falta, então só o pedido explícito diz
		// "não". Vai como inteiro porque o ReadBool do INI é ReadInteger <> 0:
		// "false" seria lido como 0 por acidente, e "true" também.
		b.KV("PrestadoEmViasPublicas", map[bool]string{true: "1", false: "0"}[*v])
	}
	b.KVOpt("MunicipioPrestacaoServico", s.MunPrestacaoServ)
	b.KVOpt("UFPrestacao", s.UFPrestacao)
	b.KVOpt("PercentualCargaTributaria", inifmt.MoneyOpt(s.PercCargaTrib))
	b.KVOpt("ValorCargaTributaria", inifmt.MoneyOpt(s.ValorCargaTrib))
	b.KVOpt("FonteCargaTributaria", s.FonteCargaTrib)
	b.KVOpt("ValorTotalRecebido", inifmt.MoneyOpt(s.ValorTotalRecebido))
	b.KVOpt("xFormaPagamento", s.XFormaPagamento)
	b.KVOpt("xItemListaServico", s.XItemListaServico)
	b.KVOpt("xCodigoTributacaoMunicipio", s.XCodigoTribMun)
	b.KVOpt("xNBS", s.XNBS)
	b.KVOpt("xMunicipio", s.XMunicipio)
	if e := s.EndPrestacao; e != nil {
		b.KVOpt("Logradouro", e.Logradouro)
		b.KVOpt("TipoLogradouro", e.TipoLogradouro)
		b.KVOpt("Numero", e.Numero)
		b.KVOpt("Complemento", e.Complemento)
		b.KVOpt("Bairro", e.Bairro)
		b.KVOpt("CEP", e.CEP)
		b.KVOpt("UF", e.UF)
	}
}

// gruposDoDocumento emite as seções de layout municipal que pendem do documento:
// parcelamento, transportadora, e-mails em cópia, hotelaria, despesas e campos
// livres.
//
// Cada lista tem uma chave ÂNCORA, que a biblioteca usa para saber onde a lista
// acaba (ela para no primeiro índice sem a âncora). Por isso a âncora vai sempre,
// mesmo zerada, e as demais só quando informadas.
func (b *iniBuilder) gruposDoDocumento(inf InfDPS) {
	if c := inf.CondicaoPagamento; c != nil {
		b.Secao("CondicaoPagamento")
		b.KVIntOpt("QtdParcela", c.QtdParcela)
		b.KVOpt("Condicao", c.Condicao)
		b.KVOpt("DataCriacao", inifmt.DataBROpt(c.DataCriacao, b.Local()))
		b.KVOpt("DataVencimento", inifmt.DataBROpt(c.DataVencimento, b.Local()))
		b.KVOpt("InstrucaoPagamento", c.InstrucaoPagamento)
		b.KVOpt("CodigoVencimento", c.CodigoVencimento)
		for i, p := range c.Parcelas {
			b.Secao("Parcelas" + inifmt.Seq2(i+1))
			b.KV("Parcela", p.Parcela) // âncora
			b.KVOpt("DataVencimento", inifmt.DataBROpt(p.DataVencimento, b.Local()))
			b.KVOpt("Valor", inifmt.MoneyOpt(p.Valor))
			b.KVOpt("Condicao", p.Condicao)
		}
	}
	if t := inf.Transportadora; t != nil {
		b.Secao("Transportadora")
		b.KVOpt("xNomeTrans", t.XNome)
		b.KVOpt("xCpfCnpjTrans", t.CpfCnpj)
		b.KVOpt("xInscEstTrans", t.InscEstuary)
		b.KVOpt("xPlacaTrans", t.Placa)
		b.KVOpt("xEndTrans", t.Endereco)
		b.KVOpt("cMunTrans", t.CMun)
		b.KVOpt("xMunTrans", t.XMun)
		b.KVOpt("xUFTrans", t.UF)
		b.KVOpt("xPaisTrans", t.XPais)
		b.KVOpt("vTipoFreteTrans", inifmt.MoneyOpt(t.TipoFrete))
	}
	for i, e := range inf.Emails {
		b.Secao("Email" + strconv.Itoa(i+1))
		b.KV("emailCC", e) // âncora
	}
	for i, q := range inf.Quartos {
		b.Secao("Quartos" + inifmt.Seq3(i+1))
		b.KVInt("CodigoInternoQuarto", q.CodigoInterno) // âncora
		b.KVIntOpt("QtdHospedes", q.QtdHospedes)
		b.KVOpt("CheckIn", inifmt.DataBROpt(q.CheckIn, b.Local()))
		b.KVIntOpt("QtdDiarias", q.QtdDiarias)
		b.KVOpt("ValorDiaria", inifmt.MoneyOpt(q.ValorDiaria))
	}
	for i, d := range inf.Despesas {
		b.Secao("Despesas" + inifmt.Seq3(i+1))
		b.KV("vDesp", inifmt.Money(d.VDesp)) // âncora
		b.KVOpt("nItemDesp", d.NItem)
		b.KVOpt("xDesp", d.XDesp)
		b.KVOpt("dDesp", inifmt.DataBROpt(d.DDesp, b.Local()))
	}
	for i, g := range inf.Genericos {
		b.Secao("Genericos" + strconv.Itoa(i+1))
		b.KV("Titulo", g.Titulo) // âncora
		b.KVOpt("Descricao", g.Descricao)
	}
}

// gruposDoServico emite as seções de layout municipal que pendem do serviço:
// deduções, quebra de impostos, locação de postes e exploração de rodovia.
func (b *iniBuilder) gruposDoServico(s Servico) {
	for i, d := range s.Deducoes {
		b.Secao("Deducoes" + inifmt.Seq3(i+1))
		b.KV("ValorDeduzir", inifmt.Money(d.ValorDeduzir)) // âncora
		b.KVOpt("TipoDeducao", d.TipoDeducao)
		b.KVOpt("CNPJCPF", d.CpfCnpjReferencia)
		b.KVOpt("NumeroNFReferencia", d.NumeroNFReferencia)
		b.KVOpt("ValorTotalReferencia", inifmt.MoneyOpt(d.ValorTotalReferencia))
		b.KVOpt("PercentualDeduzir", inifmt.MoneyOpt(d.PercentualDeduzir))
		b.KVOpt("DeducaoPor", d.DeducaoPor)
	}
	for i, im := range s.Impostos {
		b.Secao("Impostos" + inifmt.Seq3(i+1))
		b.KV("Valor", inifmt.Money(im.Valor)) // âncora
		b.KVIntOpt("Codigo", im.Codigo)
		b.KVOpt("Descricao", im.Descricao)
		b.KVOpt("Aliquota", inifmt.MoneyOpt(im.Aliquota))
	}
	if l := s.Locacao; l != nil {
		b.Secao("LocacaoSubLocacao")
		b.KVOpt("categ", l.Categ)
		b.KVOpt("objeto", l.Objeto)
		b.KVOpt("extensao", inifmt.MoneyOpt(l.Extensao))
		b.KVIntOpt("nPostes", l.NPostes)
	}
	for i, d := range s.DocsDeducao {
		idx := inifmt.Seq4(i + 1)
		b.Secao("DocumentosDeducaoReducao" + idx)
		b.KV("dtEmiDoc", inifmt.DataBR(d.DtEmiDoc, b.Local())) // âncora
		b.KVOpt("tpDedRed", d.TpDedRed)
		b.KVOpt("xDescOutDed", d.XDescOutDed)
		b.KVOpt("vDedutivelRedutivel", inifmt.MoneyOpt(d.VDedutivelRedutivel))
		b.KVOpt("vDeducaoReducao", inifmt.MoneyOpt(d.VDeducaoReducao))
		b.KVOpt("chNFSe", d.ChNFSe)
		b.KVOpt("chNFe", d.ChNFe)
		b.KVOpt("nDocFisc", d.NDocFisc)
		b.KVOpt("nDoc", d.NDoc)
		b.KVOpt("cMunNFSeMun", d.CMunNFSeMun)
		b.KVOpt("nNFSeMun", d.NNFSeMun)
		b.KVOpt("cVerifNFSeMun", d.CVerifNFSeMun)
		b.KVOpt("nNFS", d.NNFS)
		b.KVOpt("modNFS", d.ModNFS)
		b.KVOpt("serieNFS", d.SerieNFS)
		if f := d.Fornecedor; f != nil {
			// O índice do fornecedor é o MESMO do documento: a lib lê os dois
			// juntos, no mesmo passo do laço.
			b.Secao("Fornecedor" + idx)
			b.KVOpt("CNPJCPF", f.CNPJCPF)
			b.KVOpt("InscricaoMunicipal", f.InscricaoMunicipal)
			b.KVOpt("NIF", f.NIF)
			b.KVOpt("cNaoNIF", f.CNaoNIF)
			b.KVOpt("CAEPF", f.CAEPF)
			b.KVOpt("RazaoSocial", f.RazaoSocial)
			b.KVOpt("Logradouro", f.Logradouro)
			b.KVOpt("Numero", f.Numero)
			b.KVOpt("Complemento", f.Complemento)
			b.KVOpt("Bairro", f.Bairro)
			b.KVOpt("CEP", f.CEP)
			b.KVOpt("xMunicipio", f.XMunicipio)
			b.KVOpt("UF", f.UF)
			b.KVOpt("Telefone", f.Telefone)
			b.KVOpt("Email", f.Email)
		}
	}
	if o := s.ConstrucaoCivil; o != nil {
		// O endereço da obra vai achatado nesta mesma seção, e não numa própria.
		b.Secao("ConstrucaoCivil")
		b.KVOpt("CodigoObra", o.CodigoObra)
		b.KVOpt("inscImobFisc", o.InscImobFisc)
		b.KVIntOpt("Cib", o.Cib)
		b.KVOpt("Art", o.Art)
		b.KVIntOpt("ObrasOpcao", o.ObrasOpcao)
		b.KVOpt("LocalConstrucao", o.LocalConstrucao)
		b.KVIntOpt("ReformaCivil", o.ReformaCivil)
		b.KVOpt("nCei", o.NCei)
		b.KVOpt("nProj", o.NProj)
		b.KVOpt("nMatri", o.NMatri)
		b.KVOpt("nNumeroEncapsulamento", o.NNumeroEncapsulamento)
		b.KVOpt("Logradouro", o.Logradouro)
		b.KVOpt("Numero", o.Numero)
		b.KVOpt("Complemento", o.Complemento)
		b.KVOpt("Bairro", o.Bairro)
		b.KVOpt("CEP", o.CEP)
		b.KVOpt("CodigoMunicipio", o.CodigoMunicipio)
		b.KVOpt("xMunicipio", o.XMunicipio)
		b.KVOpt("UF", o.UF)
	}
	for i, it := range s.Itens {
		idx := inifmt.Seq3(i + 1)
		b.Secao("Itens" + idx)
		b.KV("Descricao", it.Descricao) // âncora: a lib para no primeiro índice sem ela
		b.KVOpt("ItemListaServico", it.ItemListaServico)
		b.KVOpt("xItemListaServico", it.XItemListaServico)
		b.KVOpt("CodServico", it.CodServico)
		b.KVOpt("codLCServico", it.CodLCServico)
		// A lib lê CodigoCnae com idCnae de segunda opção: mandamos os dois.
		b.KVOpt("CodigoCnae", it.CodigoCnae)
		b.KVOpt("idCnae", it.CodigoCnae)
		b.KVOpt("CodigoTributacaoMunicipio", it.CodigoTributacaoMunicipio)
		b.KVOpt("xCodigoTributacaoMunicipio", it.XCodigoTribMun)
		b.KVOpt("CodigoServicoNacional", it.CodigoServicoNacional)
		b.KVOpt("CodigoTributacaoNacional", it.CodigoTributacaoNacional)
		b.KVOpt("CodigoNBS", it.CodigoNBS)
		b.KVOpt("xNBS", it.XNBS)
		b.KVOpt("CodigoInterContr", it.CodigoInterContr)
		b.KVOpt("CodCNO", it.CodCNO)
		b.KVOpt("CFPS", it.CFPS)
		b.KVOpt("cClassTrib", it.CClassTrib)
		b.KVOpt("INDOP", it.INDOP)
		b.KVOpt("InfAdicional", it.InfAdicional)
		b.KVOpt("NumeroProcesso", it.NumeroProcesso)
		b.KVOpt("IdentifNaoExigibilidade", it.IdentifNaoExigib)
		b.KVOpt("TipoLancamento", it.TipoLancamento)
		b.KVOpt("Operacao", it.Operacao)
		b.KVOpt("Tributacao", it.Tributacao)
		b.KVOpt("LocalPrestacao", it.LocalPrestacao)
		b.KVOpt("xFormaPagamento", it.XFormaPagamento)
		b.KVOpt("FonteCargaTributaria", it.FonteCargaTrib)
		b.KVOpt("xJustDeducao", it.XJustDeducao)
		b.KVOpt("DescricaoOutrasRetencoes", it.DescricaoOutrasRet)
		b.KVOpt("Unidade", it.Unidade)
		b.KVOpt("TipoUnidade", it.TipoUnidade)
		b.KVOpt("xMunicipioIncidencia", it.XMunicipioIncidencia)
		// Mesma dupla de nomes para o município da prestação do item.
		b.KVOpt("CodigoMunicipio", it.CodigoMunicipio)
		b.KVOpt("CodMunPrestacao", it.CodigoMunicipio)
		b.KVIntOpt("MunicipioIncidencia", it.MunicipioIncidencia)
		b.KVIntOpt("CodigoPais", it.CodigoPais)
		b.KVIntOpt("ExigibilidadeISS", it.ExigibilidadeISS)
		b.KVIntOpt("ResponsavelRetencao", it.RespRetencao)
		b.KVIntOpt("SituacaoTributaria", it.SituacaoTributaria)
		b.KVIntOpt("ISSRetido", it.ISSRetido)
		b.KVIntOpt("Tributavel", it.Tributavel)
		b.KVIntOpt("TribMunPrestador", it.TribMunPrestador)
		if v := it.PrestadoViasPublic; v != nil {
			b.KV("PrestadoEmViasPublicas", map[bool]string{true: "1", false: "0"}[*v])
		}
		b.KVOpt("Quantidade", inifmt.MoneyOpt(it.Quantidade))
		b.KVOpt("ValorUnitario", inifmt.MoneyOpt(it.ValorUnitario))
		// ValorTotal tem ValorServicos de segunda opção, e ValorRecebido tem
		// ValorTotalRecebido: os dois nomes vão, como o par CNPJCPF/CNPJ.
		b.KVOpt("ValorTotal", inifmt.MoneyOpt(it.ValorTotal))
		b.KVOpt("ValorServicos", inifmt.MoneyOpt(cmp.Or(it.ValorServicos, it.ValorTotal)))
		b.KVOpt("ValorRecebido", inifmt.MoneyOpt(it.ValorRecebido))
		b.KVOpt("ValorTotalRecebido", inifmt.MoneyOpt(it.ValorRecebido))
		b.KVOpt("QtdeDiaria", inifmt.MoneyOpt(it.QtdeDiaria))
		b.KVOpt("ValorTaxaTurismo", inifmt.MoneyOpt(it.ValorTaxaTurismo))
		b.KVOpt("AliquotaDeducoes", inifmt.MoneyOpt(it.AliquotaDeducoes))
		b.KVOpt("ValorDeducoes", inifmt.MoneyOpt(it.ValorDeducoes))
		b.KVOpt("AliqReducao", inifmt.MoneyOpt(it.AliqReducao))
		b.KVOpt("ValorReducao", inifmt.MoneyOpt(it.ValorReducao))
		b.KVOpt("Aliquota", inifmt.MoneyOpt(it.Aliquota))
		b.KVOpt("AliquotaSN", inifmt.MoneyOpt(it.AliquotaSN))
		b.KVOpt("BaseCalculo", inifmt.MoneyOpt(it.BaseCalculo))
		b.KVOpt("ValorISS", inifmt.MoneyOpt(it.ValorISS))
		b.KVOpt("ValorISSRetido", inifmt.MoneyOpt(it.ValorISSRetido))
		b.KVOpt("AliqISSST", inifmt.MoneyOpt(it.AliqISSST))
		b.KVOpt("ValorISSST", inifmt.MoneyOpt(it.ValorISSST))
		b.KVOpt("ValorTributavel", inifmt.MoneyOpt(it.ValorTributavel))
		b.KVOpt("DescontoIncondicionado", inifmt.MoneyOpt(it.DescontoIncondicionado))
		b.KVOpt("DescontoCondicionado", inifmt.MoneyOpt(it.DescontoCondicionado))
		b.KVOpt("OutrosDescontos", inifmt.MoneyOpt(it.OutrosDescontos))
		b.KVOpt("OutrasRetencoes", inifmt.MoneyOpt(it.OutrasRetencoes))
		b.KVOpt("ValorOutrasRetencoes", inifmt.MoneyOpt(it.OutrasRetencoes))
		b.KVOpt("RetencoesFederais", inifmt.MoneyOpt(it.RetencoesFederais))
		b.KVOpt("IrrfIndenizacao", inifmt.MoneyOpt(it.IrrfIndenizacao))
		b.KVOpt("ValorBCCSLL", inifmt.MoneyOpt(it.ValorBCCSLL))
		b.KVOpt("AliqRetCSLL", inifmt.MoneyOpt(it.AliqRetCSLL))
		b.KVIntOpt("RetidoCSLL", it.RetidoCSLL)
		b.KVOpt("ValorCSLL", inifmt.MoneyOpt(it.ValorCSLL))
		b.KVOpt("ValorBCPIS", inifmt.MoneyOpt(it.ValorBCPIS))
		b.KVOpt("AliqRetPIS", inifmt.MoneyOpt(it.AliqRetPIS))
		b.KVIntOpt("RetidoPIS", it.RetidoPIS)
		b.KVOpt("ValorPIS", inifmt.MoneyOpt(it.ValorPIS))
		b.KVOpt("ValorBCCOFINS", inifmt.MoneyOpt(it.ValorBCCOFINS))
		b.KVOpt("AliqRetCOFINS", inifmt.MoneyOpt(it.AliqRetCOFINS))
		b.KVIntOpt("RetidoCOFINS", it.RetidoCOFINS)
		b.KVOpt("ValorCOFINS", inifmt.MoneyOpt(it.ValorCOFINS))
		b.KVOpt("ValorBCINSS", inifmt.MoneyOpt(it.ValorBCINSS))
		b.KVOpt("AliqRetINSS", inifmt.MoneyOpt(it.AliqRetINSS))
		b.KVIntOpt("RetidoINSS", it.RetidoINSS)
		b.KVOpt("ValorINSS", inifmt.MoneyOpt(it.ValorINSS))
		b.KVOpt("ValorBCRetIRRF", inifmt.MoneyOpt(it.ValorBCRetIRRF))
		b.KVOpt("AliqRetIRRF", inifmt.MoneyOpt(it.AliqRetIRRF))
		b.KVIntOpt("RetidoIRRF", it.RetidoIRRF)
		b.KVOpt("ValorIRRF", inifmt.MoneyOpt(it.ValorIRRF))
		b.KVOpt("ValorBCCPP", inifmt.MoneyOpt(it.ValorBCCPP))
		b.KVOpt("AliqRetCPP", inifmt.MoneyOpt(it.AliqRetCPP))
		b.KVIntOpt("RetidoCPP", it.RetidoCPP)
		b.KVOpt("ValorCPP", inifmt.MoneyOpt(it.ValorCPP))
		b.KVOpt("ValorIPI", inifmt.MoneyOpt(it.ValorIPI))
		b.KVOpt("ValorLiquidoNfse", inifmt.MoneyOpt(it.ValorLiquidoNfse))
		b.KVOpt("ValorRepasse", inifmt.MoneyOpt(it.ValorRepasse))
		b.KVOpt("ValorInicialCobrado", inifmt.MoneyOpt(it.ValorInicialCobrado))
		b.KVOpt("ValorFinalCobrado", inifmt.MoneyOpt(it.ValorFinalCobrado))
		b.KVOpt("PercentualCargaTributaria", inifmt.MoneyOpt(it.PercCargaTrib))
		b.KVOpt("ValorCargaTributaria", inifmt.MoneyOpt(it.ValorCargaTrib))
		b.KVOpt("TotalAproxTribServ", inifmt.MoneyOpt(it.TotalAproxTribServ))
		b.KVOpt("AliqIBS", inifmt.MoneyOpt(it.AliqIBS))
		b.KVOpt("AliqCBS", inifmt.MoneyOpt(it.AliqCBS))
		b.enderecoServico(it.EnderecoServico, i+1)
	}
	if e := s.Evento; e != nil {
		b.Secao("Evento")
		// RazaoSocial é a grafia principal e xNome é o fallback que o leitor
		// aceita. Vão as duas, como nos outros pares do INI.
		b.KVOpt("RazaoSocial", e.XNome)
		b.KVOpt("xNome", e.XNome)
		b.KVOpt("dtIni", inifmt.DataBROpt(e.DtIni, b.Local()))
		b.KVOpt("dtFim", inifmt.DataBROpt(e.DtFim, b.Local()))
		b.KVOpt("idAtvEvt", e.IdAtvEvt)
		b.KVIntOpt("AtividadeEventoOpcao", e.Opcao)
		b.KVOpt("CEP", e.CEP)
		b.KVOpt("xMunicipio", e.XMunicipio)
		b.KVOpt("CodigoMunicipio", e.CMun)
		b.KVOpt("UF", e.UF)
		b.KVOpt("Logradouro", e.Logradouro)
		b.KVOpt("Numero", e.Numero)
		b.KVOpt("Complemento", e.Complemento)
		b.KVOpt("Bairro", e.Bairro)
	}
	if r := s.Rodoviaria; r != nil {
		b.Secao("Rodoviaria")
		b.KVOpt("categVeic", r.CategVeic)
		b.KVIntOpt("nEixos", r.NEixos)
		b.KVOpt("rodagem", r.Rodagem)
		b.KVOpt("sentido", r.Sentido)
		b.KVOpt("placa", r.Placa)
		b.KVOpt("codAcessoPed", r.CodAcessoPed)
		b.KVOpt("codContrato", r.CodContrato)
	}
	// Sem lista de itens, o endereço de execução do serviço é o do item 1, que é
	// o que o construtor deriva em [Itens001].
	if len(s.Itens) == 0 {
		b.enderecoServico(s.EnderecoServico, 1)
	}
}

// enderecoServico emite [EnderecoServicoNNN], o endereço de execução do item de
// mesmo índice. A seção não tem âncora: a lib a lê por SectionExists.
func (b *iniBuilder) enderecoServico(e *EnderecoServico, indice int) {
	if e != nil {
		b.Secao("EnderecoServico" + inifmt.Seq3(indice))
		b.KVOpt("EnderecoInformado", e.EnderecoInformado)
		b.KVOpt("TipoLogradouro", e.TipoLogradouro)
		b.KVOpt("Endereco", e.Endereco)
		b.KVOpt("Numero", e.Numero)
		b.KVOpt("Complemento", e.Complemento)
		b.KVOpt("TipoBairro", e.TipoBairro)
		b.KVOpt("Bairro", e.Bairro)
		b.KVOpt("CodigoMunicipio", e.CodigoMunicipio)
		b.KVOpt("UF", e.UF)
		b.KVOpt("CEP", e.CEP)
		b.KVOpt("xMunicipio", e.XMunicipio)
		b.KVIntOpt("CodigoPais", e.CodigoPais)
		b.KVOpt("xPais", e.XPais)
		b.KVOpt("PontoReferencia", e.PontoReferencia)
	}
}

// complementosServico emite [ComercioExterior] e [InformacoesComplementares].
// Vai nos dois builders: o leitor de INI é um só, e há gravador fora do Padrão
// Nacional que leva o comércio exterior ao XML (o GISS escreve <comExt>).
func (b *iniBuilder) complementosServico(s Servico) {
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
}

// identificacaoRps emite o resto da seção [IdentificacaoRps]: as chaves que a
// biblioteca aceita e que os layouts municipais leem (datas do RPS, espécie e
// série do documento, recolhimento, parcelas e carga tributária).
//
// Vai nos dois builders, como valoresProvedor e servicoProvedor: o leitor de INI
// é um só, e o contrato cobre o INI inteiro.
func (b *iniBuilder) identificacaoRps(inf InfDPS) {
	b.KVOpt("DataEmissaoRPS", inifmt.DataBROpt(inf.DataEmissaoRPS, b.Local()))
	b.KVOpt("DataFatoGerador", inifmt.DataBROpt(inf.DataFatoGerador, b.Local()))
	b.KVOpt("DataPagamento", inifmt.DataBROpt(inf.DataPagamento, b.Local()))
	b.KVOpt("Vencimento", inifmt.DataBROpt(inf.Vencimento, b.Local()))
	b.KVOpt("dhRecebimento", b.DataHoraOpt(inf.DhRecebimento))
	b.KVOpt("TipoRecolhimento", inf.TipoRecolhimento)
	b.KVOpt("TipoTributacaoRPS", inf.TipoTributacaoRPS)
	b.KVOpt("SituacaoTrib", inf.SituacaoTrib)
	b.KVIntOpt("Situacao", inf.Situacao)
	b.KVIntOpt("TipoNota", inf.TipoNota)
	b.KVIntOpt("NumeroParcelas", inf.NumeroParcelas)
	b.KVOpt("FormaPagamento", inf.FormaPagamento)
	b.KVOpt("EspecieDocumento", inf.EspecieDocumento)
	b.KVOpt("SerieTalonario", inf.SerieTalonario)
	b.KVOpt("SeriePrestacao", inf.SeriePrestacao)
	b.KVOpt("SiglaUF", inf.SiglaUF)
	b.KVOpt("OutrasInformacoes", inf.OutrasInformacoes)
	b.KVOpt("InformacoesComplementares", inf.InformacoesComplementares)
	b.KVOpt("IdentificacaoRemessa", inf.IdentificacaoRemessa)
	b.KVOpt("EqptoRecibo", inf.EqptoRecibo)
	b.KVOpt("RegRec", inf.RegRec)
	b.KVOpt("FrmRec", inf.FrmRec)
	b.KVIntOpt("Producao", inf.Producao)
	b.KVIntOpt("DeducaoMateriais", inf.DeducaoMateriais)
	b.KVOpt("cMotivoEmisTI", inf.CMotivoEmisTI)
	b.KVIntOpt("id_sis_legado", inf.IDSisLegado)
	b.KVOpt("PercentualCargaTributariaMunicipal", inifmt.MoneyOpt(inf.PercCargaTribMunicipal))
	b.KVOpt("ValorCargaTributariaMunicipal", inifmt.MoneyOpt(inf.ValorCargaTribMunicipal))
	b.KVOpt("PercentualCargaTributariaEstadual", inifmt.MoneyOpt(inf.PercCargaTribEstadual))
	b.KVOpt("ValorCargaTributariaEstadual", inifmt.MoneyOpt(inf.ValorCargaTribEstadual))
}

// valoresProvedor emite o resto da seção [Valores]: as chaves que a biblioteca
// aceita e que os layouts municipais leem, mas que não existem no Padrão
// Nacional (base do ISS, alíquotas e retenções por tributo, descontos e totais).
//
// Vai nos dois builders porque o leitor de INI é um só, e o contrato deste
// serviço cobre o INI, não um subconjunto dele: quem manda o campo espera vê-lo
// no documento, e só o gravador do município decide se ele sai.
func (b *iniBuilder) valoresProvedor(v Valores) {
	b.KVOpt("BaseCalculo", inifmt.MoneyOpt(v.BaseCalculo))
	b.KVOpt("ValorIss", inifmt.MoneyOpt(v.ValorIss))
	b.KVOpt("ValorIssRetido", inifmt.MoneyOpt(v.ValorIssRetido))
	b.KVOpt("AliquotaSN", inifmt.MoneyOpt(v.AliquotaSN))
	b.KVOpt("BaseCalculoPISCOFINS", inifmt.MoneyOpt(v.BaseCalculoPISCOFINS))
	b.KVOpt("AliquotaPIS", inifmt.MoneyOpt(v.AliquotaPIS))
	b.KVOpt("AliquotaCofins", inifmt.MoneyOpt(v.AliquotaCofins))
	b.KVOpt("AliquotaINSS", inifmt.MoneyOpt(v.AliquotaINSS))
	b.KVOpt("AliquotaIR", inifmt.MoneyOpt(v.AliquotaIR))
	b.KVOpt("AliquotaCSLL", inifmt.MoneyOpt(v.AliquotaCSLL))
	b.KVOpt("AliquotaCPP", inifmt.MoneyOpt(v.AliquotaCPP))
	b.KVOpt("ValorCPP", inifmt.MoneyOpt(v.ValorCPP))
	b.KVOpt("ValorIPI", inifmt.MoneyOpt(v.ValorIPI))
	b.KVIntOpt("RetidoPIS", v.RetidoPIS)
	b.KVIntOpt("RetidoCOFINS", v.RetidoCofins)
	b.KVIntOpt("RetidoINSS", v.RetidoINSS)
	b.KVIntOpt("RetidoIR", v.RetidoIR)
	b.KVIntOpt("RetidoCSLL", v.RetidoCSLL)
	b.KVIntOpt("RetidoCPP", v.RetidoCPP)
	b.KVOpt("RetencoesFederais", inifmt.MoneyOpt(v.RetencoesFederais))
	b.KVOpt("IrrfIndenizacao", inifmt.MoneyOpt(v.IrrfIndenizacao))
	// A lib lê ValorOutrasRetencoes e, na falta dele, OutrasRetencoes: mandamos
	// os dois, como em CNPJCPF/CNPJ, porque há gravador que espera cada nome.
	b.KVOpt("ValorOutrasRetencoes", inifmt.MoneyOpt(v.OutrasRetencoes))
	b.KVOpt("OutrasRetencoes", inifmt.MoneyOpt(v.OutrasRetencoes))
	b.KVOpt("DescricaoOutrasRetencoes", v.DescricaoOutrasRet)
	b.KVOpt("OutrosDescontos", inifmt.MoneyOpt(v.OutrosDescontos))
	b.KVOpt("JustificativaDeducao", v.JustificativaDeducao)
	b.KVOpt("ValorRepasse", inifmt.MoneyOpt(v.ValorRepasse))
	b.KVOpt("ValorInicialCobrado", inifmt.MoneyOpt(v.ValorInicialCobrado))
	b.KVOpt("ValorFinalCobrado", inifmt.MoneyOpt(v.ValorFinalCobrado))
	b.KVOpt("ValorLiquidoNfse", inifmt.MoneyOpt(v.ValorLiquidoNfse))
	b.KVOpt("ValorTotalNotaFiscal", inifmt.MoneyOpt(v.ValorTotalNotaFiscal))
	b.KVOpt("ValorTotalRecebido", inifmt.MoneyOpt(v.ValorTotalRecebido))
	b.KVOpt("ValorTotalTributos", inifmt.MoneyOpt(v.ValorTotalTributos))
	b.KVOpt("TotalAproxTrib", inifmt.MoneyOpt(v.TotalAproxTrib))
	b.KVOpt("dsImpostos", v.DsImpostos)
}

// tributacao emite [tribMun], [tribFederal] e [totTrib]. Vai nos dois builders:
// o leitor de INI é um só, e fora do Padrão Nacional há gravador que leva esses
// grupos ao XML (o GISS 2.04 escreve <trib> quando recebe o CST federal).
//
// Ao ler [tribFederal] a lib também sobrescreve Valores.BaseCalculo,
// AliquotaPis e AliquotaCofins com vBCPisCofins, pAliqPis e pAliqCofins, mesmo
// quando vêm vazios (ACBrNFSeX.LerIni.pas).
func (b *iniBuilder) tributacao(v Valores) {
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
		b.KVIntOpt("cPaisResult", t.CPaisResult)
	} else {
		b.KV("tribISSQN", strconv.Itoa(cmp.Or(v.TribISSQN, 1)))
		b.KVOpt("pAliq", inifmt.MoneyOpt(v.PAliq))
	}

	if t := v.TribFed; t != nil {
		b.Secao("tribFederal")
		b.KVOpt("CST", t.CST)
		b.KVOpt("vBCPisCofins", inifmt.MoneyOpt(t.VBCPisCofins))
		b.KVOpt("pAliqPis", inifmt.MoneyOpt(t.PAliqPis))
		b.KVOpt("pAliqCofins", inifmt.MoneyOpt(t.PAliqCofins))
		b.KVOpt("vPis", inifmt.MoneyOpt(t.VPis))
		b.KVOpt("vCofins", inifmt.MoneyOpt(t.VCofins))
		if r := t.TpRetPisCofins; r != nil {
			// 0 é "PIS/COFINS/CSLL não retidos", valor válido. Um int com omitempty
			// o descartava, e sem a chave a lib deixa o campo vazio, que há gravador
			// emitindo mesmo assim: o IPM saía com <tipo_retencao/>.
			b.KV("tpRetPisCofins", strconv.Itoa(*r))
		}
		b.KVOpt("vRetCP", inifmt.MoneyOpt(t.VRetCP))
		b.KVOpt("vRetIRRF", inifmt.MoneyOpt(t.VRetIRRF))
		b.KVOpt("vRetCSLL", inifmt.MoneyOpt(t.VRetCSLL))
		// As bases das retenções federais. A lib as lê separadas dos valores
		// retidos, e sem elas o provedor que confere base contra retenção
		// recebe a retenção sozinha.
		b.KVOpt("vBCPIRRF", inifmt.MoneyOpt(t.VBCPIRRF))
		b.KVOpt("vBCCSLL", inifmt.MoneyOpt(t.VBCCSLL))
		b.KVOpt("vBCPCP", inifmt.MoneyOpt(t.VBCPCP))
	}

	if t := v.TotTrib; t != nil {
		b.Secao("totTrib")
		if i := t.IndTotTrib; i != nil {
			// 0 é o único valor válido do indicador de não destaque, e era o que
			// KVIntOpt descartava. Sem a chave, o gravador do GISS cai no ramo do
			// pTotTribSN e emite 0.00, que a prefeitura recusa com E160.
			b.KV("indTotTrib", strconv.Itoa(*i))
		}
		b.KVOpt("pTotTribSN", inifmt.MoneyOpt(t.PTotTribSN))
		b.KVOpt("vTotTribFed", inifmt.MoneyOpt(t.VTotTribFed))
		b.KVOpt("vTotTribEst", inifmt.MoneyOpt(t.VTotTribEst))
		b.KVOpt("vTotTribMun", inifmt.MoneyOpt(t.VTotTribMun))
		b.KVOpt("pTotTribFed", inifmt.MoneyOpt(t.PTotTribFed))
		b.KVOpt("pTotTribEst", inifmt.MoneyOpt(t.PTotTribEst))
		b.KVOpt("pTotTribMun", inifmt.MoneyOpt(t.PTotTribMun))
	}

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
	// Campos que provedores específicos leem daqui: SigISSWeb (operação com o
	// exterior, consumo pessoal), Conam (alíquota única) e eGoverneISS (local
	// de incidência). Todos têm default "0" no leitor, então omitir é neutro.
	b.KVOpt("OperExterior", g.OperExterior)
	b.KVOpt("OperUF", g.OperUF)
	b.KVOpt("OperxCidade", g.OperxCidade)
	b.KVOpt("ConsumoPessoal", g.ConsumoPessoal)
	b.KVOpt("IndOpeOne", g.IndOpeOne)
	b.KVOpt("IdLocalIncidencia", g.IdLocalIncidencia)
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
		b.KVOpt("InscricaoEstadual", d.IE)
		b.KVOpt("xPais", d.XPais)
		b.KVOpt("cNaoNIF", d.CNaoNIF)
		b.KVOpt("TipoServico", d.TipoServico)
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
		b.KVOpt("CodigoMunicipio", m.CMun)
		// Imóvel no exterior: a lib guarda estes quatro num endereço à parte.
		b.KVOpt("cEndPost", m.CEndPost)
		b.KVOpt("xCidade", m.XCidade)
		b.KVOpt("xEstProvReg", m.XEstProvReg)
		b.KVOpt("cPais", m.CPais)
	}
	// [gRefNFSeNN]: as NFS-e que este documento referencia. A seção é indexada
	// com DOIS dígitos, e refNFSe é a chave-âncora: sem ela o leitor para no
	// índice e as seguintes somem em silêncio.
	for i, ref := range g.GRefNFSe {
		b.Secao("gRefNFSe" + inifmt.Seq2(i+1))
		b.KV("refNFSe", ref)
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
		// xNome é a grafia alternativa que o leitor aceita como fallback de
		// RazaoSocial. Vai junto, como nos outros pares do INI.
		b.KVOpt("xNome", d.FornecNome)
		b.KVOpt("NIF", d.FornecNIF)
		b.KVOpt("cNaoNIF", d.FornecCNaoNIF)
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

	// [IBSCBSNFSE] e [IBSCBSValoresNFSE] são o lado da NFS-e do grupo, e parte
	// dos gravadores municipais os escreve DENTRO do RPS: o GISS 2.04 tira daqui
	// cLocalidadeIncid, pRedutor e vBC, e sem eles a nota sai com
	// <cLocalidadeIncid>0000000</cLocalidadeIncid>.
	if n := g.NFSe; n != nil {
		b.Secao("IBSCBSNFSE")
		b.KVOpt("cLocalidadeIncid", n.CLocalidadeIncid)
		b.KVOpt("xLocalidadeIncid", n.XLocalidadeIncid)
		b.KVOpt("pRedutor", inifmt.MoneyOpt(n.PRedutor))
		if v := n.Valores; v != nil {
			b.Secao("IBSCBSValoresNFSE")
			b.KVOpt("vBC", inifmt.MoneyOpt(v.VBC))
			b.KVOpt("vCalcReeRepRes", inifmt.MoneyOpt(v.VCalcReeRepRes))
			b.KVOpt("pIBSUF", inifmt.MoneyOpt(v.PIBSUF))
			b.KVOpt("pRedAliqUF", inifmt.MoneyOpt(v.PRedAliqUF))
			b.KVOpt("pAliqEfetUF", inifmt.MoneyOpt(v.PAliqEfetUF))
			b.KVOpt("pIBSMun", inifmt.MoneyOpt(v.PIBSMun))
			b.KVOpt("pRedAliqMun", inifmt.MoneyOpt(v.PRedAliqMun))
			b.KVOpt("pAliqEfetMun", inifmt.MoneyOpt(v.PAliqEfetMun))
			b.KVOpt("pCBS", inifmt.MoneyOpt(v.PCBS))
			b.KVOpt("pRedAliqCBS", inifmt.MoneyOpt(v.PRedAliqCBS))
			b.KVOpt("pAliqEfetCBS", inifmt.MoneyOpt(v.PAliqEfetCBS))
		}
		// Os totais consolidados. O leitor só entra em [gTribRegularNFSe],
		// [gTribCompraGov], [TotgIBS] e [TotgCBS] se [TotCIBS] existir, e por
		// isso as quatro saem de dentro dela: escritas soltas, seriam lidas por
		// ninguém.
		if t := n.TotCIBS; t != nil {
			b.Secao("TotCIBS")
			b.KVOpt("vTotNF", inifmt.MoneyOpt(t.VTotNF))
			if r := t.TribRegular; r != nil {
				b.Secao("gTribRegularNFSe")
				b.KVOpt("pAliqEfeRegIBSUF", inifmt.MoneyOpt(r.PAliqEfeRegIBSUF))
				b.KVOpt("vTribRegIBSUF", inifmt.MoneyOpt(r.VTribRegIBSUF))
				b.KVOpt("pAliqEfeRegIBSMun", inifmt.MoneyOpt(r.PAliqEfeRegIBSMun))
				b.KVOpt("vTribRegIBSMun", inifmt.MoneyOpt(r.VTribRegIBSMun))
				b.KVOpt("pAliqEfeRegCBS", inifmt.MoneyOpt(r.PAliqEfeRegCBS))
				b.KVOpt("vTribRegCBS", inifmt.MoneyOpt(r.VTribRegCBS))
			}
			if c := t.CompraGov; c != nil {
				b.Secao("gTribCompraGov")
				b.KVOpt("pIBSUF", inifmt.MoneyOpt(c.PIBSUF))
				b.KVOpt("vIBSUF", inifmt.MoneyOpt(c.VIBSUF))
				b.KVOpt("pIBSMun", inifmt.MoneyOpt(c.PIBSMun))
				b.KVOpt("vIBSMun", inifmt.MoneyOpt(c.VIBSMun))
				b.KVOpt("pCBS", inifmt.MoneyOpt(c.PCBS))
				b.KVOpt("vCBS", inifmt.MoneyOpt(c.VCBS))
			}
			if i := t.IBS; i != nil {
				b.Secao("TotgIBS")
				b.KVOpt("vIBSTot", inifmt.MoneyOpt(i.VIBSTot))
				b.KVOpt("pCredPresIBS", inifmt.MoneyOpt(i.PCredPresIBS))
				b.KVOpt("vCredPresIBS", inifmt.MoneyOpt(i.VCredPresIBS))
				b.KVOpt("vDifUF", inifmt.MoneyOpt(i.VDifUF))
				b.KVOpt("vIBSUF", inifmt.MoneyOpt(i.VIBSUF))
				b.KVOpt("vDifMun", inifmt.MoneyOpt(i.VDifMun))
				b.KVOpt("vIBSMun", inifmt.MoneyOpt(i.VIBSMun))
			}
			if c := t.CBS; c != nil {
				b.Secao("TotgCBS")
				b.KVOpt("vDifCBS", inifmt.MoneyOpt(c.VDifCBS))
				b.KVOpt("vCBS", inifmt.MoneyOpt(c.VCBS))
				b.KVOpt("pCredPresCBS", inifmt.MoneyOpt(c.PCredPresCBS))
				b.KVOpt("vCredPresCBS", inifmt.MoneyOpt(c.VCredPresCBS))
			}
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
	b.KVIntOpt("CodigoPais", codigoPais(p))
	b.KVOpt("xPais", p.XPais)
	b.KVOpt("xMunicipio", p.XMunicipio)
	b.KVOpt("Telefone", p.Telefone)
	b.KVOpt("Email", p.Email)
	// Identificação no exterior: as três seções leem estas três.
	b.KVOpt("NIF", p.NIF)
	b.KVOpt("cNaoNIF", p.CNaoNIF)
	b.KVOpt("CAEPF", p.CAEPF)
}

// pessoaPrestador emite as chaves que SÓ a seção [Prestador] lê. Separado de
// pessoaCommon porque chave que a seção não lê é chave morta, e o lockstep
// cobra isso.
func (b *iniBuilder) pessoaPrestador(p Prestador) {
	b.KVOpt("TipoPessoa", p.TipoPessoa)
	b.KVOpt("InscricaoEstadual", p.InscricaoEstadual)
	b.KVOpt("NomeFantasia", p.NomeFantasia)
	b.KVOpt("TipoLogradouro", p.TipoLogradouro)
	b.KVOpt("DDD", p.DDD)
	b.KVOpt("xSite", p.XSite)
	b.KVOpt("crc", p.CRC)
	b.KVOpt("crc_estado", p.CRCEstado)
	b.KVOpt("Anexo", p.Anexo)
	b.KVOpt("ValorReceitaBruta", inifmt.MoneyOpt(p.ValorReceitaBruta))
	b.KVOpt("DataInicioAtividade", inifmt.DataBROpt(p.DataInicioAtividade, b.Local()))
	b.KVIntOpt("OptanteMEISimei", p.OptanteMEISimei)
}

// pessoaTomador emite as chaves que SÓ a seção [Tomador] lê.
func (b *iniBuilder) pessoaTomador(p Tomador) {
	b.KVOpt("TipoPessoa", p.TipoPessoa)
	b.KVOpt("InscricaoEstadual", p.InscricaoEstadual)
	b.KVOpt("NomeFantasia", p.NomeFantasia)
	b.KVOpt("TipoLogradouro", p.TipoLogradouro)
	b.KVOpt("TipoBairro", p.TipoBairro)
	b.KVOpt("PontoReferencia", p.PontoReferencia)
	b.KVOpt("DDD", p.DDD)
	b.KVOpt("TipoTelefone", p.TipoTelefone)
	b.KVOpt("DocEstrangeiro", p.DocEstrangeiro)
	b.KVOpt("EnderecoInformado", p.EnderecoInformado)
	b.KVIntOpt("AtualizaTomador", p.AtualizaTomador)
	b.KVIntOpt("TomadorExterior", p.TomadorExterior)
	b.KVIntOpt("TomadorSubstitutoTributario", p.TomadorSubstituto)
}

// pessoaIntermediario emite a chave que SÓ a seção [Intermediario] lê.
func (b *iniBuilder) pessoaIntermediario(p Intermediario) {
	b.KVIntOpt("IssRetido", p.IssRetido)
}

// codigoPais é o CodigoPais de uma pessoa, igual nos dois builders.
//
// No ABRASF 2.04 é o país, e não o município, que escolhe o endereço
// (ACBrNFSeXGravarXml_ABRASFv2.pas):
//
//	if GerarEnderecoExterior and (NFSe.Tomador.Endereco.CodigoPais <> 1058) then
//	  Result.AppendChild(GerarEnderecoExteriorTomador)
//
// e o LerIni assume 0 quando a chave falta. Lá dentro a lib trata o país como
// opcional e os 37 XSDs 2.04 que ela traz o exigem; o GISS ainda converte o
// código BACEN para ISO e descarta o que não acha na tabela. Um tomador com
// cMun brasileiro e cPais=76 (o ISO do Brasil) saía como <EnderecoExterior>
// só com o logradouro, e o GISS recusava:
//
//	1871 - Element 'EnderecoCompletoExterior': This element is not expected.
//	Expected is ( CodigoPais ).
//
// Município brasileiro decide, então: o país é 1058 mesmo que venha outro cPais,
// que é o que o gravador do Padrão Nacional já faz ao escolher <endNac> pelo
// CodigoMunicipio. Sem município vale o cPais informado, e nada é presumido, nem
// no ABRASF: parte dos municípios roteados para ele usa gravador APIPropria,
// descendente do Padrão Nacional, onde CodigoPais diferente de 0 sem
// CodigoMunicipio faz sair <endExt>, e um 1058 presumido viraria endereço no
// exterior com país Brasil e campos vazios.
func codigoPais(p Pessoa) int {
	if municipioBrasileiro(p.CMun) {
		return codigoPaisBrasil
	}
	return p.CPais
}

// municipioBrasileiro diz se o código é de município do Brasil: preenchido e
// diferente do 9999999 que o ABRASF usa para o exterior.
func municipioBrasileiro(codigo string) bool {
	return codigo != "" && codigo != codigoMunicipioExterior
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
