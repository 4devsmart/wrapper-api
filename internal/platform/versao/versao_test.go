package versao

import "testing"

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
