package cte

import (
	"net/http"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/fiscal"
)

// O ambiente do pedido alimenta DOIS destinos com convenções opostas: o ordinal
// da sessão nativa (0 = produção) e o tpAmb do XML (1 = produção). Enquanto cada
// um normalizava por conta própria, "Producao" configurava a sessão para
// produção e escrevia tpAmb=2 no documento. A SEFAZ chama isso de rejeição 252,
// e o desfecho ruim é o outro: um documento de homologação chegando ao
// webservice de produção.
//
// Os testes abaixo cobram o alinhamento pelo handler, que é onde o cliente entra.

func geracaoCom(t *testing.T, ambiente string, tpAmb int) (*libFake, int, string) {
	t.Helper()
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	corpo := map[string]any{
		"ambiente": ambiente,
		"infCte": map[string]any{
			"emit": map[string]any{"CNPJ": "12345678000190"},
			"ide":  map[string]any{"tpAmb": tpAmb, "CFOP": "5353", "natOp": "Transporte"},
		},
	}
	rec := post(t, muxDe(f), "/cte/xml", corpo)
	return f, rec.Code, rec.Body.String()
}

// ordinalDaSessao lê o Ambiente que foi configurado na sessão nativa.
func ordinalDaSessao(t *testing.T, tn acbr.TenantConfig) string {
	t.Helper()
	for _, kv := range tn.Config {
		if kv.Key == "Ambiente" {
			return kv.Value
		}
	}
	t.Fatal("a sessão nativa subiu sem a chave Ambiente")
	return ""
}

func TestAmbiente_SessaoEXMLNuncaDivergem(t *testing.T) {
	casos := map[string]struct{ ambiente, ordinal, tpAmb string }{
		"canônico produção":    {"producao", "0", "tpAmb=1"},
		"canônico homologação": {"homologacao", "1", "tpAmb=2"},
		"omitido":              {"", "1", "tpAmb=2"},
		// As três grafias que produziam a divergência.
		"maiúsculas":     {"PRODUCAO", "0", "tpAmb=1"},
		"capitalizado":   {"Producao", "0", "tpAmb=1"},
		"espaço na hora": {" producao", "0", "tpAmb=1"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f, code, corpo := geracaoCom(t, c.ambiente, 0)
			if code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", code, corpo)
			}
			if got := ordinalDaSessao(t, f.tenantMontar); got != c.ordinal {
				t.Errorf("sessão nativa com Ambiente=%s, esperava %s", got, c.ordinal)
			}
			if !strings.Contains(f.iniMontar, c.tpAmb) {
				t.Errorf("INI sem %q; sessão e documento discordam de novo", c.tpAmb)
			}
		})
	}
}

func TestAmbiente_ValorDesconhecidoEhErroDoCliente(t *testing.T) {
	for _, amb := range []string{"prod", "production", "homolog", "1"} {
		t.Run(amb, func(t *testing.T) {
			_, code, corpo := geracaoCom(t, amb, 0)
			if code != http.StatusBadRequest {
				t.Fatalf("HTTP %d para ambiente=%q: valor não reconhecido virando homologação em silêncio é como se emite sem valor fiscal", code, amb)
			}
			if !strings.Contains(corpo, "ambiente_invalido") {
				t.Errorf("resposta sem código acionável: %s", corpo)
			}
		})
	}
}

