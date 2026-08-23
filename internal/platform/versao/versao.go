// Package versao expõe a identidade do binário EM EXECUÇÃO, carimbada no build.
//
// Por que existe: as versões que o painel mostrava vinham de variáveis de
// ambiente (APP_VERSION/CLI_VERSION) escritas pelo instalador. Elas dizem o que
// o instalador PRETENDIA subir, não o que subiu. Um APP_IMAGE apontando para o
// registry errado (ou um pull que não trocou a imagem) deixava o painel
// anunciando uma versão nova com a imagem antiga rodando, sem nenhum sinal.
//
// O carimbo abaixo vem do próprio binário (ldflags), então não tem como mentir:
// se o container não trocou, o commit não muda.
package versao

import "strings"

// Commit e Build são preenchidos no link:
//
//	-ldflags "-X github.com/4devsmart/wrapper-api/internal/platform/versao.Commit=<sha>"
//
// Vazio = binário compilado fora do pipeline (dev local sem carimbo).
var (
	Commit string
	Build  string
)

// Info é o retrato do binário em execução.
type Info struct {
	Commit string `json:"commit,omitempty"`       // sha completo do commit buildado
	Curto  string `json:"commit_curto,omitempty"` // 7 primeiros caracteres
	Build  string `json:"build,omitempty"`        // data/hora do build (RFC3339)
}

// Atual devolve o carimbo do binário.
func Atual() Info {
	c := strings.TrimSpace(Commit)
	return Info{Commit: c, Curto: Curto(), Build: strings.TrimSpace(Build)}
}

// Curto é o commit abreviado: é o que se compara com a tag da imagem no
// registry (o Deploy publica a imagem também com o sha como tag).
func Curto() string {
	c := strings.TrimSpace(Commit)
	if len(c) > 7 {
		return c[:7]
	}
	return c
}
