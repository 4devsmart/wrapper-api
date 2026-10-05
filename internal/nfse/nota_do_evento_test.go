package nfse

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// Chave real de uma NFS-e do ISSNet de Brasília no leiaute nacional: número
// 17645, emitida em 10/2026.
const chaveDeBrasilia = "53001081240030122000145000000001764526101791211730"

func TestNumeroDaChaveLeAsTrezePosicoesDoNumero(t *testing.T) {
	casos := map[string]string{
		chaveDeBrasilia:             "17645",
		"https://nota.gov/link?n=1": "",
		"123":                       "",
		"5300108124003012200014500000000176452610179121173X": "",
	}
	for chave, quero := range casos {
		if got := NumeroDaChave(chave); got != quero {
			t.Errorf("NumeroDaChave(%q) = %q, quero %q", chave, got, quero)
		}
	}
}

func TestChaveDoXMLSoAceitaOIdDaNFSe(t *testing.T) {
	nfse := `<NFSe versao="1.01"><infNFSe Id="NFS` + chaveDeBrasilia + `"><nNFSe>17645</nNFSe></infNFSe></NFSe>`
	if got := ChaveDoXML(nfse); got != chaveDeBrasilia {
		t.Errorf("ChaveDoXML = %q", got)
	}
	for _, xml := range []string{`<infDPS Id="DPS530010824003012200014500003000000000017653"/>`, `<infNFSe Id="NFS123"/>`, ""} {
		if got := ChaveDoXML(xml); got != "" {
			t.Errorf("ChaveDoXML(%q) = %q, quero vazio", xml, got)
		}
	}
}

