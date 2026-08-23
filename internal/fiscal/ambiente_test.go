package fiscal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- ambiente: um valor, dois códigos, nunca em desacordo -------------------

// TestAmbiente_OrdinalETpAmbConcordam cobre o segundo defeito: o ordinal da
// sessão e o tpAmb do XML saíam de funções diferentes, com normalizações
// diferentes. "Producao" abria a sessão em produção e escrevia tpAmb=2 no
// documento: rejeição 252 na melhor hipótese.
func TestAmbiente_OrdinalETpAmbConcordam(t *testing.T) {
	producao := []string{"producao", "Producao", "PRODUCAO", " producao ", "PrOdUcAo"}
	homologacao := []string{"", "homologacao", "Homologacao", "HOMOLOGACAO", " homologacao "}

	for _, amb := range producao {
		if o, tp := AmbienteOrdinal(amb), TpAmb(amb); o != "0" || tp != "1" {
			t.Errorf("%q: sessão=%s tpAmb=%s, esperava produção nos dois (0 e 1)", amb, o, tp)
		}
	}
	for _, amb := range homologacao {
		if o, tp := AmbienteOrdinal(amb), TpAmb(amb); o != "1" || tp != "2" {
			t.Errorf("%q: sessão=%s tpAmb=%s, esperava homologação nos dois (1 e 2)", amb, o, tp)
		}
	}
}

func TestNormalizarAmbiente_RecusaOQueNaoReconhece(t *testing.T) {
	for _, amb := range []string{"prod", "production", "hom", "producao1", "teste"} {
		if _, ok := NormalizarAmbiente(amb); ok {
			t.Errorf("%q foi aceito; valor não reconhecido tem de ser recusado, não virar homologação", amb)
		}
	}
	for _, amb := range []string{"", "producao", "homologacao"} {
		if _, ok := NormalizarAmbiente(amb); !ok {
			t.Errorf("%q deveria ser aceito", amb)
		}
	}
}

func TestAmbienteValido_RespondeErroDoCliente(t *testing.T) {
	w := httptest.NewRecorder()
	if _, ok := AmbienteValido(w, "prod"); ok {
		t.Fatal(`"prod" não pode passar`)
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, esperava 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ambiente_invalido") {
		t.Errorf("o código do erro precisa ser acionável: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	amb, ok := AmbienteValido(w, "  PRODUCAO ")
	if !ok || amb != AmbienteProducao {
		t.Errorf("esperava %q normalizado, veio %q (ok=%v)", AmbienteProducao, amb, ok)
	}
}
