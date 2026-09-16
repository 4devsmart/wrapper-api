package nfse

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func pedidoAbrasfExemplo() DPSPedido {
	return DPSPedido{
		Ambiente: "homologacao",
		InfDPS: InfDPS{
			Serie: "1", NDPS: "100", DCompet: "2026-05-01", DhEmi: "2026-05-01",
			Prest: Prestador{
				Pessoa:  Pessoa{CNPJ: "12345678000190", IM: "98765", CMun: "3304557", UF: "RJ"},
				RegTrib: &RegTrib{OpSimpNac: 3, RegEspTrib: 0, IncentCultural: 2},
			},
			Toma: &Tomador{Pessoa: Pessoa{CNPJ: "11222333000181", XNome: "Cliente XPTO"}},
			Serv: Servico{
				CMunPrestacao: "3304557", ItemListaServico: "01.07", CTribMun: "010701",
				XDescServ: "Consultoria em TI",
			},
			Valores: Valores{VServ: 1500, PAliq: 2, IssRetido: 2},
		},
	}
}

func TestToINIAbrasf_EstruturaEFamilia(t *testing.T) {
	ini := ToINIAbrasf(pedidoAbrasfExemplo())

	devemConter := []string{
		"[IdentificacaoNFSe]", "TipoXML=RPS",
		"[IdentificacaoRps]", "Tipo=1", "Status=1", "NaturezaOperacao=1",
		"[Prestador]", "OptanteSN=1", "IncentivadorCultural=2",
		"[Servico]", "ItemListaServico=01.07", "CodigoTributacaoMunicipio=010701",
		"Discriminacao=Consultoria em TI",
		"[Valores]", "ValorServicos=1500,00", "IssRetido=2", "Aliquota=2,00",
	}
	for _, s := range devemConter {
		if !strings.Contains(ini, s) {
			t.Errorf("INI ABRASF não contém %q\n---\n%s", s, ini)
		}
	}
}

// TestToINIAbrasf_GruposDetalhados: os grupos detalhados e a Reforma Tributária
// vão ao INI do ABRASF iguais aos do Padrão Nacional. Eram omitidos, e o GISS
// 2.04 os leva ao XML: o PIS/COFINS com CST, os totais e o IBS/CBS sumiam da
// nota.
func TestToINIAbrasf_GruposDetalhados(t *testing.T) {
	p := pedidoAbrasfExemplo()
	p.InfDPS.Valores.TribFed = &TribFed{CST: "01", VBCPisCofins: 6071, PAliqPis: 0.65, VPis: 39.46, TpRetPisCofins: new(int)}
	p.InfDPS.Valores.TotTrib = &TotTrib{PTotTribFed: 3.65}
	p.InfDPS.IBSCBS = &IBSCBSDPS{
		CIndOp:  "000001",
		GIBSCBS: &GIBSCBSDPS{CST: "000", CClassTrib: "000001"},
		// O lado da NFS-e é o que o GISS 2.04 escreve dentro do RPS.
		NFSe: &IBSCBSNFSe{
			CLocalidadeIncid: "3518800", PRedutor: 0,
			Valores: &IBSCBSValoresNFSe{VBC: 6071, PIBSMun: 0.1},
		},
	}
	p.InfDPS.Serv.ComExt = &ComExt{MdPrestacao: "1"}
	p.InfDPS.Serv.InfoCompl = &InfoCompl{XInfComp: "obs"}

	pn, abrasf := ToINI(p), ToINIAbrasf(p)
	for _, secao := range []string{"tribMun", "tribFederal", "totTrib", "IBSCBSDPS", "gIBSCBS",
		"IBSCBSNFSE", "IBSCBSValoresNFSE", "ComercioExterior", "InformacoesComplementares"} {
		if got, quero := secaoINI(abrasf, secao), secaoINI(pn, secao); got == "" || got != quero {
			t.Errorf("[%s] no ABRASF difere do Padrão Nacional\nabrasf:\n%s\npn:\n%s", secao, got, quero)
		}
	}

	// 0 é "PIS/COFINS/CSLL não retidos" e precisa chegar: o int com omitempty o
	// descartava, e o IPM emitia <tipo_retencao/> vazio.
	if got := valorINI(secaoINI(abrasf, "tribFederal"), "tpRetPisCofins"); got != "0" {
		t.Errorf("tpRetPisCofins=0 deveria chegar a [tribFederal], veio %q", got)
	}
}

