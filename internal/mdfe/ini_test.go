package mdfe

import (
	"strings"
	"testing"
)

// pedidoFixture monta um MDF-e rodoviário sintético (uma carga, uma descarga
// com NF-e), sem dados reais.
func pedidoFixture() PedidoEmissao {
	return PedidoEmissao{
		Ambiente: "homologacao",
		InfMDFe: InfMDFe{
			Ide: Ide{
				TpEmit: 1, Serie: 1, NMDF: 100,
				UFIni: "SP", UFFim: "RJ",
				InfMunCarrega: []InfMunCarrega{{CMunCarrega: "3550308", XMunCarrega: "Sao Paulo"}},
				InfPercurso:   []InfPercurso{{UFPer: "MG"}},
			},
			Emit: Emit{CNPJ: "99999999000191", IE: "ISENTO", XNome: "Transportadora Teste",
				EnderEmit: &EndeEmi{XLgr: "Rua A", Nro: "100", XBairro: "Centro", CMun: "3550308", XMun: "Sao Paulo", UF: "SP", CEP: "01001000"}},
			InfModal: InfModal{Rodo: &Rodo{
				InfANTT:    &InfANTT{RNTRC: "12345678"},
				VeicTracao: VeicTracao{Placa: "ABC1D23", Tara: 5000, TpRod: "01", TpCar: "02", UF: "SP", Condutor: []Condutor{{XNome: "Joao Motorista", CPF: "11122233344"}}},
			}},
			InfDoc: InfDoc{InfMunDescarga: []InfMunDescarga{{
				CMunDescarga: "3304557", XMunDescarga: "Rio de Janeiro",
				InfNFe: []InfNFe{{ChNFe: "35200714200166000187550010000000123456789012"}},
			}}},
			Tot: Tot{QNFe: 1, VCarga: 25000, CUnid: "01", QCarga: 1200.000},
		},
	}
}

func TestToINI_SecoesEChaves(t *testing.T) {
	ini := ToINI(pedidoFixture())
	must := []string{
		"[ide]", "tpAmb=2", "mod=58", "modal=1", "serie=1", "nMDF=100", "UFIni=SP", "UFFim=RJ",
		"[perc001]", "UFPer=MG",
		"[CARR001]", "cMunCarrega=3550308",
		"[emit]", "CNPJCPF=99999999000191",
		"[Rodo]", "[infANTT]", "RNTRC=12345678",
		"[veicTracao]", "placa=ABC1D23", "tara=5000", "tpRod=01", "tpCar=02",
		"[moto001]", "xNome=Joao Motorista", "CPF=11122233344",
		"[DESC001]", "cMunDescarga=3304557",
		"[infNFe001001]", "chNFe=35200714200166000187550010000000123456789012",
		"[tot]", "qNFe=1", "vCarga=25000,00", "cUnid=01", "qCarga=1200,00",
	}
	for _, frag := range must {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_TpAmbProducao(t *testing.T) {
	p := pedidoFixture()
	p.Ambiente = "producao"
	if !strings.Contains(ToINI(p), "tpAmb=1") {
		t.Error("produção deveria gerar tpAmb=1")
	}
}

func TestEncerramentoINI(t *testing.T) {
	chave := "35200799999999000191580010000001001234567890"
	ini := ToINIEncerramento(chave, "99999999000191", "135200000999", "2026-05-31 10:00:00",
		PedidoEncerramento{CUF: 35, CMun: "3550308"})
	// Formato INDEXADO que o MDFe.LerFromIni lê (não singular [evento]/[detEvento]).
	for _, frag := range []string{"[EVENTO]", "[EVENTO001]", "tpEvento=110112", "chMDFe=" + chave,
		"cOrgao=35", "CNPJCPF=99999999000191", "dtEnc=", "cMun=3550308", "nProt=135200000999"} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI de encerramento não contém %q\n---\n%s", frag, ini)
		}
	}
	if strings.Contains(ini, "[evento]") || strings.Contains(ini, "[detEvento]") {
		t.Errorf("INI usa formato SINGULAR (não lido pelo LerFromIni):\n%s", ini)
	}
}

