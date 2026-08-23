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

// Aplicativo é o nome deste serviço. Vai no campo que identifica QUEM gerou o
// documento fiscal: verProc no CT-e e no MDF-e, verAplic na NFS-e.
const Aplicativo = "wrapper-api"

// maxEmissor é o limite dos três layouts para esse campo (minLength 1,
// maxLength 20, conferido nos XSD do CT-e 4.00, do MDF-e 3.00 e do Padrão
// Nacional 1.01). Estourar aqui é rejeição de schema.
const maxEmissor = 20

// Emissor identifica este serviço no documento fiscal quando o cliente não
// informa o seu próprio verProc/verAplic.
//
// O commit entra junto porque é isso que o campo serve para responder: quando
// uma rejeição vier de um erro de montagem, o XML autorizado diz qual build o
// produziu. Sem carimbo (build local), sobra só o nome.
//
// O cliente sempre pode sobrescrever: quem emite é o aplicativo dele, e há
// quem precise se identificar com nome próprio perante o fisco.
func Emissor() string {
	e := Aplicativo
	if c := Curto(); c != "" {
		e += "/" + c
	}
	if len(e) > maxEmissor {
		return e[:maxEmissor]
	}
	return e
}
