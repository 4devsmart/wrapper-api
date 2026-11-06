package nfse

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// chaveNacionalDeBrasilia é uma nota de Brasília gerada pelo Sistema Nacional: a
// posição 8 da chave é o ambiente gerador, 1 na prefeitura e 2 no nacional.
var chaveNacionalDeBrasilia = chaveDeBrasilia[:7] + "2" + chaveDeBrasilia[8:]

// pedidoSimples é a nota de Brasília do Simples, na competência e no regime dados.
func pedidoSimples(t *testing.T, dCompet string, opSimpNac int) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(pedidoBrasiliaSN), &p); err != nil {
		t.Fatal(err)
	}
	inf := p["infDPS"].(map[string]any)
	inf["dCompet"] = dCompet
	inf["prest"].(map[string]any)["regTrib"].(map[string]any)["opSimpNac"] = opSimpNac
	return p
}

// A fronteira é a competência, não a emissão, e só vale para ME/EPP em município
// listado. Quem não é do Simples tem de continuar no provedor: é por isso que a
// troca não pode ser feita na tabela.
func TestPeloAmbienteNacional(t *testing.T) {
	casos := []struct {
		nome, cmun string
		op         int
		dCompet    string
		quero      bool
	}{
		{"ME/EPP de Brasília em novembro", munAPIPropria, 3, "2026-11-05", true},
		{"primeiro dia da regra", munAPIPropria, 3, "2026-11-01", true},
		{"último dia antes da regra", munAPIPropria, 3, "2026-10-31", false},
		{"competência com hora", munAPIPropria, 3, "2026-11-01T00:00:00-03:00", true},
		{"não optante", munAPIPropria, 1, "2026-11-05", false},
		{"MEI", munAPIPropria, 2, "2026-11-05", false},
		{"regime ausente", munAPIPropria, 0, "2026-11-05", false},
		{"sem competência", munAPIPropria, 3, "", false},
		{"código com pontuação", "53.00108", 3, "2026-11-05", true},
		{"município fora da lista", "5208707", 3, "2026-11-05", false},
		{"município do Padrão Nacional", munPadraoNacional, 3, "2026-11-05", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := PeloAmbienteNacional(c.cmun, c.op, c.dCompet); got != c.quero {
				t.Errorf("= %v, quero %v", got, c.quero)
			}
		})
	}
}

// A sessão muda de município, o documento não: o cLocEmi é o que vai para a
// chave da DPS, e é assim que a nota continua sendo de Brasília.
func TestXMLDoSimplesAbreSessaoNacional(t *testing.T) {
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	rec := post(t, muxDe(f), "/nfse/xml", pedidoSimples(t, "2026-11-05", 3))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != MunicipioADN {
		t.Errorf("sessão em %q, quero %q", got, MunicipioADN)
	}
	if !strings.Contains(f.iniMontar, "cLocEmi=5300108") {
		t.Errorf("o INI perdeu o município da nota:\n%s", f.iniMontar)
	}
	if !strings.Contains(rec.Body.String(), `"provedor":"PadraoNacional"`) {
		t.Errorf("a resposta tem de dizer por onde a nota sai: %s", rec.Body)
	}
}

func TestXMLForaDaRegraContinuaNoProvedor(t *testing.T) {
	casos := map[string]map[string]any{
		"outubro":     pedidoSimples(t, "2026-10-31", 3),
		"não optante": pedidoSimples(t, "2026-11-05", 1),
	}
	for nome, p := range casos {
		t.Run(nome, func(t *testing.T) {
			f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
			rec := post(t, muxDe(f), "/nfse/xml", p)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != munAPIPropria {
				t.Errorf("sessão em %q, quero %q", got, munAPIPropria)
			}
			if !strings.Contains(rec.Body.String(), `"provedor":"ISSNet"`) {
				t.Errorf("provedor errado: %s", rec.Body)
			}
		})
	}
}

// O cTribMun muda de regra junto com o caminho: obrigatório e inteiro no ISSNet,
// opcional e com 3 posições no gravador nacional.
func TestCTribMunNoAmbienteNacional(t *testing.T) {
	casos := []struct {
		nome, cTribMun string
		quero          int
	}{
		{"3 dígitos", "601", http.StatusOK},
		{"ausente", "", http.StatusOK},
		{"4 dígitos", "0601", http.StatusBadRequest},
		{"2 dígitos", "61", http.StatusBadRequest},
		{"texto", "6.1", http.StatusBadRequest},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := pedidoSimples(t, "2026-11-05", 3)
			p["infDPS"].(map[string]any)["serv"].(map[string]any)["cTribMun"] = c.cTribMun
			f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
			if rec := post(t, muxDe(f), "/nfse/xml", p); rec.Code != c.quero {
				t.Errorf("status = %d, quero %d: %s", rec.Code, c.quero, rec.Body)
			}
		})
	}
}

