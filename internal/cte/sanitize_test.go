package cte

import (
	"strings"
	"testing"
)

// A sanitização em si vive em internal/platform/inifmt e é testada lá. Este
// teste guarda o OUTRO lado: que o builder deste pacote continue passando por
// ela. Trocar sanitizeINIVal por escrita direta compila e não quebra nada até
// alguém mandar CR/LF num campo de texto livre e forjar uma seção do INI.
func TestSanitizeINIVal_AntiInjecao(t *testing.T) {
	got := sanitizeINIVal("X\n[Secao]\nChave=valor\rmais")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("sanitizeINIVal deixou passar quebra de linha: %q", got)
	}
}
