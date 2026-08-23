package cte

import (
	"strings"
	"testing"
)

// TestToINICancelamento_FormatoIndexado garante o formato que o ACBr LerFromIni
// realmente lê: [EVENTO001] (indexado), NUNCA [evento] singular.
func TestToINICancelamento_FormatoIndexado(t *testing.T) {
	chave := "35240812345678000190570010000000011000000010"
	ini := ToINICancelamento(chave, "12345678000190", "135240000000001", "2026-05-31 10:00:00",
		PedidoCancelamento{Justificativa: "cancelamento de teste homologacao"})
	must := []string{"[EVENTO]", "[EVENTO001]", "chCTe=" + chave, "cOrgao=35",
		"CNPJ=12345678000190", "tpEvento=110111", "nProt=135240000000001", "xJust=cancelamento"}
	for _, s := range must {
		if !strings.Contains(ini, s) {
			t.Errorf("INI de cancelamento sem %q\n%s", s, ini)
		}
	}
	if strings.Contains(ini, "[evento]") || strings.Contains(ini, "[detEvento]") {
		t.Errorf("INI usa formato SINGULAR (não lido pelo LerFromIni):\n%s", ini)
	}
}

// TestToINICartaCorrecao_DetEvento garante CC-e (110110) + [DETEVENTO001].
func TestToINICartaCorrecao_DetEvento(t *testing.T) {
	ini := ToINICartaCorrecao("35240812345678000190570010000000011000000010", "12345678000190", "2026-05-31 10:00:00",
		PedidoCartaCorrecao{Correcoes: []Correcao{
			{GrupoAlterado: "ide", CampoAlterado: "xMun", ValorAlterado: "SAO PAULO", NroItemAlterado: 0},
		}})
	must := []string{"tpEvento=110110", "[DETEVENTO001]", "grupoAlterado=ide",
		"campoAlterado=xMun", "valorAlterado=SAO PAULO", "nroItemAlterado=0"}
	for _, s := range must {
		if !strings.Contains(ini, s) {
			t.Errorf("INI da CC-e sem %q\n%s", s, ini)
		}
	}
}

// TestToINIEPEC_FormatoIndexado: EPEC (110113) com [EVENTO001] + [TOMADOR] + float vírgula.
func TestToINIEPEC_FormatoIndexado(t *testing.T) {
	ini := ToINIEPEC("35240812345678000190570010000000011000000010", "12345678000190", "2026-05-31 10:00:00",
		PedidoEPEC{VICMS: 12.5, VTPrest: 100, VCarga: 5000, UFIni: "SP", UFFim: "RJ", DhEmi: "2026-05-31 09:00:00",
			Tomador: TomadorEPEC{Toma: "3", UF: "RJ", CNPJCPF: "99999999000191"}})
	for _, s := range []string{"tpEvento=110113", "[EVENTO001]", "vICMS=12,50", "vTPrest=100,00",
		"vCarga=5000,00", "modal=01", "UFIni=SP", "[TOMADOR]", "toma=3", "CNPJCPF=99999999000191"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI EPEC sem %q\n%s", s, ini)
		}
	}
}

// TestToINIComprovanteEntrega: 110180 com [infEntrega0001] + coord vírgula.
func TestToINIComprovanteEntrega(t *testing.T) {
	ini := ToINIComprovanteEntrega("35240812345678000190570010000000011000000010", "12345678000190", "2026-05-31 10:00:00",
		PedidoComprovanteEntrega{NProt: "135240000000001", XNome: "RECEBEDOR", Latitude: -23.5505, Longitude: -46.6333,
			Documentos: []string{"35200711111111000111550010000000011000000017"}})
	for _, s := range []string{"tpEvento=110180", "[EVENTO001]", "nProt=135240000000001",
		"latitude=-23,5505", "[infEntrega0001]", "chNFe=35200711111111000111550010000000011000000017"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI comprovante sem %q\n%s", s, ini)
		}
	}
}

// TestToINIInsucessoEntrega: 110190 com tpMotivo default + infEntrega.
func TestToINIInsucessoEntrega(t *testing.T) {
	ini := ToINIInsucessoEntrega("35240812345678000190570010000000011000000010", "12345678000190", "2026-05-31 10:00:00",
		PedidoInsucessoEntrega{NTentativa: 2, XJustMotivo: "ausente", Documentos: []string{"35200711111111000111550010000000011000000017"}})
	for _, s := range []string{"tpEvento=110190", "[EVENTO001]", "nTentativa=2", "tpMotivo=1",
		"xJustMotivo=ausente", "[infEntrega0001]", "chNFe=35200711111111000111550010000000011000000017"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI insucesso sem %q\n%s", s, ini)
		}
	}
}

// TestToINIDesacordo: 610110 com xObs.
func TestToINIDesacordo(t *testing.T) {
	ini := ToINIDesacordo("35240812345678000190570010000000011000000010", "12345678000190", "2026-05-31 10:00:00",
		PedidoDesacordo{XObs: "carga divergente do contratado"})
	for _, s := range []string{"tpEvento=610110", "[EVENTO001]", "xObs=carga divergente do contratado"} {
		if !strings.Contains(ini, s) {
			t.Errorf("INI desacordo sem %q\n%s", s, ini)
		}
	}
}
