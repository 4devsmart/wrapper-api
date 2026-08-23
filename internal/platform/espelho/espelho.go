// Package espelho fecha a direção que faltava no lockstep dos documentos.
//
// O lockstep existente compara o que a biblioteca fiscal LÊ com o que os nossos
// builders ESCREVEM, e é uma comparação entre biblioteca e builder. Nenhuma das
// duas pontas é o contrato público. Um campo pode estar no modelo Go, ser
// aceito pelo decodificador, sair publicado no OpenAPI e não ser escrito por
// linha nenhuma: as duas comparações passam, porque o campo simplesmente não
// aparece em nenhum dos dois conjuntos. Foi o que aconteceu com o grupo de
// seguro do multimodal, aceito e descartado desde sempre.
//
// Aqui a comparação é modelo contra saída: preenche o modelo inteiro com
// valores sentinela e exige que cada um apareça no INI gerado. O que não
// aparecer é um campo que o cliente pode mandar e que morre na tradução.
//
// Falhar aqui não é "conserte o código", é "decida": passar a escrever, ou
// registrar no TSV com o motivo.
package espelho

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Campo é uma folha do modelo e as formas em que o valor dela pode aparecer na
// saída. Mais de uma porque o mesmo número sai com vírgula decimal no INI
// fiscal e com ponto em outros lugares, e a pergunta aqui é "o dado chegou",
// não "o dado foi formatado assim".
type Campo struct {
	Caminho  string
	Esperado []string
}

// Grupos declara alternativas mutuamente exclusivas, por nome de tipo. O
// builder escolhe UMA (o grupo de ICMS, o tipo de agendamento de entrega), e
// preencher todas de uma vez faria as não escolhidas parecerem esquecidas.
// Cada passada escolhe uma alternativa diferente, então todas acabam cobertas.
type Grupos map[string][][]string

// Passos é quantas passadas cobrem todas as alternativas declaradas.
func Passos(g Grupos) int {
	n := 1
	for _, grupos := range g {
		for _, alt := range grupos {
			if len(alt) > n {
				n = len(alt)
			}
		}
	}
	return n
}

// Preencher enche alvo (ponteiro para struct) com sentinelas e devolve as
// folhas preenchidas. O passo escolhe a alternativa de cada grupo; passo
// negativo ignora os grupos e preenche tudo, que é como se descobre o conjunto
// completo de folhas.
func Preencher(alvo any, passo int, grupos Grupos) []Campo {
	v := reflect.ValueOf(alvo)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		panic("espelho: alvo precisa ser ponteiro não nulo para struct")
	}
	e := &estado{passo: passo, grupos: grupos}
	e.andar(v.Elem(), "", 0)
	return e.campos
}

type estado struct {
	passo  int
	grupos Grupos
	n      int
	campos []Campo
}

// profundidadeMax é rede de segurança: os modelos fiscais são árvores, mas um
// tipo recursivo introduzido sem querer viraria pilha estourada em vez de erro.
const profundidadeMax = 40

func (e *estado) andar(v reflect.Value, caminho string, prof int) {
	if prof > profundidadeMax {
		panic("espelho: modelo fundo demais em " + caminho + " (tipo recursivo?)")
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		e.andar(v.Elem(), caminho, prof+1)

	case reflect.Struct:
		tp := v.Type()
		pular := e.pulados(tp)
		for i := range tp.NumField() {
			c := tp.Field(i)
			if !c.IsExported() || pular[c.Name] {
				continue
			}
			e.andar(v.Field(i), juntar(caminho, nomeJSON(c)), prof+1)
		}

	case reflect.Slice:
		// Um elemento basta: a pergunta é se o builder percorre a lista, não
		// quantos itens ele aguenta.
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		e.andar(v.Index(0), caminho+"[]", prof+1)

	case reflect.String:
		e.n++
		// Dez caracteres porque o builder de data corta em dez: mais curto que
		// isso vira string vazia e o campo pareceria esquecido sem ser. E só
		// dígitos porque há campo que o builder filtra para dígitos (o código
		// NBS da NFS-e): com letra, a sentinela sobreviveria mutilada e o campo
		// pareceria descartado quando na verdade chegou inteiro. O 9 inicial
		// mantém as sentinelas de texto fora da faixa das numéricas.
		s := fmt.Sprintf("9%09d", e.n)
		v.SetString(s)
		e.anotar(caminho, s)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.n++
		// Largura fixa para que uma sentinela nunca seja pedaço de outra.
		n := int64(8100000 + e.n)
		v.SetInt(n)
		e.anotar(caminho, strconv.FormatInt(n, 10))

	case reflect.Float32, reflect.Float64:
		e.n++
		f := float64(8200000+e.n) + 0.07
		v.SetFloat(f)
		comPonto := strconv.FormatFloat(f, 'f', 2, 64)
		e.anotar(caminho, strings.Replace(comPonto, ".", ",", 1), comPonto)

	default:
		panic("espelho: tipo não previsto em " + caminho + ": " + v.Kind().String())
	}
}

