package nfse

import (
	"cmp"
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// ToINIAbrasf traduz o MESMO pedido JSON (contrato único, estilo Padrão
// Nacional) para o INI RPS genérico consumido pela ACBrLibNFSe pelos provedores
// NÃO-PN: tanto ABRASF (v1/v2) quanto os de layout PRÓPRIO. O leitor de INI do
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

	b.Secao("IdentificacaoNFSe")
	b.KV("TipoXML", "RPS")

	b.Secao("IdentificacaoRps")
	b.KV("Numero", inf.NDPS)
	b.KV("Serie", inf.Serie)
	b.KV("Tipo", "1") // 1 = RPS
	b.KV("DataEmissao", inifmt.DataBR(inf.DhEmi, b.Local()))
	b.KV("Competencia", inifmt.DataBR(inf.DCompet, b.Local()))
	b.KV("Status", "1") // 1 = normal (não cancelado)
	b.KV("NaturezaOperacao", strconv.Itoa(cmp.Or(inf.Serv.NaturezaOperacao, 1)))
	b.KV("tpEmit", strconv.Itoa(cmp.Or(inf.TpEmit, 1)))
	b.KV("cLocEmi", cmp.Or(inf.CLocEmi, inf.Prest.CMun))
	// verAplic identifica o emissor no RPS como identifica no Padrão Nacional.
	// Só o builder do PN emitia, e o campo do contrato era descartado nos
	// municípios não-PN. Achado pelo teste de espelho.
	b.KV("verAplic", cmp.Or(inf.VerAplic, versao.Emissor()))

	b.Secao("Prestador")
	b.pessoaCommon(inf.Prest)
	if rt := inf.Prest.RegTrib; rt != nil {
		b.KV("OptanteSN", strconv.Itoa(optanteSN(rt.OpSimpNac)))
		b.KVIntOpt("RegimeEspTrib", rt.RegEspTrib)
		b.KVIntOpt("RegimeApuracaoSN", rt.RegApTribSN)
		b.KV("IncentivadorCultural", strconv.Itoa(cmp.Or(rt.IncentCultural, 2)))
		b.KVOpt("DataOptanteSimplesNacional", inifmt.DataBROpt(rt.DataOpSimpNac, b.Local()))
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
	b.KVOpt("CodigoMunicipio", fiscal.Primeiro(s.MunIncidencia, s.CMunPrestacao))
	b.KV("ItemListaServico", fiscal.Primeiro(s.ItemListaServico, s.CServ))
	// CodigoServicoNacional é o código nacional do serviço (o cTribNac do Padrão
	// Nacional). No ABRASF ele é um campo PRÓPRIO, até aqui o cServ do pedido só
	// servia de fallback do ItemListaServico e nunca chegava ao XML. Não existe no
	// Padrão Nacional (o gravador dele não lê este campo), por isso só aqui.
	b.KVOpt("CodigoServicoNacional", s.CServ)
	b.KVOpt("CodigoTributacaoMunicipio", s.CTribMun)
	// CodigoNBS alimenta o <CodigoNbs> do layout ABRASF (tsCodigoNbs, 9
	// caracteres, entre CodigoTributacaoMunicipio e Discriminacao). Faltava aqui
	//: o builder do Padrão Nacional já emitia, então o campo cNBS do contrato
	// era simplesmente descartado nos municípios não-PN. Provedores que suprimem
	// a tag (NrOcorrCodigoNBS=-1: SigCorp, GovDigital, Etherium, Sudoeste) seguem
	// sem ela, o que é do provedor, não do pedido.
	b.KVOpt("CodigoNBS", nbs(s.CNBS))
	b.KV("Discriminacao", s.XDescServ)
	b.KVOpt("CodigoCnae", s.CodigoCnae)
	b.KVIntOpt("ExigibilidadeISS", s.ExigibISS)
	b.KVOpt("MunicipioIncidencia", s.MunIncidencia)
	b.KVOpt("xMunicipioIncidencia", s.XMunIncidencia)
	// País do LOCAL DA PRESTAÇÃO (serviço prestado no exterior). O ABRASF aceita
	// (tcDadosServico tem CodigoPais) e só o builder do PN enviava: mesma
	// família do CodigoPais do tomador, que fazia o endereço nacional virar
	// exterior. Achado pelo teste de lockstep.
	b.KVOpt("CodigoPais", s.CodigoPais)
	b.KVOpt("xPais", s.XPais)
	// NumeroProcesso (processo judicial/administrativo que suspende a
	// exigibilidade do ISS). Existe nos dois layouts: o builder do Padrão
	// Nacional já enviava e este descartava. Máx. 30 no ABRASF.
	b.KVOpt("NumeroProcesso", s.NumeroProcesso)
	b.KVIntOpt("ResponsavelRetencao", s.RespRetencao)

	v := inf.Valores
	aliq := v.PAliq
	if aliq == 0 && v.TribMun != nil {
		aliq = v.TribMun.PAliq
	}
	b.Secao("Valores")
	b.KV("ValorServicos", inifmt.Money(v.VServ))
	// IssRetido (ABRASF) é Sim/Não obrigatório; default 2 (não retido).
	b.KV("IssRetido", strconv.Itoa(cmp.Or(v.IssRetido, 2)))
	b.KVOpt("Aliquota", inifmt.MoneyOpt(aliq))
	b.KVOpt("ValorRecebido", inifmt.MoneyOpt(v.VReceb))
	b.KVOpt("DescontoIncondicionado", inifmt.MoneyOpt(v.VDescIncond))
	b.KVOpt("DescontoCondicionado", inifmt.MoneyOpt(v.VDescCond))
	b.KVOpt("ValorDeducoes", inifmt.MoneyOpt(v.VDeducoes))
	b.KVOpt("AliquotaDeducoes", inifmt.MoneyOpt(v.PDeducoes))
	// Retenções federais: no ABRASF ficam dentro de [Valores] (não há [tribFed]).
	if t := v.TribFed; t != nil {
		b.KVOpt("ValorPis", inifmt.MoneyOpt(t.VPis))
		b.KVOpt("ValorCofins", inifmt.MoneyOpt(t.VCofins))
		b.KVOpt("ValorInss", inifmt.MoneyOpt(t.VRetCP))
		b.KVOpt("ValorIr", inifmt.MoneyOpt(t.VRetIRRF))
		b.KVOpt("ValorCsll", inifmt.MoneyOpt(t.VRetCSLL))
	}

	b.itemServico(s, v, aliq)
	return b.String()
}

// itemServico emite [Itens001]: a LISTA de serviços da NFS-e, derivada do
// serviço único do contrato.
//
// Por que existe: alguns layouts não têm o bloco <Servico> singular. O fintelISS
// 2.02, por exemplo, tem `GerarServico` devolvendo nil e monta um <ListaServicos>
// a partir de `NFSe.Servico.ItemServico`. Sem nenhum item, a lista saía VAZIA e
// discriminação, item da lista e valores sumiam do XML: o pedido era aceito e
// o documento saía oco. Achado pela varredura por provedor.
//
// A lib preenche a lista lendo seções indexadas [Itens001..] e para no primeiro
// índice sem `Descricao`, por isso ela é a âncora e vai sempre. Os demais campos
// do item que esses layouts usam vêm do serviço compartilhado, que já enviamos;
// aqui só entram os que a lib lê do PRÓPRIO item.
//
// Nosso contrato tem um serviço só, então emitimos exatamente um item.
func (b *iniBuilder) itemServico(s Servico, v Valores, aliq float64) {
	if s.XDescServ == "" {
		return // sem descrição não há âncora: a lib pararia no índice 1 de qualquer forma
	}
	b.Secao("Itens001")
	b.KV("Descricao", s.XDescServ)
	b.KVOpt("ItemListaServico", fiscal.Primeiro(s.ItemListaServico, s.CServ))
	b.KVOpt("CodigoCnae", s.CodigoCnae)
	// Serviço único: uma unidade cujo valor é o total.
	b.KV("Quantidade", "1")
	b.KV("ValorUnitario", inifmt.Money(v.VServ))
	// ValorTotal é o que os gravadores leem do item. A lib documenta um fallback
	// ('ValorTotal' com default vindo de 'ValorServicos'), mas ele NÃO funciona na
	// prática, está verificado: sem a chave explícita o valor chega zerado no XML.
	// Mandamos as duas: explícita para o item, e ValorServicos porque parte dos
	// layouts lê esse nome.
	b.KV("ValorServicos", inifmt.Money(v.VServ))
	b.KV("ValorTotal", inifmt.Money(v.VServ))
	b.KVOpt("Aliquota", inifmt.MoneyOpt(aliq))
	b.KVOpt("ValorDeducoes", inifmt.MoneyOpt(v.VDeducoes))
	b.KVOpt("DescontoIncondicionado", inifmt.MoneyOpt(v.VDescIncond))
	b.KVOpt("DescontoCondicionado", inifmt.MoneyOpt(v.VDescCond))
}

// optanteSN converte o opSimpNac do Padrão Nacional (1=não optante, 2=MEI,
// 3=ME/EPP) no OptanteSimplesNacional do ABRASF (1=Sim, 2=Não).
func optanteSN(opSimpNac int) int {
	if opSimpNac == 2 || opSimpNac == 3 {
		return 1
	}
	return 2
}
