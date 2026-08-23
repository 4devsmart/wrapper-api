package fiscal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// --- XML que entra e sai em base64 ------------------------------------------

func TestXMLdeBase64(t *testing.T) {
	xml := `<CTe><infCte Id="CTe35"/></CTe>`

	w := httptest.NewRecorder()
	got, ok := XMLdeBase64(w, "xml_b64", "  "+Base64(xml)+"  ")
	if !ok {
		t.Fatalf("base64 válido recusado: %s", w.Body)
	}
	if got != xml {
		t.Errorf("decodificou %q", got)
	}
}

func TestXMLdeBase64_ErrosNomeiamOCampo(t *testing.T) {
	casos := map[string]string{
		"não é base64": "isto não é base64!!",
		"vazio":        "",
		"só espaço":    Base64("   \n  "),
	}
	for nome, entrada := range casos {
		t.Run(nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			if _, ok := XMLdeBase64(w, "xml_proc_b64", entrada); ok {
				t.Fatal("passou")
			}
			if w.Code != http.StatusBadRequest {
				t.Errorf("status %d, esperava 400", w.Code)
			}
			// O nome do campo vem por parâmetro justamente porque há mais de um
			// campo base64 no contrato: dizer qual deles falhou é metade da ajuda.
			if !strings.Contains(w.Body.String(), "xml_proc_b64") {
				t.Errorf("a resposta não nomeia o campo: %s", w.Body)
			}
		})
	}
}

func TestBase64_VazioContinuaVazio(t *testing.T) {
	if Base64("") != "" {
		t.Error(`Base64("") deveria ser "", para o campo omitempty sumir da resposta`)
	}
	if _, err := base64.StdEncoding.DecodeString(Base64("x")); err != nil {
		t.Errorf("Base64 não produziu base64 válido: %v", err)
	}
}

// --- bloco aninhado ---------------------------------------------------------

type eventoDeTeste struct {
	Justificativa string `json:"justificativa"`
	Protocolo     string `json:"protocolo,omitempty"`
}

// O bloco "evento" é decodificado à parte, e precisa recusar campo desconhecido
// como o nível de cima: é assim que o cliente descobre que errou um nome, em vez
// de transmitir um evento sem o dado.
func TestDecodarAninhado(t *testing.T) {
	var e eventoDeTeste
	if msg := DecodarAninhado("evento", json.RawMessage(`{"justificativa":"erro de digitacao"}`), &e); msg != "" {
		t.Fatalf("bloco válido recusado: %s", msg)
	}
	if e.Justificativa != "erro de digitacao" {
		t.Errorf("decodificou errado: %+v", e)
	}
}

func TestDecodarAninhado_BlocoAusenteEhBlocoVazio(t *testing.T) {
	var e eventoDeTeste
	if msg := DecodarAninhado("evento", nil, &e); msg != "" {
		t.Errorf("bloco ausente deveria ser tolerado (vira {}), veio: %s", msg)
	}
}

func TestDecodarAninhado_CampoDesconhecido(t *testing.T) {
	var e eventoDeTeste
	msg := DecodarAninhado("evento", json.RawMessage(`{"justificativaa":"typo"}`), &e)
	if msg == "" {
		t.Fatal("campo desconhecido passou")
	}
	for _, quero := range []string{"evento", "justificativaa"} {
		if !strings.Contains(msg, quero) {
			t.Errorf("a mensagem não menciona %q: %s", quero, msg)
		}
	}
	if strings.Contains(msg, "json: unknown field") {
		t.Errorf("a mensagem da stdlib vazou sem tradução: %s", msg)
	}
}

func TestDecodarAninhado_JSONQuebrado(t *testing.T) {
	var e eventoDeTeste
	if msg := DecodarAninhado("evento", json.RawMessage(`{"justificativa":`), &e); msg == "" {
		t.Fatal("JSON quebrado passou")
	} else if !strings.HasPrefix(msg, "evento: ") {
		t.Errorf("a mensagem precisa dizer QUAL bloco falhou: %s", msg)
	}
}

// --- chave de acesso --------------------------------------------------------

const chave44 = "35240812345678000190570010000000011000000010"

func TestChaveValida(t *testing.T) {
	w := httptest.NewRecorder()
	// Com máscara: o cliente costuma mandar a chave formatada.
	got, ok := ChaveValida(w, "3524 0812 3456 7800 0190 5700 1000 0000 0110 0000 0010")
	if !ok {
		t.Fatalf("chave com separador recusada: %s", w.Body)
	}
	if got != chave44 {
		t.Errorf("normalizou para %q", got)
	}
}

func TestChaveValida_TamanhoErrado(t *testing.T) {
	for nome, chave := range map[string]string{
		"curta": "35240812345678",
		"longa": chave44 + "9",
		"vazia": "",
		"letra": strings.Repeat("a", 44),
	} {
		t.Run(nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			if _, ok := ChaveValida(w, chave); ok {
				t.Fatal("passou")
			}
			if w.Code != http.StatusBadRequest {
				t.Errorf("status %d, esperava 400", w.Code)
			}
			if !strings.Contains(w.Body.String(), "44") {
				t.Errorf("a mensagem precisa dizer o tamanho esperado: %s", w.Body)
			}
		})
	}
}

