package fiscal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Este serviço não persiste nada, e por isso LOG é a única forma realista de um
// segredo escapar do processo: o certificado A1 de terceiros, a senha dele, as
// credenciais de webservice do banco e do município. A defesa é o tipo redigir
// a si mesmo, e a defesa só vale se ninguém esquecer de aplicá-la ao próximo
// tipo que nascer carregando segredo.
//
// Este teste varre o repositório e cobra isso. Ele mora aqui porque é em
// fiscal que a redação do certificado nasceu, e é o pacote comum aos
// documentos; a varredura, porém, é de todo o internal/.
//
// Só LogValue conta. String() cobre fmt, e é necessário, mas NÃO protege o
// caminho que importa: o serviço loga em JSON, e o manipulador JSON do slog não
// consulta Stringer, ele serializa a struct campo a campo. Foi exatamente assim
// que nfse.Credenciais pareceu protegida por meses sem estar.

// nomeDeSegredo casa campo que carrega credencial. Conservador de propósito:
// um falso positivo vira uma linha na lista de exceções, e um falso negativo
// vira segredo no log.
//
// Casa o NOME do campo, nunca o comentário: detectar pelo comentário faria
// apagar um comentário desligar a checagem daquele tipo, em silêncio.
var nomeDeSegredo = regexp.MustCompile(`(?i)senha|password|secret|pfx|token|certkey|privkey|chaveprivada|credencial|clientid|apikey|\bcsrt\b`)

// semSegredo são os campos que casam com o padrão e NÃO são credencial. Cada
// linha é uma decisão registrada, com o porquê.
var semSegredo = map[string]string{}

// campo é uma folha suspeita encontrada na varredura.
type campo struct{ pacote, tipo, nome string }

func (c campo) String() string { return c.pacote + "." + c.tipo + "." + c.nome }

var (
	reTipo  = regexp.MustCompile(`(?m)^type (\w+) struct \{\n((?:.*\n)*?)\}`)
	reCampo = regexp.MustCompile(`^\s*([A-Z]\w*)\s`)
)

func TestTodoTipoComSegredoRedigeASiMesmo(t *testing.T) {
	raiz := "../.."
	porTipo := map[campo]bool{}
	temLogValue := map[string]bool{}

	err := filepath.WalkDir(filepath.Join(raiz, "internal"), func(caminho string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir(), !strings.HasSuffix(caminho, ".go"), strings.HasSuffix(caminho, "_test.go"):
			return nil
		}
		b, err := os.ReadFile(caminho)
		if err != nil {
			return err
		}
		fonte := string(b)
		pacote := filepath.Base(filepath.Dir(caminho))

		for _, m := range regexp.MustCompile(`func \(\w+ (\w+)\) LogValue\(\)`).FindAllStringSubmatch(fonte, -1) {
			temLogValue[pacote+"."+m[1]] = true
		}
		for _, m := range reTipo.FindAllStringSubmatch(fonte, -1) {
			for _, linha := range strings.Split(m[2], "\n") {
				// Fora a tag e o comentário: o que sobra é "Nome Tipo", e é só
				// no nome que a detecção pode confiar.
				decl, _, _ := strings.Cut(linha, "`")
				if i := strings.Index(decl, "//"); i >= 0 {
					decl = decl[:i]
				}
				nm := reCampo.FindStringSubmatch(decl)
				if nm == nil || !nomeDeSegredo.MatchString(nm[1]) {
					continue
				}
				porTipo[campo{pacote, m[1], nm[1]}] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(porTipo) < 5 {
		t.Fatalf("só %d campos suspeitos encontrados; a varredura quebrou", len(porTipo))
	}

	var desprotegidos, excecoesMortas []string
	usada := map[string]bool{}
	for c := range porTipo {
		if motivo, ok := semSegredo[c.String()]; ok {
			usada[c.String()] = true
			_ = motivo
			continue
		}
		if !temLogValue[c.pacote+"."+c.tipo] {
			desprotegidos = append(desprotegidos, fmt.Sprintf("%s.%s carrega %s", c.pacote, c.tipo, c.nome))
		}
	}
	for k := range semSegredo {
		if !usada[k] {
			excecoesMortas = append(excecoesMortas, k)
		}
	}

	if len(desprotegidos) > 0 {
		sort.Strings(desprotegidos)
		t.Errorf("%d tipo(s) carregam segredo e não redigem a si mesmos.\n\n"+
			"Acrescente String() e LogValue() ao tipo (ver fiscal.Certificado), ou, se o\n"+
			"campo não é credencial, registre em semSegredo com o motivo. Só LogValue\n"+
			"protege o log JSON, que é o do serviço.\n\n  %s",
			len(desprotegidos), strings.Join(desprotegidos, "\n  "))
	}
	if len(excecoesMortas) > 0 {
		sort.Strings(excecoesMortas)
		t.Errorf("%d exceção(ões) em semSegredo que não correspondem a campo nenhum.\n"+
			"Remova, senão a lista vira depósito:\n  %s",
			len(excecoesMortas), strings.Join(excecoesMortas, "\n  "))
	}
}