func (e *estado) anotar(caminho string, formas ...string) {
	e.campos = append(e.campos, Campo{Caminho: caminho, Esperado: formas})
}

// pulados devolve os campos que esta passada NÃO preenche, por serem
// alternativas não escolhidas.
func (e *estado) pulados(tp reflect.Type) map[string]bool {
	grupos := e.grupos[tp.Name()]
	if len(grupos) == 0 || e.passo < 0 {
		return nil
	}
	out := map[string]bool{}
	for _, alt := range grupos {
		for _, nome := range alt {
			if _, ok := tp.FieldByName(nome); !ok {
				panic("espelho: grupo de " + tp.Name() + " cita campo inexistente: " + nome)
			}
		}
		escolhido := alt[e.passo%len(alt)]
		for _, nome := range alt {
			if nome != escolhido {
				out[nome] = true
			}
		}
	}
	return out
}

// nomeJSON usa o nome do CONTRATO, não o do Go: quem lê o TSV de exceções está
// olhando para o payload que o cliente monta.
func nomeJSON(c reflect.StructField) string {
	tag := c.Tag.Get("json")
	if tag == "" || tag == "-" {
		return c.Name
	}
	if nome, _, _ := strings.Cut(tag, ","); nome != "" {
		return nome
	}
	return c.Name
}

func juntar(caminho, nome string) string {
	if caminho == "" {
		return nome
	}
	return caminho + "." + nome
}

// --- conferência ------------------------------------------------------------

// Reportador é o pedaço de *testing.T que interessa. Declarado aqui para o
// pacote não importar testing: ele é apoio de teste, não um teste.
type Reportador interface {
	Helper()
	Errorf(formato string, args ...any)
	Fatalf(formato string, args ...any)
}

// Caso descreve um tradutor a conferir.
type Caso struct {
	// Nome aparece na falha (ex.: "cte.ToINI").
	Nome string
	// Novo devolve um ponteiro para o modelo zerado.
	Novo func() any
	// Gerar recebe o ponteiro preenchido e devolve a saída a inspecionar.
	Gerar func(alvo any) string
	// Grupos são as alternativas mutuamente exclusivas do modelo.
	Grupos Grupos
	// Permitidas é o TSV com os campos que não saem, e o porquê de cada um.
	Permitidas string
}

