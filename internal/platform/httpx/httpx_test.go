package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pedido struct {
	Chave  string `json:"chave"`
	Numero int    `json:"numero"`
}

func lerCorpo(t *testing.T, corpo string, limite int64) (*httptest.ResponseRecorder, pedido, bool) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(corpo))
	if limite > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limite)
	}
	var p pedido
	return w, p, LerJSON(w, r, &p)
}

func envelope(t *testing.T, w *httptest.ResponseRecorder) Erro {
	t.Helper()
	var env struct {
		Erro Erro `json:"erro"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("resposta não é o envelope de erro: %s", w.Body)
	}
	return env.Erro
}

func TestLerJSON_CorpoValido(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"chave":"abc","numero":7}`))
	var p pedido
	if !LerJSON(w, r, &p) {
		t.Fatalf("corpo válido recusado: %s", w.Body)
	}
	if p.Chave != "abc" || p.Numero != 7 {
		t.Errorf("decodificou errado: %+v", p)
	}
}

// Campo desconhecido é recusado de propósito: num contrato fiscal, um nome
// errado que o servidor ignora vira documento transmitido com dado faltando, e
// isso só aparece depois de o fisco autorizar.
func TestLerJSON_CampoDesconhecidoEhRecusado(t *testing.T) {
	w, _, ok := lerCorpo(t, `{"chave":"abc","chavee":"typo"}`, 0)
	if ok {
		t.Fatal("campo desconhecido passou")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, esperava 400", w.Code)
	}
	e := envelope(t, w)
	if e.Codigo != "json_invalido" {
		t.Errorf("codigo = %q", e.Codigo)
	}
	// A mensagem tem de NOMEAR o campo: é o que o cliente usa para corrigir.
	if !strings.Contains(e.Mensagem, `"chavee"`) {
		t.Errorf("a mensagem não nomeia o campo errado: %q", e.Mensagem)
	}
	if strings.Contains(e.Mensagem, "json: unknown field") {
		t.Errorf("a mensagem da stdlib vazou sem tradução: %q", e.Mensagem)
	}
}

func TestLerJSON_TipoErradoNomeiaOCampo(t *testing.T) {
	w, _, ok := lerCorpo(t, `{"numero":"não é número"}`, 0)
	if ok {
		t.Fatal("tipo errado passou")
	}
	e := envelope(t, w)
	if !strings.Contains(e.Mensagem, "numero") {
		t.Errorf("a mensagem não nomeia o campo: %q", e.Mensagem)
	}
	for _, esperado := range []string{"esperava", "veio"} {
		if !strings.Contains(e.Mensagem, esperado) {
			t.Errorf("a mensagem não diz o que esperava e o que veio: %q", e.Mensagem)
		}
	}
}

// Um segundo objeto no corpo quase sempre é cliente montando o pedido errado.
// Sem esta checagem, o segundo era silenciosamente descartado.
func TestLerJSON_SegundoObjetoNoCorpo(t *testing.T) {
	w, _, ok := lerCorpo(t, `{"chave":"a"}{"chave":"b"}`, 0)
	if ok {
		t.Fatal("dois objetos no corpo passaram")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, esperava 400", w.Code)
	}
	if !strings.Contains(envelope(t, w).Mensagem, "único objeto") {
		t.Errorf("a mensagem não explica o problema: %q", envelope(t, w).Mensagem)
	}
}

// O corpo carrega certificado e XML: sem teto, é exaustão de memória barata.
// O status precisa ser 413, não 400: o cliente não errou o formato.
func TestLerJSON_CorpoAcimaDoLimiteEh413(t *testing.T) {
	grande := `{"chave":"` + strings.Repeat("x", 500) + `"}`
	w, _, ok := lerCorpo(t, grande, 64)
	if ok {
		t.Fatal("corpo acima do limite passou")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d, esperava 413", w.Code)
	}
	if !strings.Contains(envelope(t, w).Mensagem, "MAX_BODY_BYTES") {
		t.Errorf("a mensagem não diz qual limite ajustar: %q", envelope(t, w).Mensagem)
	}
}

func TestLerJSON_CorpoVazioEMalFormado(t *testing.T) {
	for nome, corpo := range map[string]string{
		"vazio":        ``,
		"chave solta":  `{"chave"}`,
		"não é objeto": `[1,2,3]`,
	} {
		t.Run(nome, func(t *testing.T) {
			w, _, ok := lerCorpo(t, corpo, 0)
			if ok {
				t.Fatalf("corpo %q passou", corpo)
			}
			if w.Code != http.StatusBadRequest {
				t.Errorf("status %d, esperava 400", w.Code)
			}
		})
	}
}

// --- envelope ---------------------------------------------------------------

// Formato único em toda rota é o que permite ao cliente escrever UM tratamento.
func TestEnvelopeDeErro(t *testing.T) {
	w := httptest.NewRecorder()
	ErroJSON(w, http.StatusUnprocessableEntity, "regras_de_negocio", "não passou")

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d", w.Code)
	}
	var bruto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &bruto); err != nil {
		t.Fatal(err)
	}
	erro, ok := bruto["erro"].(map[string]any)
	if !ok {
		t.Fatalf("o erro precisa vir aninhado em \"erro\": %s", w.Body)
	}
	if erro["codigo"] != "regras_de_negocio" || erro["mensagem"] != "não passou" {
		t.Errorf("envelope = %v", erro)
	}
	if _, tem := erro["detalhes"]; tem {
		t.Error("detalhes vazio não deve aparecer no JSON")
	}
}

func TestErroDetalhado_LevaOCorpoAuxiliar(t *testing.T) {
	w := httptest.NewRecorder()
	ErroDetalhado(w, http.StatusBadGateway, "falha_na_lib", "a lib falhou",
		map[string]any{"codigo": -10, "resposta": "CTE_CarregarINI falhou"})

	e := envelope(t, w)
	d, ok := e.Detalhes.(map[string]any)
	if !ok {
		t.Fatalf("detalhes não chegou: %s", w.Body)
	}
	if d["resposta"] != "CTE_CarregarINI falhou" {
		t.Errorf("detalhes = %v", d)
	}
}

func TestJSON_EscreveStatusEContentType(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusCreated, map[string]string{"status": "ok"})

	if w.Code != http.StatusCreated {
		t.Errorf("status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("corpo = %s", w.Body)
	}
}
