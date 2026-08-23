package cte

import (
	"strings"
	"testing"
)

// pedidoSimpFixture monta um CT-e Simplificado rodoviário sintético com um
// trecho de detalhamento e NF-e vinculada — sem dados reais.
func pedidoSimpFixture() PedidoSimp {
	return PedidoSimp{
		Ambiente: "homologacao",
		InfCte: InfCteSimp{
			Ide: Ide{
				CFOP: "5353", NatOp: "Transporte simplificado",
				Serie: 1, NCT: 200,
				CMunIni: "3550308", XMunIni: "Sao Paulo", UFIni: "SP",
				CMunFim: "3304557", XMunFim: "Rio de Janeiro", UFFim: "RJ",
				IndIEToma: 9,
			},
			Emit: Emit{CNPJ: "99999999000191", IE: "ISENTO", XNome: "Transportadora Teste", CRT: 1,
				EnderEmit: &EndeEmi{XLgr: "Rua A", Nro: "100", XBairro: "Centro", CMun: "3550308", XMun: "Sao Paulo", UF: "SP", CEP: "01001000"}},
			Toma: Toma{Toma: 3, IndIEToma: 9, CNPJ: "44555666000172", IE: "ISENTO", XNome: "Tomador Teste",
				EnderToma: Endereco{XLgr: "Rua C", Nro: "300", XBairro: "Centro", CMun: "3304557", XMun: "Rio de Janeiro", UF: "RJ", CEP: "20010000"}},
			InfCarga: InfCarga{VCarga: 25000, ProPred: "Eletronicos",
				InfQ: []InfQ{{CUnid: "01", TpMed: "PESO BRUTO", QCarga: 1200.000}}},
			Det: []Det{{
				CMunIni: "3550308", XMunIni: "Sao Paulo", CMunFim: "3304557", XMunFim: "Rio de Janeiro",
				VPrest: 1500.50, VRec: 1500.50,
				Comp:   []Comp{{XNome: "FRETE VALOR", VComp: 1500.50}},
				InfNFe: []InfNFe{{Chave: "35200714200166000187550010000000123456789012"}},
			}},
			Imp:        Imp{ICMS: ICMS{ICMSSN: &ICMSSN{CST: "90", IndSN: 1}}},
			InfModal:   InfModal{Rodo: &Rodo{RNTRC: "12345678"}},
			Total:      Total{VTPrest: 1500.50, VTRec: 1500.50},
			InfRespTec: &RespTec{CNPJ: "11222333000181", XContato: "Suporte", Email: "ti@exemplo.com", Fone: "1133334444"},
		},
	}
}

func TestToINISimp_SecoesEChaves(t *testing.T) {
	ini := ToINISimp(pedidoSimpFixture())
	must := []string{
		"[infCTe]", "versao=4.00",
		"[ide]", "tpCTe=5", "mod=57", "modal=01",
		"[emit]", "CNPJ=99999999000191",
		"[toma]", "toma=3", "CNPJCPF=44555666000172", "xNome=Tomador Teste",
		"[infCarga]", "proPred=Eletronicos",
		"[det001]", "cMunIni=3550308", "vPrest=1500,50",
		"[Comp001001]", "xNome=FRETE VALOR", "vComp=1500,50",
		"[infNFe001001]", "chave=35200714200166000187550010000000123456789012",
		"[ICMSSN]", "CST=90",
		"[Rodo]", "RNTRC=12345678",
		"[total]", "vTPrest=1500,50", "vTRec=1500,50",
		"[infRespTec]", "xContato=Suporte",
	}
	for _, frag := range must {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI Simplificado não contém %q\n---\n%s", frag, ini)
		}
	}
	// O Simplificado NÃO usa rem/dest/vPrest do Normal.
	for _, proibido := range []string{"[rem]", "[Dest]", "[vPrest]"} {
		if strings.Contains(ini, proibido) {
			t.Errorf("Simplificado não deveria conter %q\n---\n%s", proibido, ini)
		}
	}
}

func TestToINISimp_IBSCBSReusaImposto(t *testing.T) {
	p := pedidoSimpFixture()
	p.InfCte.Imp.IBSCBS = &TribCTe{CST: "000", CClassTrib: "200001",
		GIBSCBS: &CIBS{VBC: 1500.50, VIBS: 100.00,
			GIBSUF:  GIBSUF{PIBSUF: 5, VIBSUF: 75.00},
			GIBSMun: GIBSMun{PIBSMun: 2, VIBSMun: 25.00},
			GCBS:    GCBS{PCBS: 8, VCBS: 120.00}}}
	ini := ToINISimp(p)
	for _, frag := range []string{"[IBSCBS]", "cClassTrib=200001", "[gIBSCBS]", "vIBS=100,00", "[gCBS]", "vCBS=120,00"} {
		if !strings.Contains(ini, frag) {
			t.Errorf("IBSCBS no Simplificado não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINISimp_DocAntTranspParcial(t *testing.T) {
	p := pedidoSimpFixture()
	p.InfCte.Det[0].InfDocAnt = []DetDocAnt{{
		ChCTe: "35200714200166000187570010000000123456789012", TpPrest: 1,
		InfNFeTranspParcial: []TranspParcial{{ChNFe: "35200714200166000187550010000000999999999999"}},
	}}
	ini := ToINISimp(p)
	for _, frag := range []string{
		"[infDocAnt001001]", "chCTe=35200714200166000187570010000000123456789012",
		"[infNFeTranspParcial001001001]", "chNFe=35200714200166000187550010000000999999999999",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("docAnt/transpParcial não contém %q\n---\n%s", frag, ini)
		}
	}
}