func eventoDe(t *testing.T, rec interface{ Bytes() []byte }) RespostaEvento {
	t.Helper()
	var resp RespostaEvento
	if err := json.Unmarshal(rec.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

// O grupo nfse é acréscimo: os campos de primeiro nível seguem iguais, para
// quem já os lê. O número sai da chave quando o cliente não o manda, e o RPS é
// o que o pedido informou.
func TestCancelamentoPadraoNacionalIdentificaANotaNoGrupoNfse(t *testing.T) {
	f := &libFake{resEvento: acbr.Result{
		Resposta: "[EnviarEvento]\nSucessoCanc=1\nDescSituacao=Nota Cancelada\nXmlRetorno=<procEveNFSe/>\n",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
		"chave": chaveDeBrasilia,
		"evento": map[string]any{
			"codigo": "1", "motivo": "erro na emissao da nota",
			"numero_rps": 17653, "serie_rps": "3",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	resp := eventoDe(t, rec.Body)
	if resp.Chave != chaveDeBrasilia || resp.Status != "concluido" {
		t.Errorf("primeiro nível mudou: %+v", resp)
	}
	quero := NotaDoEvento{Chave: chaveDeBrasilia, Numero: "17645", NumeroRps: "17653", SerieRps: "3"}
	if resp.NFSe == nil || *resp.NFSe != quero {
		t.Errorf("nfse = %+v, quero %+v", resp.NFSe, quero)
	}
	if resp.Substituta != nil {
		t.Errorf("cancelamento com substituta: %+v", resp.Substituta)
	}
}

// Na recusa a nota continua identificada: "já está cancelada" sem dizer qual
// nota é exatamente o que deixava o cliente sem saber o que conferir.
func TestCancelamentoRecusadoTambemIdentificaANota(t *testing.T) {
	f := &libFake{resEvento: acbr.Result{Resposta: "[EnviarEvento]\nSucessoCanc=0\n" +
		"[Erro1]\nCodigo=EM097\nDescricao=NFS-e já está cancelada.\n"}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
		"chave":  chaveDeBrasilia,
		"evento": map[string]any{"codigo": "1", "motivo": "erro na emissao da nota"},
	}))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
	}
	resp := eventoDe(t, rec.Body)
	if resp.Status != "rejeitado" {
		t.Errorf("status = %q, quero rejeitado", resp.Status)
	}
	if resp.NFSe == nil || resp.NFSe.Numero != "17645" || resp.NFSe.Chave != chaveDeBrasilia {
		t.Errorf("nfse = %+v", resp.NFSe)
	}
}

// No ABRASF o primeiro nível leva o link, e o grupo separa link e número.
func TestCancelamentoAbrasfIdentificaANotaPeloNumero(t *testing.T) {
	f := &libFake{resCancelar: acbr.Result{
		Resposta: "[Cancelamento]\nSucesso=1\nProtocolo=P9\n", XML: "<evento/>",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munAbrasf, map[string]any{
		"chave": "https://prefeitura/nota/100",
		"evento": map[string]any{
			"codigo": "1", "motivo": "erro na emissao", "numero": "100",
			"codigo_verificacao": "XYZ", "numero_rps": 55, "serie_rps": "A",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	quero := NotaDoEvento{
		Chave: "https://prefeitura/nota/100", Numero: "100", CodigoVerificacao: "XYZ",
		NumeroRps: "55", SerieRps: "A",
	}
	if resp := eventoDe(t, rec.Body); resp.NFSe == nil || *resp.NFSe != quero {
		t.Errorf("nfse = %+v, quero %+v", resp.NFSe, quero)
	}
}

// A substituição devolve as duas notas. A chave da substituta sai do XML
// quando a lib não preenche o Link, que é o caso do ISSNet de Brasília, e o
// chave de primeiro nível, que antes vinha vazio, passa a vir com ela.
func TestSubstituicaoIdentificaAsDuasNotas(t *testing.T) {
	novaChave := "53001081240030122000145000000001765026101791211735"
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[Envio]\nSucesso=1\nNumeroNota=17650\n",
		XML: `<NFSe><infNFSe Id="NFS` + novaChave + `"><DPS><infDPS Id="DPS530010824003012200014500001000000000000001">` +
			`</infDPS></DPS></infNFSe></NFSe>`,
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munPadraoNacional, map[string]any{
		"evento": map[string]any{
			"dps":         pedidoMinimo(munPadraoNacional),
			"substituida": map[string]any{"chave": chaveDeBrasilia},
			"codigo":      "05",
			"motivo":      "rejeitada pelo tomador",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	resp := eventoDe(t, rec.Body)
	if resp.Chave != novaChave {
		t.Errorf("chave = %q, quero a da substituta %q", resp.Chave, novaChave)
	}
	antiga := NotaDoEvento{Chave: chaveDeBrasilia, Numero: "17645"}
	if resp.NFSe == nil || *resp.NFSe != antiga {
		t.Errorf("nfse = %+v, quero %+v", resp.NFSe, antiga)
	}
	nova := NotaDoEvento{
		Chave: novaChave, Numero: "17650",
		IDDPS:     "DPS530010824003012200014500001000000000000001",
		NumeroRps: "1", SerieRps: "1",
	}
	if resp.Substituta == nil || *resp.Substituta != nova {
		t.Errorf("substituta = %+v, quero %+v", resp.Substituta, nova)
	}
}

func TestSubstituicaoRecusadaNaoInventaSubstituta(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{
		Resposta: "[Envio]\nSucesso=0\n[Erro1]\nCodigo=E1\nDescricao=recusada\n",
	}}
	rec := post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munAbrasf, map[string]any{
		"evento": map[string]any{
			"dps":         pedidoMinimo(munAbrasf),
			"substituida": map[string]any{"numero": "100", "codigo_verificacao": "XYZ"},
			"motivo":      "erro de valor",
		},
	}))
	resp := eventoDe(t, rec.Body)
	if resp.Substituta != nil {
		t.Errorf("substituta numa recusa: %+v", resp.Substituta)
	}
	if resp.NFSe == nil || resp.NFSe.Numero != "100" || resp.NFSe.CodigoVerificacao != "XYZ" {
		t.Errorf("nfse = %+v", resp.NFSe)
	}
}

func TestJaCanceladaReconheceAsFormasDaRecusa(t *testing.T) {
	casos := map[Mensagem]bool{
		{Codigo: "EM097", Descricao: "NFS-e já está cancelada."}: true,
		{Codigo: "em097"}: true,
		{Codigo: "E79", Descricao: "Nota fiscal já cancelada"}:                true,
		{Codigo: "X1", Descricao: "A NFS-e ja se encontra cancelada"}:         true,
		{Codigo: "X2", Descricao: "NFS-e encontra-se cancelada"}:              true,
		{Codigo: "X3", Descricao: "Nota JÁ FOI CANCELADA anteriormente"}:      true,
		{Codigo: "E1", Descricao: "Prazo de cancelamento expirado"}:           false,
		{Codigo: "E2", Descricao: "Nota não pode ser cancelada"}:              false,
		{Codigo: "E3", Descricao: "Já existe pedido de cancelamento"}:         false,
		{Codigo: "E4", Descricao: "NFS-e substituída não pode ser cancelada"}: false,
	}
	for m, quero := range casos {
		if got := JaCancelada([]Mensagem{m}); got != quero {
			t.Errorf("JaCancelada(%+v) = %v, quero %v", m, got, quero)
		}
	}
}

// O status continua rejeitado, com o mesmo HTTP: o campo novo só acrescenta.
func TestCancelamentoJaCanceladoMarcaSemMudarOStatus(t *testing.T) {
	casos := map[string]struct {
		f      *libFake
		mun    string
		evento map[string]any
		chave  string
	}{
		"padrão nacional, EM097": {
			f: &libFake{resEvento: acbr.Result{Resposta: "[EnviarEvento]\nSucessoCanc=0\n" +
				"[Erro1]\nCodigo=EM097\nDescricao=NFS-e já está cancelada.\n"}},
			mun: munPadraoNacional, chave: chaveDeBrasilia,
			evento: map[string]any{"codigo": "1", "motivo": "erro na emissao da nota"},
		},
		"abrasf, E79": {
			f: &libFake{resCancelar: acbr.Result{Resposta: "[CancelarNFSe]\n" +
				"[Erro1]\nCodigo=E79\nDescricao=Nota fiscal já cancelada\n" +
				"[RetCancelamento]\nSucesso=\n"}},
			mun:    munAbrasf,
			evento: map[string]any{"codigo": "1", "motivo": "erro na emissao", "numero": "86"},
		},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			rec := post(t, muxDe(c.f), "/nfse/eventos/cancelamento", envelope(c.mun, map[string]any{
				"chave": c.chave, "evento": c.evento,
			}))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, quero 422: %s", rec.Code, rec.Body)
			}
			resp := eventoDe(t, rec.Body)
			if resp.Status != "rejeitado" || !resp.NFSeJaCancelada {
				t.Errorf("status = %q, nfse_ja_cancelada = %v", resp.Status, resp.NFSeJaCancelada)
			}
		})
	}
}

// Outra recusa não ganha o campo, e o JSON fica byte a byte como era.
func TestCancelamentoRecusadoPorOutroMotivoNaoMarca(t *testing.T) {
	f := &libFake{resEvento: acbr.Result{Resposta: "[EnviarEvento]\nSucessoCanc=0\n" +
		"[Erro1]\nCodigo=E1\nDescricao=Prazo de cancelamento expirado\n"}}
	rec := post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munPadraoNacional, map[string]any{
		"chave":  chaveDeBrasilia,
		"evento": map[string]any{"codigo": "1", "motivo": "erro na emissao da nota"},
	}))
	var bruto map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &bruto)
	if _, tem := bruto["nfse_ja_cancelada"]; tem {
		t.Errorf("nfse_ja_cancelada numa recusa comum: %s", rec.Body)
	}
}

