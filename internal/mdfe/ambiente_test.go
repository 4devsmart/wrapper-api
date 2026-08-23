package mdfe

import (
	"net/http"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// O ambiente do pedido vai para dois lugares com convenções opostas: o ordinal
// da sessão nativa (0 = produção) e o tpAmb do XML (1 = produção). Aqui havia
// duas falhas na mesma decisão: normalizações diferentes nos dois caminhos, e o
// tpAmb explícito de infMDFe.ide sendo aceito no contrato e descartado ao montar
// o INI. Quem pedisse produção pelo tpAmb recebia homologação com resposta 200.

func geracaoCom(t *testing.T, ambiente string, tpAmb int) (*libFake, int, string) {
	t.Helper()
	f := &libFake{resMontar: acbr.Result{XML: xmlFixture("2")}}
	p := pedidoMinimo()
	p["ambiente"] = ambiente
	p["infMDFe"].(map[string]any)["ide"].(map[string]any)["tpAmb"] = tpAmb
	rec := post(t, muxDe(f), "/mdfe/xml", p)
	return f, rec.Code, rec.Body.String()
}

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
		"maiúsculas":           {"PRODUCAO", "0", "tpAmb=1"},
		"capitalizado":         {"Producao", "0", "tpAmb=1"},
		"espaço na hora":       {" producao", "0", "tpAmb=1"},
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
	for _, amb := range []string{"prod", "production", "homolog"} {
		t.Run(amb, func(t *testing.T) {
			_, code, corpo := geracaoCom(t, amb, 0)
			if code != http.StatusBadRequest {
				t.Fatalf("HTTP %d para ambiente=%q, esperava 400", code, amb)
			}
			if !strings.Contains(corpo, "ambiente_invalido") {
				t.Errorf("resposta sem código acionável: %s", corpo)
			}
		})
	}
}

// TestAmbiente_TpAmbExplicitoDeixaDeSerIgnorado é a regressão do caso pior: o
// campo existia no contrato, aparecia na spec e nunca chegava ao INI. Contradizer
// o ambiente agora é 400; concordar segue emitindo.
func TestAmbiente_TpAmbExplicitoDeixaDeSerIgnorado(t *testing.T) {
	t.Run("contradiz", func(t *testing.T) {
		_, code, corpo := geracaoCom(t, "homologacao", 1)
		if code != http.StatusBadRequest {
			t.Fatalf("HTTP %d: ambiente=homologacao com tpAmb=1 precisa ser recusado, não ignorado", code)
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

// --- escopo: só o modal rodoviário ------------------------------------------

// O contrato deixou de aceitar aéreo, aquaviário e ferroviário. Um pedido que
// traga o grupo é 400 campo desconhecido; um que declare o modal sem o grupo é
// 422, porque sairia um MDF-e com modal declarado e nenhum dado de modal.
func TestEscopo_ApenasModalRodoviario(t *testing.T) {
	mux := muxDe(&libFake{resMontar: acbr.Result{XML: xmlFixture("2")}})
	for _, modal := range []int{2, 3, 4} {
		p := pedidoMinimo()
		p["infMDFe"].(map[string]any)["ide"].(map[string]any)["modal"] = modal
		rec := post(t, mux, "/mdfe/xml", p)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("modal=%d: HTTP %d, esperava 422", modal, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "modal_nao_suportado") {
			t.Errorf("modal=%d: resposta sem código acionável: %s", modal, rec.Body)
		}
	}
	if rec := post(t, mux, "/mdfe/xml", pedidoMinimo()); rec.Code != http.StatusOK {
		t.Errorf("rodoviário recusado: HTTP %d %s", rec.Code, rec.Body)
	}
}

func TestEscopo_GrupoDeModalForaDoContrato(t *testing.T) {
	mux := muxDe(&libFake{resMontar: acbr.Result{XML: xmlFixture("2")}})
	for _, grupo := range []string{"aereo", "aquav", "ferrov"} {
		p := pedidoMinimo()
		p["infMDFe"].(map[string]any)["infModal"] = map[string]any{
			"versaoModal": "3.00", grupo: map[string]any{},
		}
		rec := post(t, mux, "/mdfe/xml", p)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("infModal.%s: HTTP %d, esperava 400", grupo, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "campo desconhecido") {
			t.Errorf("infModal.%s: resposta sem a explicação: %s", grupo, rec.Body)
		}
	}
}
