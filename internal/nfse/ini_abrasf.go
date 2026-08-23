package nfse

import "strconv"

// ToINIAbrasf traduz o MESMO pedido JSON (contrato único, estilo Padrão
// Nacional) para o INI RPS genérico consumido pela ACBrLibNFSe pelos provedores
// NÃO-PN — tanto ABRASF (v1/v2) quanto os de layout PRÓPRIO. O leitor de INI do
// NFSeX é único: todos os provedores leem deste mesmo formato; o que é
// proprietário é só o XML de SAÍDA, gerado pela engine por município.
//
// Diferenças em relação ao Padrão Nacional (ToINI):
//   - OMITE os grupos exclusivos do PN: [tribMun] detalhado, [tribFed],
//     [totTrib] e [IBSCBSDPS] (Reforma Tributária).
//   - Emite os campos que o ABRASF exige: regime/optante/incentivador no
//     [Prestador] e Status/Tipo/NaturezaOperacao no [IdentificacaoRps].
//   - Retenções federais vão DENTRO de [Valores] (não numa seção separada).
//
// Mapeamento dos campos do contrato único: serv.itemListaServico (LC116) com
// fallback para serv.cServ; serv.cTribMun → CodigoTributacaoMunicipio;
// valores.pAliq → Aliquota; opSimpNac (PN) → OptanteSN (ver optanteSN).
func ToINIAbrasf(p DPSPedido) string {
	inf := p.InfDPS
	var b iniBuilder

	b.section("IdentificacaoNFSe")
	b.kv("TipoXML", "RPS")

	b.section("IdentificacaoRps")
	b.kv("Numero", inf.NDPS)
	b.kv("Serie", inf.Serie)
	b.kv("Tipo", "1") // 1 = RPS
	b.kv("DataEmissao", dateBR(inf.DhEmi))
	b.kv("Competencia", dateBR(inf.DCompet))
	b.kv("Status", "1") // 1 = normal (não cancelado)
	b.kv("NaturezaOperacao", strconv.Itoa(defaultInt(inf.Serv.NaturezaOperacao, 1)))
	b.kv("tpEmit", strconv.Itoa(defaultInt(inf.TpEmit, 1)))
	b.kv("cLocEmi", defaultStr(inf.CLocEmi, inf.Prest.CMun))

	b.section("Prestador")
	b.pessoaCommon(inf.Prest)
	if rt := inf.Prest.RegTrib; rt != nil {
		b.kv("OptanteSN", strconv.Itoa(optanteSN(rt.OpSimpNac)))
		b.kvIntOpt("RegimeEspTrib", rt.RegEspTrib)
		b.kvIntOpt("RegimeApuracaoSN", rt.RegApTribSN)
		b.kv("IncentivadorCultural", strconv.Itoa(defaultInt(rt.IncentCultural, 2)))
		b.kvOpt("DataOptanteSimplesNacional", dateBROpt(rt.DataOpSimpNac))
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
	b.kvOpt("CodigoMunicipio", firstNonEmpty(s.MunIncidencia, s.CMunPrestacao))
	b.kv("ItemListaServico", firstNonEmpty(s.ItemListaServico, s.CServ))
	// CodigoServicoNacional é o código nacional do serviço (o cTribNac do Padrão
	// Nacional). No ABRASF ele é um campo PRÓPRIO — até aqui o cServ do pedido só
	// servia de fallback do ItemListaServico e nunca chegava ao XML. Não existe no
	// Padrão Nacional (o gravador dele não lê este campo), por isso só aqui.
	b.kvOpt("CodigoServicoNacional", s.CServ)
	b.kvOpt("CodigoTributacaoMunicipio", s.CTribMun)
	// CodigoNBS alimenta o <CodigoNbs> do layout ABRASF (tsCodigoNbs, 9
	// caracteres, entre CodigoTributacaoMunicipio e Discriminacao). Faltava aqui
	// — o builder do Padrão Nacional já emitia, então o campo cNBS do contrato
	// era simplesmente descartado nos municípios não-PN. Provedores que suprimem
	// a tag (NrOcorrCodigoNBS=-1: SigCorp, GovDigital, Etherium, Sudoeste) seguem
	// sem ela, o que é do provedor, não do pedido.
	b.kvOpt("CodigoNBS", nbs(s.CNBS))
	b.kv("Discriminacao", s.XDescServ)
	b.kvOpt("CodigoCnae", s.CodigoCnae)
	b.kvIntOpt("ExigibilidadeISS", s.ExigibISS)
	b.kvOpt("MunicipioIncidencia", s.MunIncidencia)
	// País do LOCAL DA PRESTAÇÃO (serviço prestado no exterior). O ABRASF aceita
	// (tcDadosServico tem CodigoPais) e só o builder do PN enviava — mesma
	// família do CodigoPais do tomador, que fazia o endereço nacional virar
	// exterior. Achado pelo teste de lockstep.
	b.kvOpt("CodigoPais", s.CodigoPais)
	b.kvOpt("xPais", s.XPais)
	// NumeroProcesso (processo judicial/administrativo que suspende a
	// exigibilidade do ISS). Existe nos dois layouts — o builder do Padrão
	// Nacional já enviava e este descartava. Máx. 30 no ABRASF.
	b.kvOpt("NumeroProcesso", s.NumeroProcesso)
	b.kvIntOpt("ResponsavelRetencao", s.RespRetencao)

	v := inf.Valores
	aliq := v.PAliq
	if aliq == 0 && v.TribMun != nil {
		aliq = v.TribMun.PAliq
	}
	b.section("Valores")
	b.kv("ValorServicos", money(v.VServ))
	// IssRetido (ABRASF) é Sim/Não obrigatório; default 2 (não retido).
	b.kv("IssRetido", strconv.Itoa(defaultInt(v.IssRetido, 2)))
	b.kvOpt("Aliquota", moneyOpt(aliq))
	b.kvOpt("ValorRecebido", moneyOpt(v.VReceb))
	b.kvOpt("DescontoIncondicionado", moneyOpt(v.VDescIncond))
	b.kvOpt("DescontoCondicionado", moneyOpt(v.VDescCond))
	b.kvOpt("ValorDeducoes", moneyOpt(v.VDeducoes))
	b.kvOpt("AliquotaDeducoes", moneyOpt(v.PDeducoes))
	// Retenções federais: no ABRASF ficam dentro de [Valores] (não há [tribFed]).
	if t := v.TribFed; t != nil {
		b.kvOpt("ValorPis", moneyOpt(t.VPis))
		b.kvOpt("ValorCofins", moneyOpt(t.VCofins))
		b.kvOpt("ValorInss", moneyOpt(t.VRetCP))
		b.kvOpt("ValorIr", moneyOpt(t.VRetIRRF))
		b.kvOpt("ValorCsll", moneyOpt(t.VRetCSLL))
	}

	b.itemServico(s, v, aliq)
	return b.String()
}

// itemServico emite [Itens001] — a LISTA de serviços da NFS-e, derivada do
// serviço único do contrato.
//
// Por que existe: alguns layouts não têm o bloco <Servico> singular. O fintelISS
// 2.02, por exemplo, tem `GerarServico` devolvendo nil e monta um <ListaServicos>
// a partir de `NFSe.Servico.ItemServico`. Sem nenhum item, a lista saía VAZIA e
// discriminação, item da lista e valores sumiam do XML — o pedido era aceito e
// o documento saía oco. Achado pela varredura por provedor.
//
// A lib preenche a lista lendo seções indexadas [Itens001..] e para no primeiro
// índice sem `Descricao` — por isso ela é a âncora e vai sempre. Os demais campos
// do item que esses layouts usam vêm do serviço compartilhado, que já enviamos;
// aqui só entram os que a lib lê do PRÓPRIO item.
//
// Nosso contrato tem um serviço só, então emitimos exatamente um item.
func (b *iniBuilder) itemServico(s Servico, v Valores, aliq float64) {
	if s.XDescServ == "" {
		return // sem descrição não há âncora: a lib pararia no índice 1 de qualquer forma
	}
	b.section("Itens001")
	b.kv("Descricao", s.XDescServ)
	b.kvOpt("ItemListaServico", firstNonEmpty(s.ItemListaServico, s.CServ))
	b.kvOpt("CodigoCnae", s.CodigoCnae)
	// Serviço único: uma unidade cujo valor é o total.
	b.kv("Quantidade", "1")
	b.kv("ValorUnitario", money(v.VServ))
	// ValorTotal é o que os gravadores leem do item. A lib documenta um fallback
	// ('ValorTotal' com default vindo de 'ValorServicos'), mas ele NÃO funciona na
	// prática — verificado: sem a chave explícita o valor chega zerado no XML.
	// Mandamos as duas: explícita para o item, e ValorServicos porque parte dos
	// layouts lê esse nome.
	b.kv("ValorServicos", money(v.VServ))
	b.kv("ValorTotal", money(v.VServ))
	b.kvOpt("Aliquota", moneyOpt(aliq))
	b.kvOpt("ValorDeducoes", moneyOpt(v.VDeducoes))
	b.kvOpt("DescontoIncondicionado", moneyOpt(v.VDescIncond))
	b.kvOpt("DescontoCondicionado", moneyOpt(v.VDescCond))
}

// optanteSN converte o opSimpNac do Padrão Nacional (1=não optante, 2=MEI,
// 3=ME/EPP) no OptanteSimplesNacional do ABRASF (1=Sim, 2=Não).
func optanteSN(opSimpNac int) int {
	if opSimpNac == 2 || opSimpNac == 3 {
		return 1
	}
	return 2
}