// TestAmbiente_TpAmbExplicitoNaoPodeContradizer cobre o outro caminho para a
// mesma divergência: infCte.ide.tpAmb é escrito no XML, o campo ambiente
// configura a sessão. Antes o explícito vencia sozinho, e ninguém avisava a
// sessão. Agora ou os dois concordam, ou é 400.
func TestAmbiente_TpAmbExplicitoNaoPodeContradizer(t *testing.T) {
	t.Run("contradiz", func(t *testing.T) {
		_, code, corpo := geracaoCom(t, "homologacao", 1)
		if code != http.StatusBadRequest {
			t.Fatalf("HTTP %d: ambiente=homologacao com tpAmb=1 precisa ser recusado", code)
		}
		if !strings.Contains(corpo, "ambiente_divergente") {
			t.Errorf("resposta sem código acionável: %s", corpo)
		}
	})

	t.Run("concorda", func(t *testing.T) {
		f, code, corpo := geracaoCom(t, "producao", 1)
		if code != http.StatusOK {
			t.Fatalf("HTTP %d: coerente tem de passar: %s", code, corpo)
		}
		if got := ordinalDaSessao(t, f.tenantMontar); got != "0" {
			t.Errorf("sessão com Ambiente=%s, esperava 0 (produção)", got)
		}
		if !strings.Contains(f.iniMontar, "tpAmb=1") {
			t.Error("INI sem tpAmb=1")
		}
	})
}

// TestAmbiente_TransmissaoSegueOXML confirma que a transmissão continua tirando
// o ambiente do documento, e não do campo do cliente: é o XML que já foi
// montado que manda.
func TestAmbiente_TransmissaoSegueOXML(t *testing.T) {
	f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\ncStat=100\n"}}
	post(t, muxDe(f), "/cte/transmissao", map[string]any{
		"xml_b64":     fiscal.Base64(xmlFixture("1")), // produção no documento
		"ambiente":    "homologacao",                  // fallback, ignorado
		"certificado": certValido(),
	})
	if got := ordinalDaSessao(t, f.tenantTransmitir); got != "0" {
		t.Errorf("sessão com Ambiente=%s; o tpAmb do XML (produção) é que manda", got)
	}
}

// --- escopo: só o modal rodoviário ------------------------------------------

// O contrato deixou de aceitar aéreo, aquaviário, ferroviário, dutoviário e
// multimodal. Um pedido que traga o grupo é 400 campo desconhecido; um que
// declare o modal sem o grupo é 422, porque sairia um CT-e com modal declarado
// e nenhum dado de modal.
func TestEscopo_ApenasModalRodoviario(t *testing.T) {
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	mux := muxDe(f)
	corpo := func(modal string) map[string]any {
		return map[string]any{
			"ambiente": "homologacao",
			"infCte": map[string]any{
				"emit": map[string]any{"CNPJ": "12345678000190"},
				"ide":  map[string]any{"modal": modal, "CFOP": "5353", "natOp": "Transporte"},
			},
		}
	}

	for _, modal := range []string{"02", "03", "04", "05", "06"} {
		rec := post(t, mux, "/cte/xml", corpo(modal))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("modal=%s: HTTP %d, esperava 422", modal, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "modal_nao_suportado") {
			t.Errorf("modal=%s: resposta sem código acionável: %s", modal, rec.Body)
		}
	}
	if rec := post(t, mux, "/cte/xml", corpo("01")); rec.Code != http.StatusOK {
		t.Errorf("rodoviário recusado: HTTP %d %s", rec.Code, rec.Body)
	}
}

// TestEscopo_GrupoDeModalForaDoContrato: o campo sumiu do modelo, então quem
// mandar recebe o 400 de campo desconhecido em vez de um documento gerado sem
// ele. É o desfecho que o DisallowUnknownFields existe para dar.
func TestEscopo_GrupoDeModalForaDoContrato(t *testing.T) {
	mux := muxDe(&libFake{resMontar: acbr.Result{XML: xmlFixture("2")}})
	for _, grupo := range []string{"aereo", "aquav", "ferrov", "duto", "multimodal"} {
		corpo := `{"ambiente":"homologacao","infCte":{"emit":{"CNPJ":"12345678000190"},` +
			`"infCTeNorm":{"infModal":{"` + grupo + `":{}}}}}`
		rec := postCru(t, mux, "/cte/xml", corpo)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("infModal.%s: HTTP %d, esperava 400", grupo, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "campo desconhecido") {
			t.Errorf("infModal.%s: resposta sem a explicação: %s", grupo, rec.Body)
		}
	}
}
