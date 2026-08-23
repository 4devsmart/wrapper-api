package versao

import (
	"strings"
	"testing"
)

func TestCurto(t *testing.T) {
	casos := map[string]string{
		"ec3dd662918c9e70ec72e782a0ab63a16cf403bd": "ec3dd66",
		"abc": "abc",
		"":    "",
	}
	for sha, want := range casos {
		Commit = sha
		if got := Curto(); got != want {
			t.Errorf("Curto(%q) = %q, quero %q", sha, got, want)
		}
	}
}

// Sem carimbo (dev local), os campos vêm vazios em vez de mentir uma versão.
func TestSemCarimbo(t *testing.T) {
	Commit, Build = "", ""
	if v := Atual(); v.Commit != "" || v.Curto != "" || v.Build != "" {
		t.Errorf("binário sem carimbo deveria devolver campos vazios, veio %+v", v)
	}
}

func TestEmissor(t *testing.T) {
	casos := map[string]string{
		"ec3dd662918c9e70ec72e782a0ab63a16cf403bd": "wrapper-api/ec3dd66",
		"": "wrapper-api",
	}
	for sha, want := range casos {
		Commit = sha
		if got := Emissor(); got != want {
			t.Errorf("Emissor() com commit %q = %q, quero %q", sha, got, want)
		}
	}
}

// TestEmissorCabeNoLayout: o campo é minLength 1, maxLength 20 nos três
// layouts. Passar disso é rejeição de schema, e o valor é um default nosso:
// o cliente não teria como perceber antes de transmitir.
func TestEmissorCabeNoLayout(t *testing.T) {
	for _, sha := range []string{"", "abc", "ec3dd662918c9e70ec72e782a0ab63a16cf403bd", strings.Repeat("f", 64)} {
		Commit = sha
		e := Emissor()
		if len(e) < 1 || len(e) > 20 {
			t.Errorf("Emissor() = %q, %d caracteres: fora de [1,20]", e, len(e))
		}
	}
}
