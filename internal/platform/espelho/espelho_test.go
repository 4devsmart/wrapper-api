package espelho

import (
	"fmt"
	"strings"
	"testing"
)

// O espelho é peça de carga: os quatro documentos dependem dele para descobrir
// campo que morre na tradução. Um bug aqui não quebra nada, só para de pegar
// coisas, em silêncio, que é o pior modo de falhar para um teste.

type modelo struct {
	Sai       string  `json:"sai"`
	Esquecido string  `json:"esquecido"`
	Numero    int     `json:"numero"`
	Valor     float64 `json:"valor"`
	Lista     []item  `json:"lista"`
	Grupo     *grupo  `json:"grupo,omitempty"`
	Escolha   escolha `json:"escolha"`
}

type item struct {
	Chave string `json:"chave"`
}

type grupo struct {
	A string `json:"a"`
	B string `json:"b"`
}

type escolha struct {
	Um    *item `json:"um,omitempty"`
	Outro *item `json:"outro,omitempty"`
}

type raiz struct {
	Raiz modelo `json:"raiz"`
}

// gerar escreve tudo menos o campo "esquecido" e a subárvore "grupo".
func gerar(a any) string {
	m := a.(*raiz).Raiz
	var sb strings.Builder
	fmt.Fprintf(&sb, "sai=%s\n", m.Sai)
	fmt.Fprintf(&sb, "numero=%d\n", m.Numero)
	fmt.Fprintf(&sb, "valor=%s\n", strings.Replace(fmt.Sprintf("%.2f", m.Valor), ".", ",", 1))
	for _, i := range m.Lista {
		fmt.Fprintf(&sb, "chave=%s\n", i.Chave)
	}
	for _, e := range []*item{m.Escolha.Um, m.Escolha.Outro} {
		if e != nil {
			fmt.Fprintf(&sb, "escolha=%s\n", e.Chave)
		}
	}
	return sb.String()
}

// espia captura o que Conferir reportaria.
type espia struct{ erros []string }

func (e *espia) Helper()                     {}
func (e *espia) Errorf(f string, a ...any)   { e.erros = append(e.erros, fmt.Sprintf(f, a...)) }
func (e *espia) Fatalf(f string, a ...any)   { e.Errorf(f, a...) }
func (e *espia) texto() string               { return strings.Join(e.erros, "\n") }
func (e *espia) reclamou(trecho string) bool { return strings.Contains(e.texto(), trecho) }
func caso(tsv string, g Grupos) Caso {
	return Caso{
		Nome:       "fake",
		Novo:       func() any { return &raiz{} },
		Gerar:      gerar,
		Grupos:     g,
		Permitidas: "testdata/" + tsv,
	}
}

// O que o teste existe para fazer: acusar o campo que não chega à saída.
func TestConferir_AcusaCampoQueNaoChega(t *testing.T) {
	e := &espia{}
	Conferir(e, caso("vazia.tsv", nil))

	if !e.reclamou("raiz.esquecido") {
		t.Errorf("o campo que o construtor ignora não foi acusado:\n%s", e.texto())
	}
	// E não pode acusar quem chega: falso positivo faz a lista de exceções
	// crescer com linhas erradas, e aí o teste deixa de significar algo.
	for _, chega := range []string{"raiz.sai", "raiz.numero", "raiz.valor", "raiz.lista[].chave"} {
		if strings.Contains(e.texto(), chega+"\n") {
			t.Errorf("%s chega à saída e foi acusado:\n%s", chega, e.texto())
		}
	}
}

func TestConferir_ExcecaoDesculpaOCampo(t *testing.T) {
	e := &espia{}
	Conferir(e, caso("com_excecao.tsv", Grupos{"grupo": {{"A", "B"}}}))
	if e.reclamou("raiz.esquecido") {
		t.Errorf("a exceção registrada não desculpou o campo:\n%s", e.texto())
	}
}

// Lista que acumula linha morta para de significar alguma coisa.
func TestConferir_AcusaExcecaoQueNaoValeMais(t *testing.T) {
	e := &espia{}
	Conferir(e, caso("obsoleta.tsv", nil))
	if !e.reclamou("não valem mais") || !e.reclamou("raiz.sai") {
		t.Errorf("a linha obsoleta não foi acusada:\n%s", e.texto())
	}
}