// TestCodigoPaisServico: no ABRASF, serviço prestado no Brasil leva 1058 mesmo
// sem codigoPais no pedido. Sem ele a homologação do GISS recusa com "E383 -
// Código do pais não informado". O Padrão Nacional segue escrevendo só o que veio.
func TestCodigoPaisServico(t *testing.T) {
	casos := []struct {
		nome                      string
		cMunPrest, munIncid, pais string
		abrasf, pn                string
	}{
		{"prestação em município brasileiro", "3518800", "", "", "1058", ""},
		{"município brasileiro com país ISO", "3518800", "", "0076", "1058", "0076"},
		{"só o município de incidência", "", "3518800", "", "1058", ""},
		{"exportação: incidência brasileira e país informado", "", "3518800", "2496", "2496", "2496"},
		{"prestação no exterior pelo 9999999", "9999999", "", "2496", "2496", "2496"},
		{"sem município nem país", "", "", "", "", ""},
	}
	for _, c := range casos {
		p := pedidoAbrasfExemplo()
		p.InfDPS.Serv.CMunPrestacao, p.InfDPS.Serv.MunIncidencia, p.InfDPS.Serv.CodigoPais = c.cMunPrest, c.munIncid, c.pais
		for layout, par := range map[string]struct{ ini, quero string }{
			"abrasf":          {ToINIAbrasf(p), c.abrasf},
			"padrao_nacional": {ToINI(p), c.pn},
		} {
			if got := valorINI(secaoINI(par.ini, "Servico"), "CodigoPais"); got != par.quero {
				t.Errorf("%s, %s: CodigoPais=%q, quero %q", c.nome, layout, got, par.quero)
			}
		}
	}
}

func TestToINIAbrasf_FallbackItemEAliquota(t *testing.T) {
	p := pedidoAbrasfExemplo()
	p.InfDPS.Serv.ItemListaServico = "" // sem o campo ABRASF → cai no cServ
	p.InfDPS.Serv.CServ = "010801"
	p.InfDPS.Valores.PAliq = 0
	p.InfDPS.Valores.TribMun = &TribMun{PAliq: 5}
	ini := ToINIAbrasf(p)
	if !strings.Contains(ini, "ItemListaServico=010801") {
		t.Error("fallback ItemListaServico→cServ falhou")
	}
	if !strings.Contains(ini, "Aliquota=5,00") {
		t.Error("fallback Aliquota→tribMun.pAliq falhou")
	}
}

func TestOptanteSN(t *testing.T) {
	casos := map[int]int{0: 2, 1: 2, 2: 1, 3: 1}
	for in, want := range casos {
		if got := optanteSN(in); got != want {
			t.Errorf("optanteSN(%d) = %d, quero %d", in, got, want)
		}
	}
}

