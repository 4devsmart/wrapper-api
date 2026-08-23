package cte

import (
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// pedidoFixture monta um CT-e rodoviário normal sintético (Simples Nacional)
// com NF-e vinculada e responsável técnico, sem dados reais.
func pedidoFixture() PedidoEmissao {
	return PedidoEmissao{
		Ambiente: "homologacao",
		InfCte: InfCte{
			Ide: Ide{
				CFOP: "5353", NatOp: "Prestacao de servico de transporte",
				Serie: 1, NCT: 100,
				CMunIni: "3550308", XMunIni: "Sao Paulo", UFIni: "SP",
				CMunFim: "3304557", XMunFim: "Rio de Janeiro", UFFim: "RJ",
				IndIEToma: 9, Toma3: &Toma3{Toma: 3},
			},
			Emit: Emit{CNPJ: "99999999000191", IE: "ISENTO", XNome: "Transportadora Teste", CRT: 1,
				EnderEmit: &EndeEmi{XLgr: "Rua A", Nro: "100", XBairro: "Centro", CMun: "3550308", XMun: "Sao Paulo", UF: "SP", CEP: "01001000"}},
			Rem: &Rem{CNPJ: "11222333000181", XNome: "Remetente Teste",
				EnderReme: Endereco{XLgr: "Rua B", Nro: "200", XBairro: "Centro", CMun: "3550308", XMun: "Sao Paulo", UF: "SP", CEP: "01002000"}},
			Dest: &Dest{CNPJ: "44555666000172", XNome: "Destinatario Teste",
				EnderDest: Endereco{XLgr: "Rua C", Nro: "300", XBairro: "Centro", CMun: "3304557", XMun: "Rio de Janeiro", UF: "RJ", CEP: "20010000"}},
			VPrest: VPrest{VTPrest: 1500.50, VRec: 1500.50, Comp: []Comp{{XNome: "FRETE VALOR", VComp: 1500.50}}},
			Imp:    Imp{ICMS: ICMS{ICMSSN: &ICMSSN{CST: "90", IndSN: 1}}},
			InfCTeNorm: &InfCTeNorm{
				InfCarga: InfCarga{VCarga: 25000, ProPred: "Eletronicos",
					InfQ: []InfQ{{CUnid: "01", TpMed: "PESO BRUTO", QCarga: 1200.000}}},
				InfDoc:   &InfDoc{InfNFe: []InfNFe{{Chave: "35200714200166000187550010000000123456789012"}}},
				InfModal: InfModal{Rodo: &Rodo{RNTRC: "12345678"}},
			},
			InfRespTec: &RespTec{CNPJ: "11222333000181", XContato: "Suporte", Email: "ti@exemplo.com", Fone: "1133334444"},
		},
	}
}

func TestToINI_SecoesEChaves(t *testing.T) {
	ini := ToINI(pedidoFixture())
	must := []string{
		"[infCTe]", "versao=4.00",
		"[ide]", "CFOP=5353", "serie=1", "nCT=100", "mod=57", "modal=01", "UFFim=RJ", "toma=3",
		"[emit]", "CNPJ=99999999000191", "CRT=1", "xMun=Sao Paulo",
		"[rem]", "CNPJCPF=11222333000181",
		"[Dest]", "CNPJCPF=44555666000172",
		"[vPrest]", "vTPrest=1500,50",
		"[Comp001]", "xNome=FRETE VALOR", "vComp=1500,50",
		"[ICMSSN]", "CST=90", "indSN=1",
		"[infCarga]", "proPred=Eletronicos",
		"[infQ001]", "qCarga=1200,00",
		"[infNFe001]", "chave=35200714200166000187550010000000123456789012",
		"[Rodo]", "RNTRC=12345678",
		"[infRespTec]", "xContato=Suporte",
	}
	for _, frag := range must {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_MoneyUsaVirgula(t *testing.T) {
	if strings.Contains(ToINI(pedidoFixture()), "vTPrest=1500.50") {
		t.Error("valor monetário saiu com ponto; o INI do ACBr exige vírgula")
	}
}

func TestToINI_VariantesICMS(t *testing.T) {
	p := pedidoFixture()
	p.InfCte.Imp.ICMS = ICMS{ICMS00: &ICMS00{CST: "00", VBC: 1500.50, PICMS: 12, VICMS: 180.06}}
	ini := ToINI(p)
	for _, frag := range []string{"[ICMS00]", "CST=00", "vBC=1500,50", "pICMS=12,00", "vICMS=180,06"} {
		if !strings.Contains(ini, frag) {
			t.Errorf("ICMS00 não contém %q\n---\n%s", frag, ini)
		}
	}
	if strings.Contains(ini, "[ICMSSN]") {
		t.Error("não deveria emitir [ICMSSN] quando ICMS00 está presente")
	}
}

func TestToINI_GruposCompletos(t *testing.T) {
	p := pedidoFixture()
	p.InfCte.Compl = &Compl{XObs: "obs teste",
		Fluxo:   &Fluxo{XOrig: "POA", XDest: "RJ", Pass: []Pass{{XPass: "Curitiba"}}},
		ObsCont: []ObsCont{{XCampo: "ped", XTexto: "12345"}}}
	p.InfCte.InfCTeNorm.InfDoc.InfNF = []InfNF{{Mod: "01", Serie: "1", NDoc: "100", NCFOP: "5353", VNF: 100}}
	p.InfCte.InfCTeNorm.InfModal.Rodo.Occ = []Occ{{Serie: "1", NOcc: 5, EmiOcc: &EmiOcc{CNPJ: "11222333000181", IE: "ISENTO", UF: "SP"}}}
	p.InfCte.InfCTeNorm.DocAnt = &DocAnt{EmiDocAnt: []EmiDocAnt{{CNPJ: "11222333000181", XNome: "Anterior",
		IdDocAnt: []IdDocAnt{{IdDocAntEle: []IdDocAntEle{{ChCTe: "35200714200166000187570010000000123456789012"}}}}}}}
	ini := ToINI(p)
	for _, frag := range []string{
		"[compl]", "xOrig=POA", "[PASS001]", "xPass=Curitiba", "[obsCont001]", "xCampo=ped",
		"[infNF001]", "nCFOP=5353", "[occ001]", "nOcc=5",
		"[emiDocAnt001]", "[idDocAntEle001001]", "chCTe=35200714200166000187570010000000123456789012",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_IBSCBS_ReformaTributaria(t *testing.T) {
	p := pedidoFixture()
	p.InfCte.Imp.IBSCBS = &TribCTe{CST: "000", CClassTrib: "200001",
		GIBSCBS: &CIBS{VBC: 1500.50, VIBS: 100.00,
			GIBSUF:  GIBSUF{PIBSUF: 5, VIBSUF: 75.00, GRed: &Red{PRedAliq: 10, PAliqEfet: 4.5}},
			GIBSMun: GIBSMun{PIBSMun: 2, VIBSMun: 25.00},
			GCBS:    GCBS{PCBS: 8, VCBS: 120.00}},
		GEstornoCred: &EstornoCred{VIBSEstCred: 10, VCBSEstCred: 12}}
	ini := ToINI(p)
	for _, frag := range []string{
		"[IBSCBS]", "CST=000", "cClassTrib=200001",
		"[gIBSCBS]", "vBC=1500,50", "vIBS=100,00",
		"[gIBSUF]", "pIBSUF=5,00", "vIBSUF=75,00", "pRedAliq=10,00", "pAliqEfet=4,50",
		"[gIBSMun]", "vIBSMun=25,00", "[gCBS]", "vCBS=120,00",
		"[gEstornoCred]", "vIBSEstCred=10,00", "vCBSEstCred=12,00",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI IBSCBS não contém %q\n---\n%s", frag, ini)
		}
	}
}

// A entrega programada é do documento, não do modal: continua coberta depois
// da poda dos modais não-rodoviários.
func TestToINI_EntregaProgramada(t *testing.T) {
	p := pedidoFixture()
	p.InfCte.Compl = &Compl{
		Entrega: &Entrega{
			ComData: &ComData{TpPer: 1, DProg: "2026-07-10"},
			ComHora: &ComHora{TpHor: 1, HProg: "14:30:00"},
		},
	}
	ini := ToINI(p)
	for _, frag := range []string{
		"[compl]", "TipoData=1", "tpPer=1", "dProg=10/07/2026",
		"TipoHora=1", "tpHor=1", "hProg=14:30:00",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestParseEnvio_Autorizado(t *testing.T) {
	resp := "[Envio]\ncStat=100\nxMotivo=Autorizado o uso do CT-e\nchCTe=35200714200166000187570010000000123456789012\nnProt=135200000123456\n"
	e := ParseEnvio(resp)
	if !e.Sucesso || StatusEmissao(e) != "autorizado" {
		t.Fatalf("esperava autorizado; got %+v", e)
	}
}

func TestParseEnvio_Rejeitado(t *testing.T) {
	resp := "[Envio]\ncStat=539\nxMotivo=Duplicidade de CT-e\n[Erro001]\nCodigo=539\nDescricao=Rejeicao: duplicidade\n"
	e := ParseEnvio(resp)
	if e.Sucesso || StatusEmissao(e) != "rejeitado" || len(e.Erros) == 0 {
		t.Errorf("esperava rejeitado com erro; got %+v", e)
	}
}

// TestToINI_TomaVaiNaSecaoPropria trava a regressão em que o indicador do
// tomador era escrito em [ide] e a lib o ignorava: o leitor faz
// ReadString('toma3','toma') e ReadString('toma4','toma'), então o valor em
// [ide] nunca chegava ao XML e o CT-e saía com <toma>0</toma>: tomador =
// Remetente, qualquer que fosse o pedido. Achado pelo teste de lockstep.
func TestToINI_TomaVaiNaSecaoPropria(t *testing.T) {
	t.Run("toma3", func(t *testing.T) {
		p := pedidoFixture()
		p.InfCte.Ide.Toma3 = &Toma3{Toma: 3}
		p.InfCte.Ide.Toma4 = nil
		ini := ToINI(p)
		if got := secaoDoINI(ini, "toma3"); !strings.Contains(got, "toma=3") {
			t.Errorf("[toma3] deveria conter toma=3, veio:\n%s", got)
		}
		if got := secaoDoINI(ini, "ide"); strings.Contains(got, "\ntoma=") {
			t.Errorf("[ide] não deve carregar 'toma' (a lib não lê de lá):\n%s", got)
		}
	})
	t.Run("toma4", func(t *testing.T) {
		p := pedidoFixture()
		p.InfCte.Ide.Toma3 = nil
		p.InfCte.Ide.Toma4 = &Toma4{Toma: 4, CNPJ: "11444777000161", XNome: "Tomador Quatro"}
		ini := ToINI(p)
		if got := secaoDoINI(ini, "toma4"); !strings.Contains(got, "toma=4") {
			t.Errorf("[toma4] deveria conter toma=4, veio:\n%s", got)
		}
	})
}

// secaoDoINI devolve só o corpo de uma seção: asserção no INI inteiro casaria
// a chave de outra seção e passaria por acidente.
func secaoDoINI(ini, secao string) string {
	i := strings.Index(ini, "["+secao+"]")
	if i < 0 {
		return ""
	}
	resto := ini[i+len(secao)+2:]
	if j := strings.Index(resto, "\n["); j >= 0 {
		return resto[:j]
	}
	return resto
}

// TestToINI_CampoSoData documenta e trava a regra dos campos que são só data
// (dEmi, dPrev, dVenc…). Eles NÃO levam fuso no XML: a lib os grava como
// AAAA-MM-DD puro, sem offset , mas a entrada continua sujeita à mesma leitura:
// com offset é um instante e é convertida para o calendário do emitente, o que
// PODE mudar o dia; sem offset é a data que o cliente escreveu.
//
// Isto mudou junto com a correção de fuso: antes o offset era descartado e
// "2026-05-02T01:00:00Z" virava 02/05 mesmo para um emitente em São Paulo, onde
// aquele instante é 22:00 do dia 01.
func TestToINI_CampoSoData(t *testing.T) {
	casos := []struct{ nome, entrada, quero string }{
		{"data pura não muda", "2026-05-02", "02/05/2026"},
		{"instante em UTC recua um dia em SP", "2026-05-02T01:00:00Z", "01/05/2026"},
		{"instante já local não muda", "2026-05-02T01:00:00-03:00", "02/05/2026"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := pedidoFixture()
			p.InfCte.Ide.CUF = 35 // São Paulo (-03:00)
			p.InfCte.InfCTeNorm.InfDoc.InfNFe[0].DPrev = c.entrada
			if got := secaoDoINI(ToINI(p), "infNFe001"); !strings.Contains(got, "dPrev="+c.quero) {
				t.Errorf("dPrev: quero %q, seção veio:\n%s", c.quero, got)
			}
		})
	}
}

// TestToINI_TpAmb tranca o campo que faltava e que a lib rejeitava com 252.
//
// Sem tpAmb no INI, a lib escreve o default (produção) no XML enquanto a sessão
// está configurada para o ambiente pedido. Como a transmissão deriva o
// ambiente do tpAmb do XML, um documento pedido em homologação acabaria
// apontando para o webservice de PRODUÇÃO.
func TestToINI_TpAmb(t *testing.T) {
	casos := map[string]struct {
		ambiente  string
		explicito int
		quero     string
	}{
		"homologação":     {"homologacao", 0, "tpAmb=2"},
		"produção":        {"producao", 0, "tpAmb=1"},
		"vazio é homolog": {"", 0, "tpAmb=2"},
		// Grafia diferente NÃO pode mudar o resultado: era por aqui que a sessão
		// ia para produção e o XML saía com tpAmb=2.
		"maiúscula":  {"PRODUCAO", 0, "tpAmb=1"},
		"com espaço": {" producao", 0, "tpAmb=1"},
		// O tpAmb explícito não vence mais nada: ele é conferido contra o
		// ambiente no handler, e contradição é 400. Coerente, dá o mesmo INI.
		"explícito coerente": {"producao", 1, "tpAmb=1"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			p := pedidoFixture()
			p.Ambiente = c.ambiente
			p.InfCte.Ide.TpAmb = c.explicito
			if ini := ToINI(p); !strings.Contains(ini, c.quero) {
				t.Errorf("INI sem %q:\n%s", c.quero, secaoIde(ini))
			}
		})
	}
}

// TestToINISimp_TpAmb: o Simplificado compartilha a [ide] e precisa do mesmo.
func TestToINISimp_TpAmb(t *testing.T) {
	p := PedidoSimp{Ambiente: "homologacao"}
	p.InfCte.Ide.Serie, p.InfCte.Ide.NCT = 1, 1
	if ini := ToINISimp(p); !strings.Contains(ini, "tpAmb=2") {
		t.Errorf("INI do Simplificado sem tpAmb=2:\n%s", secaoIde(ini))
	}
}

// secaoIde recorta a [ide] para a mensagem de erro caber.
func secaoIde(ini string) string {
	i := strings.Index(ini, "[ide]")
	if i < 0 {
		return ini
	}
	fim := strings.Index(ini[i+5:], "\n[")
	if fim < 0 {
		return ini[i:]
	}
	return ini[i : i+5+fim]
}

// TestToINI_VerProcIdentificaEsteServico: o default do campo era o nome do
// repositório privado de origem, e ele vai para o XML autorizado na SEFAZ.
// Quem não preenchesse transmitia documentos assinados como um produto alheio.
func TestToINI_VerProcIdentificaEsteServico(t *testing.T) {
	p := pedidoFixture()
	ini := ToINI(p)
	if !strings.Contains(ini, "verProc="+versao.Emissor()) {
		t.Errorf("INI sem verProc=%s:\n%s", versao.Emissor(), ini[:min(400, len(ini))])
	}
	if strings.Contains(ini, "nuvem-fiscal") {
		t.Error("o nome do projeto de origem voltou ao documento")
	}

	// O que o cliente informa continua vencendo: quem emite pode ser o
	// aplicativo dele, e há quem precise se identificar com nome próprio.
	p.InfCte.Ide.VerProc = "erp-do-cliente/2.1"
	if ini := ToINI(p); !strings.Contains(ini, "verProc=erp-do-cliente/2.1") {
		t.Error("verProc informado pelo cliente foi ignorado")
	}
}