func TestInclusaoDFeINI(t *testing.T) {
	ini := ToINIInclusaoDFe("35200799999999000191580010000001001234567890", "99999999000191", "2026-05-31 10:00:00",
		PedidoInclusaoDFe{CMunCarrega: "3550308", XMunCarrega: "SAO PAULO", Documentos: []DocInclusao{
			{ChNFe: "35200711111111000111550010000000011000000017", CMunDescarga: "3304557", XMunDescarga: "RIO DE JANEIRO"},
		}})
	for _, frag := range []string{"tpEvento=110115", "[EVENTO001]", "cMunCarrega=3550308",
		"[infDoc0001]", "chNFe=35200711111111000111550010000000011000000017", "cMunDescarga=3304557"} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI de inclusão de DF-e não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_GruposCompletos(t *testing.T) {
	p := pedidoFixture()
	p.InfMDFe.InfModal.Rodo.InfANTT.InfCIOT = []InfCIOT{{CIOT: "121212121212", CNPJ: "11222333000181"}}
	p.InfMDFe.InfModal.Rodo.InfANTT.ValePed = &ValePed{CategCombVeic: "06", Disp: []Disp{{CNPJForn: "11222333000181", VValePed: 50.0}}}
	p.InfMDFe.InfModal.Rodo.VeicReboque = []VeicReboque{{Placa: "DEF2G34", Tara: 3000, TpCar: "02", UF: "SP"}}
	p.InfMDFe.InfModal.Rodo.LacRodo = []LacRodo{{NLacre: "L123"}}
	p.InfMDFe.Seg = []Seg{{InfResp: InfResp{RespSeg: 1, CNPJ: "11222333000181"}, InfSeg: &InfSeg{XSeg: "Seguradora", CNPJ: "44555666000172"}, NApol: "APX"}}
	ini := ToINI(p)
	for _, frag := range []string{
		"[infCIOT001]", "CIOT=121212121212",
		"[valePed]", "CategCombVeic=06", "[disp001]", "vValePed=50,00",
		"[reboque01]", "placa=DEF2G34", "[lacRodo001]", "nLacre=L123",
		"[seg001]", "respSeg=1", "xSeg=Seguradora",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestParseEnvio_Autorizado(t *testing.T) {
	resp := "[Envio]\ncStat=100\nxMotivo=Autorizado o uso do MDF-e\nchMDFe=35200799999999000191580010000001001234567890\nnProt=135200000999\n"
	e := ParseEnvio(resp)
	if !e.Sucesso || StatusEmissao(e) != "autorizado" {
		t.Fatalf("esperava autorizado; got %+v", e)
	}
}

func TestParseEvento_Concluido(t *testing.T) {
	ev := ParseEvento("[Evento]\ncStat=135\nxMotivo=Evento registrado e vinculado ao MDF-e\nnProt=135200000111\n")
	if !ev.Sucesso || StatusEvento(ev) != "concluido" {
		t.Fatalf("esperava concluido; got %+v", ev)
	}
}

// TestToINIPagamentoOperacao: 110116 com infViagens/infPag/Comp/infPrazo/infBanc
// (índices compostos 3+3) e floats com vírgula.
func TestToINIPagamentoOperacao(t *testing.T) {
	ini := ToINIPagamentoOperacao("35200799999999000191580010000001001234567890", "99999999000191", "2026-05-31 10:00:00",
		PedidoPagamentoOperacao{
			InfViagens: &InfViagens{QtdViagens: 1, NroViagem: 1},
			Pagamentos: []InfPag{{
				XNome: "TRANSPORTADOR", CNPJ: "11222333000181", VContrato: 1000.5, IndPag: 1,
				Comp:     []Comp{{TpComp: "01", VComp: 1000.5, XComp: "frete"}},
				InfPrazo: []InfPrazo{{NParcela: 1, DVenc: "2026-06-30", VParcela: 1000.5}},
				InfBanc:  InfBanc{PIX: "chave-pix@ex.com"},
			}},
		})
	for _, s := range []string{"tpEvento=110116", "[EVENTO001]", "[infViagens]", "qtdViagens=1",
		"[infPag001]", "vContrato=1000,50", "indPag=1", "[Comp001001]", "vComp=1000,50",
		"[infPrazo001001]", "vParcela=1000,50", "[infBanc001]", "PIX=chave-pix@ex.com"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI pagamento sem %q\n%s", s, ini)
		}
	}
}