func xmlDeBrasilia(dCompet, opSimpNac string) string {
	return `<?xml version="1.0"?><DPS xmlns="http://www.sped.fazenda.gov.br/nfse"><infDPS Id="DPS5300108">` +
		`<tpAmb>2</tpAmb><dCompet>` + dCompet + `</dCompet><cLocEmi>5300108</cLocEmi>` +
		`<prest><regTrib><opSimpNac>` + opSimpNac + `</opSimpNac></regTrib></prest></infDPS></DPS>`
}

// A transmissão só recebe a DPS pronta, e o cliente continua mandando o município
// de sempre: regime e competência saem do documento, e as credenciais da
// prefeitura não vão para o Ambiente Nacional.
func TestTransmissaoDecidePeloXML(t *testing.T) {
	casos := []struct {
		nome, dCompet, op, quero string
	}{
		{"Simples em novembro", "2026-11-05", "3", MunicipioADN},
		{"Simples em outubro", "2026-10-31", "3", munAPIPropria},
		{"não optante", "2026-11-05", "1", munAPIPropria},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\nNumeroNota=1\n", XML: "<NFSe/>"}}
			xml := xmlDeBrasilia(c.dCompet, c.op)
			rec := post(t, muxDe(f), "/nfse/transmissao", envelope(munAPIPropria, map[string]any{
				"xml_b64":     base64.StdEncoding.EncodeToString([]byte(xml)),
				"credenciais": map[string]any{"usuario": "u", "senha": "s"},
			}))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
			if f.xmlTransmitido != xml {
				t.Error("o XML não pode mudar no caminho")
			}
			if tem := cfgDaSessao(f.sessao, "Emitente.WSUser") != ""; tem != (c.quero == munAPIPropria) {
				t.Errorf("credenciais da prefeitura na sessão = %v com sessão em %s", tem, c.quero)
			}
		})
	}
}

// Notas dos dois sistemas convivem no mesmo município, e só a chave diz onde
// cada uma está. Notas do provedor têm de continuar saindo por ele.
func TestRotaDaNota(t *testing.T) {
	casos := []struct {
		nome, cmun, chave, quero string
	}{
		{"nota do Sistema Nacional", munAPIPropria, chaveNacionalDeBrasilia, MunicipioADN},
		{"nota da prefeitura", munAPIPropria, chaveDeBrasilia, munAPIPropria},
		{"sem chave", munAPIPropria, "", munAPIPropria},
		{"link no lugar da chave", munAPIPropria, "https://nfse.exemplo/2", munAPIPropria},
		{"município fora da lista", munPadraoNacional, chaveNacionalDeBrasilia, munPadraoNacional},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r, ok := RotaDoMunicipio(c.cmun)
			if !ok {
				t.Fatalf("município %s sem rota", c.cmun)
			}
			r = r.DaNota(c.chave)
			if r.Sessao != c.quero {
				t.Errorf("sessão = %q, quero %q", r.Sessao, c.quero)
			}
			if r.Municipio != c.cmun {
				t.Errorf("o município do documento mudou para %q", r.Municipio)
			}
		})
	}
}

func TestEventosEConsultasSeguemAChave(t *testing.T) {
	casos := []struct {
		nome, chave, quero string
	}{
		{"nota nacional", chaveNacionalDeBrasilia, MunicipioADN},
		{"nota da prefeitura", chaveDeBrasilia, munAPIPropria},
	}
	for _, c := range casos {
		t.Run("cancelamento/"+c.nome, func(t *testing.T) {
			f := &libFake{resEvento: acbr.Result{Resposta: "[Evento]\nSucesso=1\n"}}
			post(t, muxDe(f), "/nfse/eventos/cancelamento", envelope(munAPIPropria, map[string]any{
				"chave":  c.chave,
				"evento": map[string]any{"codigo": "1", "motivo": "erro na emissão da nota"},
			}))
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
		})
		t.Run("consulta/"+c.nome, func(t *testing.T) {
			f := &libFake{resConsulta: acbr.Result{Resposta: "ok"}}
			post(t, muxDe(f), "/nfse/consulta", envelope(munAPIPropria, map[string]any{"chave": c.chave}))
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
		})
		t.Run("pdf por chave/"+c.nome, func(t *testing.T) {
			f := &libFake{resConsulta: acbr.Result{PDF: []byte("%PDF")}}
			post(t, muxDe(f), "/nfse/pdf", envelope(munAPIPropria, map[string]any{"chave": c.chave}))
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
		})
		t.Run("pdf por XML/"+c.nome, func(t *testing.T) {
			f := &libFake{resConsulta: acbr.Result{PDF: []byte("%PDF")}}
			xml := `<NFSe><infNFSe Id="NFS` + c.chave + `"><cLocEmi>5300108</cLocEmi></infNFSe></NFSe>`
			post(t, muxDe(f), "/nfse/pdf", map[string]any{"xml_b64": base64.StdEncoding.EncodeToString([]byte(xml))})
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
		})
	}
}

