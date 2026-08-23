package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Configuração é promessa. Uma variável lida no boot e nunca consultada promete
// um comportamento que não existe, e quem opera o serviço não tem como
// descobrir a diferença: ela aparece no .env.example, aceita valor, e não faz
// nada. Foi assim que API_RATE_PER_MIN prometeu um limite por IP que ninguém
// aplicava. Estes dois testes fecham as duas direções da deriva.

const raiz = "../../../"

// campos lidos por Load, extraídos do próprio fonte: é a lista autoritativa.
var reChave = regexp.MustCompile(`env(?:Int|Bool|List)?\("([A-Z_0-9]+)"`)

func chavesLidas(t *testing.T) map[string]bool {
	t.Helper()
	fonte, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	chaves := map[string]bool{}
	for _, m := range reChave.FindAllStringSubmatch(string(fonte), -1) {
		chaves[m[1]] = true
	}
	if len(chaves) < 10 {
		t.Fatalf("só %d chaves encontradas no fonte; a extração quebrou", len(chaves))
	}
	return chaves
}

// TestEnvExampleNaoOmiteNemInventa mantém o .env.example honesto: ele diz ser a
// lista completa, e uma lista completa que omite variável é pior que nenhuma.
func TestEnvExampleNaoOmiteNemInventa(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join(raiz, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	// Aceita tanto "CHAVE=valor" quanto "# CHAVE=" (documentada e desligada).
	reCitada := regexp.MustCompile(`(?m)^#?\s*([A-Z][A-Z_0-9]+)=`)
	citadas := map[string]bool{}
	for _, m := range reCitada.FindAllStringSubmatch(string(bruto), -1) {
		citadas[m[1]] = true
	}

	// HOST_PORT é do compose, não do binário: publica a porta no host.
	deCompose := map[string]bool{"HOST_PORT": true}

	for chave := range chavesLidas(t) {
		if !citadas[chave] {
			t.Errorf("%s é lida por Load e não aparece no .env.example, que se anuncia completo", chave)
		}
	}
	for chave := range citadas {
		if !chavesLidas(t)[chave] && !deCompose[chave] {
			t.Errorf("%s está no .env.example e ninguém lê: ou some, ou passa a valer", chave)
		}
	}
}

// TestTodoCampoDeConfigTemConsumidor recusa campo que existe só para ser
// preenchido. Não basta Load atribuir: alguém fora daqui precisa ler.
func TestTodoCampoDeConfigTemConsumidor(t *testing.T) {
	fontes := fontesForaDaqui(t)

	for _, campo := range camposDe(reflect.TypeOf(Config{}), reflect.TypeOf(ACBr{})) {
		if !strings.Contains(fontes, "."+campo) {
			t.Errorf("Config.%s é preenchido no boot e ninguém consome: "+
				"a variável de ambiente promete um comportamento que não existe", campo)
		}
	}
}

func camposDe(tipos ...reflect.Type) []string {
	var nomes []string
	for _, tp := range tipos {
		for i := range tp.NumField() {
			if c := tp.Field(i); c.Type.Kind() != reflect.Struct {
				nomes = append(nomes, c.Name)
			}
		}
	}
	return nomes
}

// fontesForaDaqui junta todo o Go do repositório menos este pacote: consumir a
// si mesmo não conta como consumir.
func fontesForaDaqui(t *testing.T) string {
	t.Helper()
	var sb strings.Builder
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(raiz, dir), func(caminho string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir(), !strings.HasSuffix(caminho, ".go"):
				return nil
			case strings.Contains(filepath.ToSlash(caminho), "platform/config/"):
				return nil
			}
			b, err := os.ReadFile(caminho)
			sb.Write(b)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if sb.Len() == 0 {
		t.Fatal("nenhum fonte lido; o caminho da raiz mudou")
	}
	return sb.String()
}

// TestComposeRepassaOQueOEnvExemplifica fecha a terceira direção da deriva. O
// compose lê o .env só para expandir ${...}: variável que ele não repassa no
// bloco environment não chega ao container. Quem edita o .env vê o valor lá e
// não vê efeito nenhum, que é o mesmo sintoma da config morta, com uma causa
// diferente e mais difícil de achar.
func TestComposeRepassaOQueOEnvExemplifica(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(raiz, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	bruto, err := os.ReadFile(filepath.Join(raiz, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}

	// Só as ATIVAS: uma linha comentada documenta o nome e diz que o default da
	// imagem serve, e é assim que ficam os caminhos que ninguém mexe.
	reAtiva := regexp.MustCompile(`(?m)^([A-Z][A-Z_0-9]+)=`)
	for _, m := range reAtiva.FindAllStringSubmatch(string(bruto), -1) {
		if !strings.Contains(string(compose), "${"+m[1]) {
			t.Errorf("%s está ativa no .env.example e o compose não a repassa: "+
				"quem editar o .env não vai ver efeito nenhum", m[1])
		}
	}
}