// A transmissão completa a chave pelo XML quando a lib não preenche o Link; o
// que a lib manda continua valendo, e recusa não inventa chave.
func TestTransmissaoCompletaAChavePeloXML(t *testing.T) {
	nfse := `<NFSe><infNFSe Id="NFS` + chaveDeBrasilia + `"></infNFSe></NFSe>`
	casos := map[string]struct {
		resposta, quero string
	}{
		"lib sem Link": {"[Envio]\nSucesso=1\nNumeroNota=17645\n", chaveDeBrasilia},
		"lib com Link": {"[Envio]\nSucesso=1\nLink=da-lib\n", "da-lib"},
		"recusa":       {"[Envio]\nSucesso=0\n[Erro1]\nCodigo=E1\nDescricao=x\n", ""},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: c.resposta, XML: nfse}}
			rec := post(t, muxDe(f), "/nfse/transmissao", envelope(munPadraoNacional, map[string]any{
				"xml_b64": base64.StdEncoding.EncodeToString([]byte(xmlFixture("2"))),
			}))
			var resp RespostaTransmissao
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.Chave != c.quero {
				t.Errorf("chave = %q, quero %q", resp.Chave, c.quero)
			}
		})
	}
}

func TestLoteAutorizadoTrazAChaveDoXML(t *testing.T) {
	f := &libFake{resLote: acbr.Result{
		Resposta: "[ConsultaLoteRps]\nSituacao=4\nProtocolo=9000001\n[Arquivo1]\nNumeroNota=17645\n",
		XML:      `<NFSe><infNFSe Id="NFS` + chaveDeBrasilia + `"></infNFSe></NFSe>`,
	}}
	rec := post(t, muxDe(f), "/nfse/transmissao/lote", envelope(munAbrasf, map[string]any{"protocolo": "9000001"}))
	var resp RespostaTransmissao
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "autorizado" || resp.Chave != chaveDeBrasilia {
		t.Errorf("resposta = %+v", resp)
	}
}
