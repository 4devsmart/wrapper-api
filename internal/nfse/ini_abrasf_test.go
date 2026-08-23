package nfse

import (
	"regexp"
	"strings"
	"testing"
)

func pedidoAbrasfExemplo() DPSPedido {
	return DPSPedido{
		Ambiente: "homologacao",
		InfDPS: InfDPS{
			Serie: "1", NDPS: "100", DCompet: "2026-05-01", DhEmi: "2026-05-01",
			Prest: Pessoa{
				CNPJ: "12345678000190", IM: "98765", CMun: "3304557", UF: "RJ",
				RegTrib: &RegTrib{OpSimpNac: 3, RegEspTrib: 0, IncentCultural: 2},
			},
			Toma: &Pessoa{CNPJ: "11222333000181", XNome: "Cliente XPTO"},
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

	// Grupos EXCLUSIVOS do Padrão Nacional não podem aparecer no ABRASF.
	for _, s := range []string{"[tribFed]", "[totTrib]", "[IBSCBSDPS]"} {
		if strings.Contains(ini, s) {
			t.Errorf("INI ABRASF não deveria conter o grupo PN %q", s)
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

// TestCodigoPaisEnderecoNacional cobre a regressão em que o endereço nacional do
// tomador saía como <EnderecoExterior> no XML. O layout ABRASF 2.04 decide
// nacional × exterior comparando CodigoPais com 1058; sem a chave o LerIni
// assume 0, e o endereço inteiro (número/bairro/município/UF/CEP) era descartado
// em favor de um único <EnderecoCompletoExterior>.
func TestCodigoPaisEnderecoNacional(t *testing.T) {
	base := func() DPSPedido {
		p := pedidoAbrasfExemplo()
		p.InfDPS.Toma = &Pessoa{
			CNPJ: "44555666000172", XNome: "Tomador Teste",
			CMun: "3205069", UF: "ES", CEP: "29375000",
			Logradouro: "Avenida ANGELO ALTOE", Numero: "340", Bairro: "SAO PEDRO",
		}
		return p
	}

	// Endereço nacional (tem município): 1058 é preenchido sozinho, nos dois
	// builders: o helper de pessoa é compartilhado por ABRASF e Padrão Nacional.
	for nome, ini := range map[string]string{
		"abrasf":          ToINIAbrasf(base()),
		"padrao_nacional": ToINI(base()),
	} {
		if toma := secaoINI(ini, "Tomador"); !strings.Contains(toma, "CodigoPais=1058") {
			t.Errorf("%s: endereço nacional do tomador deveria trazer CodigoPais=1058\n---\n%s", nome, toma)
		}
	}

	// Exterior explícito é respeitado (não sobrescrevemos com 1058).
	pExt := base()
	pExt.InfDPS.Toma.CPais = 249 // Estados Unidos
	pExt.InfDPS.Toma.XPais = "ESTADOS UNIDOS"
	toma := secaoINI(ToINIAbrasf(pExt), "Tomador")
	if !strings.Contains(toma, "CodigoPais=249") || strings.Contains(toma, "CodigoPais=1058") {
		t.Errorf("país estrangeiro informado deveria ser preservado\n---\n%s", toma)
	}
	if !strings.Contains(toma, "xPais=ESTADOS UNIDOS") {
		t.Errorf("xPais deveria ser repassado\n---\n%s", toma)
	}

	// Sem município e sem país: não inventamos país (evita mudar o comportamento
	// de quem manda só contato, sem endereço).
	pSem := base()
	pSem.InfDPS.Toma = &Pessoa{CNPJ: "44555666000172", XNome: "Sem Endereco"}
	if s := secaoINI(ToINIAbrasf(pSem), "Tomador"); strings.Contains(s, "CodigoPais=") {
		t.Errorf("sem município nem país informado, CodigoPais não deve ser emitido\n---\n%s", s)
	}
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