// Subárvore inteira numa linha só, para o caso em que um grupo não tem lugar no
// destino. Repetir o mesmo motivo dezenas de vezes esconderia a decisão.
func TestConferir_SubarvoreCobreOGrupoInteiro(t *testing.T) {
	e := &espia{}
	Conferir(e, caso("subarvore.tsv", nil))
	for _, campo := range []string{"raiz.grupo.a", "raiz.grupo.b"} {
		if e.reclamou(campo) {
			t.Errorf("a subárvore não cobriu %s:\n%s", campo, e.texto())
		}
	}
}

// --- preenchimento ----------------------------------------------------------

// As sentinelas não podem ser pedaço umas das outras: se fossem, um campo
// ausente pareceria presente por casar dentro do valor de outro.
func TestPreencher_SentinelasNaoSeConfundem(t *testing.T) {
	campos := Preencher(&raiz{}, -1, nil)
	if len(campos) < 8 {
		t.Fatalf("só %d folhas: a varredura não desceu", len(campos))
	}
	for _, a := range campos {
		for _, b := range campos {
			if a.Caminho == b.Caminho {
				continue
			}
			for _, fa := range a.Esperado {
				for _, fb := range b.Esperado {
					if strings.Contains(fa, fb) {
						t.Errorf("a sentinela de %s (%q) contém a de %s (%q)", a.Caminho, fa, b.Caminho, fb)
					}
				}
			}
		}
	}
}

// O caminho publicado usa o nome do CONTRATO: quem lê a lista de exceções está
// olhando para o payload que o cliente monta, não para o struct Go.
func TestPreencher_CaminhoUsaONomeDoContrato(t *testing.T) {
	visto := map[string]bool{}
	for _, c := range Preencher(&raiz{}, -1, nil) {
		visto[c.Caminho] = true
	}
	for _, quero := range []string{"raiz.sai", "raiz.lista[].chave", "raiz.grupo.a", "raiz.escolha.um.chave"} {
		if !visto[quero] {
			t.Errorf("caminho %q não foi produzido; produzidos: %v", quero, visto)
		}
	}
	if visto["Raiz.Sai"] {
		t.Error("o caminho saiu com o nome do campo Go")
	}
}

// Alternativas mutuamente exclusivas: cada passada exercita uma, e a união das
// passadas tem de cobrir todas. É o que evita que a alternativa não escolhida
// pareça esquecida.
func TestGrupos_CadaPassadaEscolheUmaAlternativa(t *testing.T) {
	g := Grupos{"escolha": {{"Um", "Outro"}}}
	if n := Passos(g); n != 2 {
		t.Fatalf("Passos = %d, quero 2", n)
	}
	uniao := map[string]bool{}
	for passo := range Passos(g) {
		var r raiz
		campos := Preencher(&r, passo, g)
		um, outro := r.Raiz.Escolha.Um != nil, r.Raiz.Escolha.Outro != nil
		if um == outro {
			t.Errorf("passo %d preencheu as duas alternativas (ou nenhuma)", passo)
		}
		for _, c := range campos {
			uniao[c.Caminho] = true
		}
	}
	for _, quero := range []string{"raiz.escolha.um.chave", "raiz.escolha.outro.chave"} {
		if !uniao[quero] {
			t.Errorf("nenhuma passada cobriu %s", quero)
		}
	}
}

// Grupo declarado com campo que não existe é engano de quem escreveu o teste, e
// engano silencioso: os campos citados simplesmente não seriam conferidos.
func TestGrupos_CampoInexistenteEhErroAlto(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("grupo citando campo inexistente passou em silêncio")
		}
	}()
	Preencher(&raiz{}, 0, Grupos{"escolha": {{"Um", "NaoExiste"}}})
}

// Exceção sem motivo é bug tolerado com outro nome.
func TestCarregarTSV_ExigeMotivo(t *testing.T) {
	e := &espia{}
	carregarTSV(e, "testdata/sem_motivo.tsv")
	if !e.reclamou("sem motivo") {
		t.Errorf("linha sem motivo passou:\n%s", e.texto())
	}
}
