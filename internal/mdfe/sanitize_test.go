package mdfe

import (
	"strings"
	"testing"
)

// TestSanitizeINIVal_AntiInjecao protege a cópia de sanitização deste pacote:
// CR/LF em campo de texto livre do cliente não pode injetar chave/seção no INI.
func TestSanitizeINIVal_AntiInjecao(t *testing.T) {
	got := sanitizeINIVal("X\n[Secao]\nChave=valor\rmais")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("sanitizeINIVal deixou passar quebra de linha: %q", got)
	}
}