// secaoINI devolve o conteúdo de uma seção do INI (as asserções precisam ser por
// seção: prestador e tomador têm as MESMAS chaves de endereço).
func secaoINI(ini, nome string) string {
	m := regexp.MustCompile(`(?s)\[` + nome + `\]\n(.*?)(\n\[|\z)`).FindStringSubmatch(ini)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestCodigoPaisEnderecoNacional cobre a escolha entre endereço nacional e
// exterior, que o ABRASF 2.04 faz pelo CodigoPais e o Padrão Nacional pelo
// município (ver codigoPais). Três regressões moram aqui: sem a chave, todo
// endereço nacional saía como <EnderecoExterior> (EloTech); com cPais=76, o GISS
// 2.04 recusava a nota por falta do <CodigoPais> dentro dele; e um 1058
// presumido sem município fazia os gravadores APIPropria emitirem <endExt> com
// país Brasil.
//
// As seções são achadas no INI pela razão social, então uma pessoa nova no
// contrato entra no teste sem ninguém precisar lembrar dela.
func TestCodigoPaisEnderecoNacional(t *testing.T) {
	endereco := Pessoa{
		CNPJ: "44555666000172", XNome: "Pessoa Teste",
		Logradouro: "RUA MACEIO", Numero: "4-22", Bairro: "CENTRO", UF: "SP", CEP: "19470000",
	}
	com := func(mudar func(*Pessoa)) Pessoa {
		p := endereco
		mudar(&p)
		return p
	}

	casos := []struct {
		nome       string
		pessoa     Pessoa
		abrasf, pn string // CodigoPais esperado; vazio é chave ausente
	}{
		{"município brasileiro", com(func(p *Pessoa) { p.CMun = "3541307" }), "1058", "1058"},
		{"município brasileiro com cPais ISO", com(func(p *Pessoa) { p.CMun, p.CPais = "3541307", 76 }), "1058", "1058"},
		// 1058 sem município faria sair <endExt> no Padrão Nacional e nos
		// gravadores APIPropria, que também recebem o INI do ABRASF.
		{"endereço sem município nem país", endereco, "", ""},
		{"exterior sem município", com(func(p *Pessoa) { p.UF, p.CEP, p.CPais = "", "", 2496 }), "2496", "2496"},
		{"exterior pelo município 9999999", com(func(p *Pessoa) { p.CMun, p.CPais = "9999999", 2496 }), "2496", "2496"},
		{"9999999 sem país", com(func(p *Pessoa) { p.CMun = "9999999" }), "", ""},
		{"sem endereço", Pessoa{CNPJ: "44555666000172", XNome: "Pessoa Teste"}, "", ""},
	}
	for _, c := range casos {
		d := comTodasAsPessoas(c.pessoa)
		for layout, par := range map[string]struct{ ini, quero string }{
			"abrasf":          {ToINIAbrasf(d), c.abrasf},
			"padrao_nacional": {ToINI(d), c.pn},
		} {
			secoes := secoesDaPessoa(par.ini, c.pessoa.XNome)
			if len(secoes) < 3 {
				t.Fatalf("%s: esperava prestador, tomador e intermediário no INI, achei %v", layout, secoes)
			}
			for _, s := range secoes {
				if got := valorINI(secaoINI(par.ini, s), "CodigoPais"); got != par.quero {
					t.Errorf("%s, %s, [%s]: CodigoPais=%q, quero %q", c.nome, layout, s, got, par.quero)
				}
			}
		}
	}
}

// comTodasAsPessoas põe a mesma pessoa em todo papel do pedido (prestador,
// tomador, intermediário). Acha os papéis pela Pessoa embutida, então um papel
// novo no contrato entra no teste sem ninguém precisar lembrar dele.
func comTodasAsPessoas(p Pessoa) DPSPedido {
	d := pedidoAbrasfExemplo()
	v := reflect.ValueOf(&d.InfDPS).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		tipo := f.Type()
		ponteiro := tipo.Kind() == reflect.Pointer
		if ponteiro {
			tipo = tipo.Elem()
		}
		if tipo.Kind() != reflect.Struct {
			continue
		}
		if campo, ok := tipo.FieldByName("Pessoa"); !ok || campo.Type != reflect.TypeFor[Pessoa]() {
			continue
		}
		papel := reflect.New(tipo)
		papel.Elem().FieldByName("Pessoa").Set(reflect.ValueOf(p))
		if ponteiro {
			f.Set(papel)
		} else {
			f.Set(papel.Elem())
		}
	}
	return d
}

// secoesDaPessoa lista as seções do INI escritas para a pessoa de nome xNome.
func secoesDaPessoa(ini, xNome string) []string {
	var nomes []string
	for _, m := range regexp.MustCompile(`(?m)^\[(\w+)\]$`).FindAllStringSubmatch(ini, -1) {
		if valorINI(secaoINI(ini, m[1]), "RazaoSocial") == xNome {
			nomes = append(nomes, m[1])
		}
	}
	return nomes
}

func valorINI(secao, chave string) string {
	for l := range strings.SplitSeq(secao, "\n") {
		if k, v, ok := strings.Cut(l, "="); ok && k == chave {
			return v
		}
	}
	return ""
}