// O CNPJ do emitente rotula a sessão nativa quando só temos o XML. Sai das
// posições 7 a 20 da chave.
func TestCNPJDaChave(t *testing.T) {
	if got := CNPJDaChave(chave44); got != "12345678000190" {
		t.Errorf("CNPJDaChave = %q", got)
	}
	if got := CNPJDaChave("3524-0812345678000190-5700100000000110000000-10"); got != "12345678000190" {
		t.Errorf("com separador: %q", got)
	}
	for _, ruim := range []string{"", "123", chave44 + "0"} {
		if got := CNPJDaChave(ruim); got != "" {
			t.Errorf("CNPJDaChave(%q) = %q, esperava vazio", ruim, got)
		}
	}
}

func TestSoDigitos(t *testing.T) {
	casos := map[string]string{
		"12.345.678/0001-90": "12345678000190",
		"  55 11 9 8888 ":    "551198888",
		"sem dígito":         "",
		"":                   "",
	}
	for entrada, quero := range casos {
		if got := SoDigitos(entrada); got != quero {
			t.Errorf("SoDigitos(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

// --- desfecho para status HTTP ----------------------------------------------

// Rejeitado é 422 e não 200: o documento do cliente não passou. O corpo vai
// completo mesmo assim, porque é com o cStat e o motivo que ele corrige.
func TestStatusDoDesfecho(t *testing.T) {
	casos := map[string]int{
		"autorizado":  http.StatusOK,
		"rejeitado":   http.StatusUnprocessableEntity,
		"erro":        http.StatusBadGateway,
		"":            http.StatusBadGateway,
		"desconúnico": http.StatusBadGateway,
	}
	for status, quero := range casos {
		if got := StatusDoDesfecho(status); got != quero {
			t.Errorf("StatusDoDesfecho(%q) = %d, quero %d", status, got, quero)
		}
	}
}

func TestStatusDoEvento(t *testing.T) {
	casos := map[string]int{
		"concluido": http.StatusOK,
		"rejeitado": http.StatusUnprocessableEntity,
		"erro":      http.StatusBadGateway,
		"pendente":  http.StatusBadGateway,
	}
	for status, quero := range casos {
		if got := StatusDoEvento(status); got != quero {
			t.Errorf("StatusDoEvento(%q) = %d, quero %d", status, got, quero)
		}
	}
}

// O evento usa "concluido" e a emissão usa "autorizado": trocar um pelo outro
// devolveria 502 num caso de sucesso.
func TestStatusDoEvento_NaoConfundeComEmissao(t *testing.T) {
	if StatusDoEvento("autorizado") == http.StatusOK {
		t.Error(`"autorizado" não é desfecho de evento`)
	}
	if StatusDoDesfecho("concluido") == http.StatusOK {
		t.Error(`"concluido" não é desfecho de emissão`)
	}
}

// --- utilidades -------------------------------------------------------------

func TestPrimeiro(t *testing.T) {
	if got := Primeiro("", "  ", "a", "b"); got != "a" {
		t.Errorf("Primeiro = %q", got)
	}
	if got := Primeiro("", "   "); got != "" {
		t.Errorf("tudo em branco deveria dar vazio, veio %q", got)
	}
	if got := Primeiro(); got != "" {
		t.Errorf("sem argumento deveria dar vazio, veio %q", got)
	}
}

func TestLinhas_DescartaVaziasENormalizaCRLF(t *testing.T) {
	got := Linhas("252 - Ambiente diverge\r\n\r\n  703 - Data de emissão   \n")
	if len(got) != 2 {
		t.Fatalf("esperava 2 linhas, veio %q", got)
	}
	if got[0] != "252 - Ambiente diverge" || got[1] != "703 - Data de emissão" {
		t.Errorf("linhas = %q", got)
	}
	if Linhas("") != nil || Linhas("  \n \r\n ") != nil {
		t.Error("texto em branco deveria dar lista vazia")
	}
}

// O StringToDateTime do ACBr não aceita fuso: a lib aplica o do estado ao gerar.
func TestAgoraLocal_NoFormatoQueALibAceita(t *testing.T) {
	got := AgoraLocal()
	if _, err := time.Parse("2006-01-02 15:04:05", got); err != nil {
		t.Errorf("AgoraLocal() = %q, fora do formato esperado: %v", got, err)
	}
}

// detalhesDaLib só aparece quando a lib devolveu algo: um bloco com
// codigo:0 e resposta vazia é ruído no envelope de erro.
func TestDetalhesDaLib_OmiteQuandoNaoHaNada(t *testing.T) {
	w := httptest.NewRecorder()
	ResponderErroDaLib(w, "CT-e", acbr.Result{}, errors.New("falhou"))
	if strings.Contains(w.Body.String(), "detalhes") {
		t.Errorf("resultado vazio não deveria produzir detalhes: %s", w.Body)
	}

	w = httptest.NewRecorder()
	ResponderErroDaLib(w, "CT-e", acbr.Result{Codigo: -10, Resposta: "CTE_CarregarINI falhou"}, errors.New("falhou"))
	for _, quero := range []string{"detalhes", "-10", "CTE_CarregarINI"} {
		if !strings.Contains(w.Body.String(), quero) {
			t.Errorf("a resposta da lib precisa chegar ao cliente (%q): %s", quero, w.Body)
		}
	}
}