// Conferir roda todas as passadas e cobra cada folha do modelo.
func Conferir(t Reportador, c Caso) {
	t.Helper()
	excecoes := carregarTSV(t, c.Permitidas)

	// Passo negativo: o conjunto COMPLETO de folhas, sem escolha de alternativa.
	// Serve para provar que as passadas cobriram tudo, e é o que pega um grupo
	// declarado com campo a mais ou a menos.
	todas := map[string]bool{}
	for _, campo := range Preencher(c.Novo(), -1, c.Grupos) {
		todas[campo.Caminho] = true
	}

	visitadas := map[string]bool{}
	faltando := map[string]bool{}
	desculpou := map[string]int{}
	for passo := range Passos(c.Grupos) {
		alvo := c.Novo()
		campos := Preencher(alvo, passo, c.Grupos)
		saida := c.Gerar(alvo)
		if strings.TrimSpace(saida) == "" {
			t.Fatalf("%s: passo %d não gerou saída nenhuma", c.Nome, passo)
		}
		for _, campo := range campos {
			visitadas[campo.Caminho] = true
			if apareceu(saida, campo) {
				continue
			}
			// A ausência é real. A lista só decide se ela é aceitável, e a
			// exceção que a desculpou fica marcada como ainda necessária.
			if e := acha(excecoes, campo.Caminho); e != nil {
				desculpou[e.padrao]++
				continue
			}
			faltando[campo.Caminho] = true
		}
	}

	if naoVisitadas := diferenca(todas, visitadas); len(naoVisitadas) > 0 {
		t.Errorf("%s: %d campo(s) que nenhuma passada preencheu. Um grupo de\n"+
			"alternativas está declarado errado, e estes campos nunca foram conferidos:\n  %s",
			c.Nome, len(naoVisitadas), strings.Join(naoVisitadas, "\n  "))
	}

	if len(faltando) > 0 {
		lista := chaves(faltando)
		t.Errorf("%s: %d campo(s) do contrato que NÃO chegam à saída.\n\n"+
			"O cliente pode mandar cada um deles, a requisição é aceita, e o dado morre\n"+
			"na tradução sem aviso. Decida um a um: passar a escrever no builder, ou\n"+
			"registrar em %s com o motivo.\n\n  %s",
			c.Nome, len(lista), c.Permitidas, strings.Join(lista, "\n  "))
	}

	// Exceção que não desculpou nada nesta rodada deixou de ser necessária: o
	// campo passou a sair, ou sumiu do modelo. Sem esta checagem a lista vira
	// depósito e o teste para de cobrar sem ninguém perceber.
	var obsoletas []string
	for _, e := range excecoes {
		if desculpou[e.padrao] > 0 {
			continue
		}
		motivo := "o campo passou a sair"
		if !alcanca(todas, e) {
			motivo = "não existe campo nenhum com este caminho"
		}
		obsoletas = append(obsoletas, e.padrao+"\t("+motivo+")")
	}
	if len(obsoletas) > 0 {
		sort.Strings(obsoletas)
		t.Errorf("%s: %d linha(s) de %s que não valem mais.\n"+
			"Remova para travar o ganho:\n  %s",
			c.Nome, len(obsoletas), c.Permitidas, strings.Join(obsoletas, "\n  "))
	}
}

// excecao é uma linha da lista: um caminho exato, ou uma subárvore inteira
// quando o caminho termina em ".*". A subárvore existe para o caso em que um
// grupo inteiro não tem lugar no layout de destino, onde repetir o mesmo motivo
// dezenas de vezes esconderia a decisão em vez de registrá-la.
type excecao struct {
	padrao  string
	motivo  string
	prefixo string // não vazio quando a linha cobre uma subárvore
}

func acha(excecoes []excecao, caminho string) *excecao {
	for i := range excecoes {
		e := &excecoes[i]
		if e.prefixo != "" {
			if strings.HasPrefix(caminho, e.prefixo) {
				return e
			}
			continue
		}
		if e.padrao == caminho {
			return e
		}
	}
	return nil
}

func alcanca(todas map[string]bool, e excecao) bool {
	if e.prefixo == "" {
		return todas[e.padrao]
	}
	for caminho := range todas {
		if strings.HasPrefix(caminho, e.prefixo) {
			return true
		}
	}
	return false
}

func apareceu(saida string, campo Campo) bool {
	for _, forma := range campo.Esperado {
		if strings.Contains(saida, forma) {
			return true
		}
	}
	return false
}

func carregarTSV(t Reportador, arquivo string) []excecao {
	t.Helper()
	b, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatalf("lista de exceções ausente: %v", err)
		return nil
	}
	var out []excecao
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		padrao, motivo, _ := strings.Cut(l, "\t")
		padrao, motivo = strings.TrimSpace(padrao), strings.TrimSpace(motivo)
		if motivo == "" {
			t.Errorf("%s: %q sem motivo. Exceção sem porquê é bug tolerado com outro nome.", arquivo, padrao)
		}
		e := excecao{padrao: padrao, motivo: motivo}
		if raiz, achou := strings.CutSuffix(padrao, ".*"); achou {
			e.prefixo = raiz + "."
		}
		out = append(out, e)
	}
	return out
}

func diferenca(todas, visitadas map[string]bool) []string {
	var out []string
	for k := range todas {
		if !visitadas[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func chaves(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