// TestCodigoNBS cobre a regressão em que o cNBS do contrato era descartado no
// layout ABRASF: o builder do Padrão Nacional emitia CodigoNBS e o do ABRASF
// não, então o <CodigoNbs> nunca saía nos municípios não-PN.
func TestCodigoNBS(t *testing.T) {
	comNBS := func(v string) DPSPedido {
		p := pedidoAbrasfExemplo()
		p.InfDPS.Serv.CNBS = v
		return p
	}

	// Os dois builders emitem, na seção [Servico].
	for nome, ini := range map[string]string{
		"abrasf":          ToINIAbrasf(comNBS("111032200")),
		"padrao_nacional": ToINI(comNBS("111032200")),
	} {
		if serv := secaoINI(ini, "Servico"); !strings.Contains(serv, "CodigoNBS=111032200") {
			t.Errorf("%s: [Servico] deveria trazer CodigoNBS=111032200\n---\n%s", nome, serv)
		}
	}

	// Pontuação é removida: a lib trunca em 9 caracteres, então "1.1103.22.00"
	// viraria "1.1103.22": um código errado, sem erro nenhum.
	if serv := secaoINI(ToINIAbrasf(comNBS("1.1103.22.00")), "Servico"); !strings.Contains(serv, "CodigoNBS=111032200") {
		t.Errorf("NBS formatado deveria ser normalizado para dígitos\n---\n%s", serv)
	}

	// Sem NBS informado, a chave não aparece (campo é opcional no layout).
	if serv := secaoINI(ToINIAbrasf(pedidoAbrasfExemplo()), "Servico"); strings.Contains(serv, "CodigoNBS") {
		t.Errorf("sem cNBS no pedido, CodigoNBS não deve ser emitido\n---\n%s", serv)
	}
}

// TestCodigoServicoNacional: o cServ do contrato (código nacional do serviço)
// tem campo próprio no ABRASF. Antes ele só servia de fallback do
// ItemListaServico e era descartado quando o pedido trazia os dois.
func TestCodigoServicoNacional(t *testing.T) {
	p := pedidoAbrasfExemplo()
	p.InfDPS.Serv.ItemListaServico = "01.05"
	p.InfDPS.Serv.CServ = "010501"
	serv := secaoINI(ToINIAbrasf(p), "Servico")

	if !strings.Contains(serv, "CodigoServicoNacional=010501") {
		t.Errorf("cServ deveria virar CodigoServicoNacional\n---\n%s", serv)
	}
	// O fallback continua valendo: quem manda só cServ segue com ItemListaServico.
	if !strings.Contains(serv, "ItemListaServico=01.05") {
		t.Errorf("ItemListaServico não deveria ser sobrescrito\n---\n%s", serv)
	}

	// Sem cServ, a chave não aparece (campo opcional no layout).
	semServ := pedidoAbrasfExemplo()
	semServ.InfDPS.Serv.CServ = ""
	if s := secaoINI(ToINIAbrasf(semServ), "Servico"); strings.Contains(s, "CodigoServicoNacional") {
		t.Errorf("sem cServ, CodigoServicoNacional não deve ser emitido\n---\n%s", s)
	}
}

// TestNumeroProcesso: campo existe nos dois layouts (máx. 30 no ABRASF) e o
// builder ABRASF o descartava: mesma classe do cNBS e do cServ.
func TestNumeroProcesso(t *testing.T) {
	p := pedidoAbrasfExemplo()
	p.InfDPS.Serv.NumeroProcesso = "PROC-2026-000123"
	if serv := secaoINI(ToINIAbrasf(p), "Servico"); !strings.Contains(serv, "NumeroProcesso=PROC-2026-000123") {
		t.Errorf("numeroProcesso deveria chegar ao INI\n---\n%s", serv)
	}
	if serv := secaoINI(ToINIAbrasf(pedidoAbrasfExemplo()), "Servico"); strings.Contains(serv, "NumeroProcesso") {
		t.Errorf("sem numeroProcesso, a chave não deve ser emitida\n---\n%s", serv)
	}
}