// Na substituição a nota nova manda: em novembro o ISSNet recusa o Simples com
// EM110, então não há outro caminho, mesmo com a antiga na prefeitura.
func TestSubstituicaoPelaNotaNova(t *testing.T) {
	novembro, outubro := pedidoSimples(t, "2026-11-05", 3), pedidoSimples(t, "2026-10-20", 3)
	casos := []struct {
		nome  string
		dps   map[string]any
		chave string
		quero string
	}{
		{"nova em novembro, antiga na prefeitura", novembro, chaveDeBrasilia, MunicipioADN},
		{"nova em outubro, antiga na prefeitura", outubro, chaveDeBrasilia, munAPIPropria},
		{"nova em outubro, antiga no nacional", outubro, chaveNacionalDeBrasilia, MunicipioADN},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=1\n", XML: "<NFSe/>"}}
			post(t, muxDe(f), "/nfse/eventos/substituicao", envelope(munAPIPropria, map[string]any{
				"evento": map[string]any{
					"dps":         c.dps,
					"substituida": map[string]any{"chave": c.chave, "numero": "17645"},
					"codigo":      "99", "motivo": "correção dos dados da nota",
				},
			}))
			if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != c.quero {
				t.Errorf("sessão em %q, quero %q", got, c.quero)
			}
		})
	}
}

// A chave da DPS não diz o sistema. O nacional só encerra a busca se achou a
// nota: uma resposta vazia dele não pode esconder a que está na prefeitura.
func TestConsultaDPSProcuraNosDoisSistemas(t *testing.T) {
	const idDPS = "DPS530010824003012200014500003000000000017653"
	t.Run("achou no nacional", func(t *testing.T) {
		f := &libFake{resConsulta: acbr.Result{Resposta: "[Consulta]\nChave=" + chaveNacionalDeBrasilia + "\n"}}
		post(t, muxDe(f), "/nfse/consulta-dps", envelope(munAPIPropria, map[string]any{"chave": idDPS}))
		if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != MunicipioADN {
			t.Errorf("sessão em %q, quero %q", got, MunicipioADN)
		}
	})
	t.Run("não achou no nacional", func(t *testing.T) {
		f := &libFake{resConsulta: acbr.Result{Resposta: "[Consulta]\nSucesso=0\n"}}
		post(t, muxDe(f), "/nfse/consulta-dps", envelope(munAPIPropria, map[string]any{"chave": idDPS}))
		if got := cfgDaSessao(f.sessao, "CodigoMunicipio"); got != munAPIPropria {
			t.Errorf("a busca terminou em %q, quero %q", got, munAPIPropria)
		}
	})
}

// Só município com dois caminhos procura em dois lugares, e a ordem importa: uma
// resposta vazia do nacional não pode esconder a nota que está na prefeitura.
func TestSessoesDaConsultaDPS(t *testing.T) {
	casos := map[string][]string{
		munAPIPropria:     {MunicipioADN, munAPIPropria},
		munPadraoNacional: {munPadraoNacional},
		munAbrasf:         {munAbrasf},
	}
	for cmun, quero := range casos {
		r, ok := RotaDoMunicipio(cmun)
		if !ok {
			t.Fatalf("município %s sem rota", cmun)
		}
		if got := r.SessoesDaConsultaDPS(); !slices.Equal(got, quero) {
			t.Errorf("%s: sessões = %v, quero %v", cmun, got, quero)
		}
	}
}

func TestRotaDoMunicipioDesconhecido(t *testing.T) {
	if _, ok := RotaDoMunicipio(munDesconhecido); ok {
		t.Error("município sem provedor não pode ter rota")
	}
}
