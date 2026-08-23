package nfse

import (
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// pedidoSintetico monta um DPSPedido com dados ANONIMIZADOS (CNPJ/endereço
// fictícios): nunca dados reais de contribuinte. Espelha o caminho de emissão
// do Padrão Nacional para exercitar ToINI.
func pedidoSintetico() DPSPedido {
	return DPSPedido{
		Ambiente: "homologacao",
		InfDPS: InfDPS{
			Serie: "1", NDPS: "100", DCompet: "2026-05-01", CLocEmi: "4314902",
			Prest: Pessoa{
				CNPJ: "11111111000191", XNome: "Empresa Teste LTDA",
				CMun: "4314902", UF: "RS", CEP: "90000000",
				Logradouro: "Rua Exemplo", Numero: "100", Bairro: "Centro",
				Telefone: "5133330000", Email: "teste@exemplo.com",
				RegTrib: &RegTrib{OpSimpNac: 3, RegApTribSN: 1},
			},
			Serv:    Servico{CServ: "010101", XDescServ: "Serviço de teste", CMunPrestacao: "4314902"},
			Valores: Valores{VServ: 1234.5, TribISSQN: 1, PAliq: 2},
		},
	}
}

func TestToINI_SecoesEChaves(t *testing.T) {
	ini := ToINI(pedidoSintetico())

	deveConter := []string{
		"[IdentificacaoNFSe]", "[IdentificacaoRps]", "[Prestador]",
		"[Servico]", "[Valores]", "[tribMun]",
		"Numero=100", "Serie=1", "cLocEmi=4314902",
		"CNPJ=11111111000191", "RazaoSocial=Empresa Teste LTDA",
		"Logradouro=Rua Exemplo", "Numero=100", "Bairro=Centro",
		"CEP=90000000", "Telefone=5133330000", "Email=teste@exemplo.com",
		"opSimpNac=3", "RegimeApuracaoSN=1",
		"ItemListaServico=010101", "Discriminacao=Serviço de teste",
	}
	for _, s := range deveConter {
		if !strings.Contains(ini, s) {
			t.Errorf("INI não contém %q\n---\n%s", s, ini)
		}
	}
}

// money() usa vírgula decimal: "1234.50" seria lido como zero pelo ACBr.
func TestToINI_ValorComVirgula(t *testing.T) {
	ini := ToINI(pedidoSintetico())
	if !strings.Contains(ini, "ValorServicos=1234,50") {
		t.Errorf("ValorServicos deveria usar vírgula (1234,50); INI:\n%s", ini)
	}
	if strings.Contains(ini, "ValorServicos=1234.50") {
		t.Error("ValorServicos não deve usar ponto decimal")
	}
}

// O Padrão Nacional rejeita IM no <prest> (E0120): se IM vazio, não escrever.
func TestToINI_SemInscricaoMunicipalNoPrestadorQuandoVazia(t *testing.T) {
	ini := ToINI(pedidoSintetico())
	if strings.Contains(ini, "InscricaoMunicipal=") {
		t.Errorf("não deve emitir InscricaoMunicipal vazia no prestador; INI:\n%s", ini)
	}
}

func TestToINI_ReformaETributacaoCompleta(t *testing.T) {
	p := pedidoSintetico()
	p.InfDPS.Valores.VDescIncond = 10
	p.InfDPS.Valores.VDeducoes = 20
	p.InfDPS.Valores.TribMun = &TribMun{TribISSQN: 1, TpRetISSQN: 1, PAliq: 3.5, TpImunidade: 0}
	p.InfDPS.Valores.TribFed = &TribFed{CST: "01", VBCPisCofins: 1000, PAliqPis: 0.65, VPis: 6.5}
	p.InfDPS.Valores.TotTrib = &TotTrib{IndTotTrib: 1, VTotTribFed: 36.5, VTotTribMun: 35}
	p.InfDPS.IBSCBS = &IBSCBSDPS{
		GIBSCBS: &GIBSCBSDPS{CST: "000", CClassTrib: "000001",
			GTribRegular: &GTribRegularDPS{CSTReg: "000", CClassTribReg: "000001"},
			GDif:         &GDifDPS{PDifUF: 5, PDifMun: 2, PDifCBS: 8}},
	}
	ini := ToINI(p)
	for _, frag := range []string{
		"DescontoIncondicionado=10,00", "ValorDeducoes=20,00",
		"[tribMun]", "tpRetISSQN=1", "pAliq=3,50",
		"[tribFed]", "CST=01", "vPis=6,50",
		"[totTrib]", "indTotTrib=1", "vTotTribFed=36,50",
		// IBS/CBS: finNFSe/indDest com default neutro (a lib estoura no vazio).
		"[IBSCBSDPS]", "finNFSe=0", "indDest=0",
		"[gIBSCBS]", "CST=000", "cClassTrib=000001",
		"[gTribRegular]", "CSTReg=000", "[gDif]", "pDifCBS=8,00",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_ServicoIntermComExtInfoCompl(t *testing.T) {
	p := pedidoSintetico()
	p.InfDPS.Interm = &Pessoa{CNPJ: "11222333000181", XNome: "Intermediario X", CMun: "4314902", UF: "RS"}
	p.InfDPS.Serv.CNBS = "123456789"
	p.InfDPS.Serv.MunIncidencia = "4314902"
	p.InfDPS.Serv.ComExt = &ComExt{MdPrestacao: "1", TpMoeda: 790, VServMoeda: 1000, NDI: "DI-1"}
	p.InfDPS.Serv.InfoCompl = &InfoCompl{DocRef: "DOC-1", XInfComp: "obs", GItemPed: []string{"item A"}}
	p.InfDPS.IBSCBS = &IBSCBSDPS{
		Dest:    &DestIBS{CNPJ: "44555666000172", XNome: "Dest X", CMun: "4314902"},
		Imovel:  &ImovelIBS{InscImobFisc: "IMO-1", CCIB: "CIB-1"},
		GIBSCBS: &GIBSCBSDPS{CST: "000"},
	}
	ini := ToINI(p)
	for _, frag := range []string{
		"[Intermediario]", "RazaoSocial=Intermediario X",
		"CodigoNBS=123456789", "MunicipioIncidencia=4314902",
		"[ComercioExterior]", "mdPrestacao=1", "vServMoeda=1000,00", "nDI=DI-1",
		"[InformacoesComplementares]", "docRef=DOC-1", "[gItemPed01]", "xItemPed=item A",
		"[Destinatario]", "RazaoSocial=Dest X", "[Imovel]", "inscImobFisc=IMO-1", "cCIB=CIB-1",
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_DocumentosReeRepRes(t *testing.T) {
	p := pedidoSintetico()
	p.InfDPS.IBSCBS = &IBSCBSDPS{
		GIBSCBS: &GIBSCBSDPS{CST: "000"},
		Documentos: []DocReeRep{
			{TipoChaveDFe: "2", ChaveDFe: "35200714200166000187550010000000123456789010", DtEmiDoc: "2026-05-10", TpReeRepRes: "01", VlrReeRepRes: 50},
			{NDoc: "REC-1", XDoc: "Recibo", DtEmiDoc: "2026-05-11"}, // sem tipoChaveDFe → default "4"
		},
	}
	ini := ToINI(p)
	for _, frag := range []string{
		"[Documentos0001]", "tipoChaveDFe=2", "chaveDFe=35200714200166000187550010000000123456789010",
		"tpReeRepRes=01", "vlrReeRepRes=50,00", "dtEmiDoc=10/05/2026",
		"[Documentos0002]", "tipoChaveDFe=4", "xDoc=Recibo", // default Outro p/ doc sem chave
	} {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINICancelamento(t *testing.T) {
	chave := "43149022203780307000140000000000001326055875359766"

	// Código default = 1 quando não informado.
	ini := ToINICancelamento(chave, "4314902", CancelamentoPedido{})
	for _, s := range []string{"[CancelarNFSe]", "ChaveNFSe=" + chave, "CodCancelamento=1", "CodMunicipio=4314902"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI cancelamento não contém %q\n%s", s, ini)
		}
	}

	// Código e motivo informados.
	ini2 := ToINICancelamento(chave, "", CancelamentoPedido{Codigo: "2", Motivo: "Serviço não prestado"})
	if !strings.Contains(ini2, "CodCancelamento=2") || !strings.Contains(ini2, "MotCancelamento=Serviço não prestado") {
		t.Errorf("código/motivo não refletidos:\n%s", ini2)
	}
	if strings.Contains(ini2, "CodMunicipio=") {
		t.Error("CodMunicipio não deve aparecer quando vazio")
	}
}

// A sanitização em si vive em internal/platform/inifmt e é testada lá. Este
// teste guarda o OUTRO lado: que o builder deste pacote continue passando por
// ela. Trocar sanitizeINIVal por escrita direta compila e não quebra nada até
// alguém mandar CR/LF num campo de texto livre e forjar uma seção do INI.
func TestSanitizeINIVal_AntiInjecao(t *testing.T) {
	malicioso := "Serviço X\n[Emitente]\nCNPJ=00000000000191\nAmbiente=1"
	got := sanitizeINIVal(malicioso)
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("sanitizeINIVal deixou passar quebra de linha: %q", got)
	}
	if strings.Contains(got, "\n[Emitente]") {
		t.Fatalf("injeção de seção não neutralizada: %q", got)
	}
}

// TestToINI_VerAplicIdentificaEsteServico: o default do campo era o nome do
// repositório privado de origem, e ele acompanha a DPS enviada ao ADN. Quem não
// preenchesse transmitia documentos identificados como um produto alheio.
func TestToINI_VerAplicIdentificaEsteServico(t *testing.T) {
	p := pedidoSintetico()
	ini := ToINI(p)
	if !strings.Contains(ini, "verAplic="+versao.Emissor()) {
		t.Errorf("INI sem verAplic=%s", versao.Emissor())
	}
	if strings.Contains(ini, "nuvem-fiscal") {
		t.Error("o nome do projeto de origem voltou ao documento")
	}

	p.InfDPS.VerAplic = "erp-do-cliente/2.1"
	if ini := ToINI(p); !strings.Contains(ini, "verAplic=erp-do-cliente/2.1") {
		t.Error("verAplic informado pelo cliente foi ignorado")
	}
}
