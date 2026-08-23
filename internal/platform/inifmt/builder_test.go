package inifmt

import (
	"strings"
	"testing"
	"time"
)

// Este construtor é usado pelos quatro documentos. Antes cada um tinha o seu,
// idêntico, e um ajuste aqui precisava ser feito quatro vezes. Em troca da
// centralização, um bug aqui sai nos quatro ao mesmo tempo: por isso o teste.

func TestBuilder_SecaoEPares(t *testing.T) {
	var b Builder
	b.Secao("ide")
	b.KV("nCT", "100")
	b.KVOpt("xJust", "")
	b.KVOpt("natOp", "Transporte")
	b.KVInt("tpNav", 0)
	b.KVIntOpt("cUF", 0)
	b.KVIntOpt("serie", 1)

	quero := "[ide]\nnCT=100\nnatOp=Transporte\ntpNav=0\nserie=1\n"
	if got := b.String(); got != quero {
		t.Errorf("got:\n%s\nquero:\n%s", got, quero)
	}
}

// KVInt existe separado de KVIntOpt porque há campo obrigatório cujo zero é
// valor válido: omiti-lo faria a biblioteca recusar o documento.
func TestBuilder_ZeroSaiComKVIntESomeComKVIntOpt(t *testing.T) {
	var b Builder
	b.Secao("x")
	b.KVInt("obrigatorio", 0)
	b.KVIntOpt("opcional", 0)
	if !strings.Contains(b.String(), "obrigatorio=0") {
		t.Error("KVInt omitiu o zero")
	}
	if strings.Contains(b.String(), "opcional") {
		t.Error("KVIntOpt escreveu o zero")
	}
}

// Cada par é uma linha: um valor com quebra de linha forjaria seção, e uma
// razão social com "\n[emit]" reescreveria o documento.
func TestBuilder_ValorNaoForjaSecao(t *testing.T) {
	var b Builder
	b.Secao("emit")
	b.KV("xNome", "Empresa\n[emit]\nCNPJ=00000000000191\rmais")

	linhas := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(linhas) != 2 {
		t.Fatalf("o valor virou %d linhas:\n%s", len(linhas), b.String())
	}
	if strings.HasPrefix(linhas[1], "[") {
		t.Errorf("o valor virou cabeçalho de seção: %q", linhas[1])
	}
}

// A linha em branco entre seções é para quem depura; não pode aparecer ANTES da
// primeira, senão o arquivo começa com linha vazia.
func TestBuilder_NaoComecaComLinhaEmBranco(t *testing.T) {
	var b Builder
	b.Secao("primeira")
	if strings.HasPrefix(b.String(), "\n") {
		t.Errorf("começou com linha em branco: %q", b.String())
	}
}

// --- fuso do documento ------------------------------------------------------

// O fuso vem do cUF do documento porque é o que a biblioteca carimba. Derivar
// de outro campo daria um instante diferente do que o próprio XML declara.
func TestBuilder_DataHoraNoFusoDoEmitente(t *testing.T) {
	var b Builder
	b.EmUF(12) // AC, -05
	if got := b.DataHora("2026-05-01T11:00:00Z"); got != "01/05/2026 06:00:00" {
		t.Errorf("DataHora = %q, quero 01/05/2026 06:00:00", got)
	}

	var sp Builder
	sp.EmUF(35) // SP, -03
	if got := sp.DataHora("2026-05-01T11:00:00Z"); got != "01/05/2026 08:00:00" {
		t.Errorf("DataHora = %q, quero 01/05/2026 08:00:00", got)
	}
}

// Sem UF o default é Brasília, igual ao da biblioteca. NUNCA o fuso do
// servidor: o container roda em UTC.
func TestBuilder_SemUFCaiEmBrasilia(t *testing.T) {
	var b Builder
	if _, off := time.Now().In(b.Local()).Zone(); off != -3*3600 {
		t.Errorf("offset %d, quero -3", off/3600)
	}
}

func TestBuilder_DataEDataHoraOpt(t *testing.T) {
	var b Builder
	b.EmUF(35)
	if got := b.Data("2026-05-01T11:00:00Z"); got != "01/05/2026" {
		t.Errorf("Data = %q", got)
	}
	if got := b.DataHoraOpt(""); got != "" {
		t.Errorf("DataHoraOpt(\"\") = %q, quero vazio: a chave some", got)
	}
	// DataHora com entrada vazia é o instante atual: o campo é obrigatório e a
	// biblioteca recusaria em branco.
	if got := b.DataHora(""); len(got) != len("02/01/2006 15:04:05") {
		t.Errorf("DataHora(\"\") = %q, esperava o instante atual formatado", got)
	}
}

// --- data sem hora ----------------------------------------------------------

// O fuso é parâmetro, e não o do servidor: o container roda em UTC, e uma nota
// emitida às 22h no horário de Brasília ganharia a data do dia SEGUINTE.
func TestDataBR_UsaOFusoInformado(t *testing.T) {
	sp := LocalDaUF("SP")
	casos := map[string]string{
		"2026-05-01":           "01/05/2026",
		"2026-05-01T11:00:00Z": "01/05/2026",
		"2026-05-01T00:00:00Z": "30/04/2026", // 21:00 do dia anterior em SP
		"01/05/2026":           "01/05/2026",
	}
	for entrada, quero := range casos {
		if got := DataBR(entrada, sp); got != quero {
			t.Errorf("DataBR(%q) = %q, quero %q", entrada, got, quero)
		}
	}
	// Entrada ilegível passa adiante: a biblioteca reclama com mensagem própria,
	// o que diz mais ao cliente do que uma data trocada em silêncio.
	if got := DataBR("ontem", sp); got != "ontem" {
		t.Errorf("entrada ilegível = %q", got)
	}
}

func TestDataBROpt_VazioContinuaVazio(t *testing.T) {
	if got := DataBROpt("", LocalDaUF("SP")); got != "" {
		t.Errorf("DataBROpt(\"\") = %q", got)
	}
	if got := DataBR("", LocalDaUF("SP")); got == "" {
		t.Error("DataBR(\"\") deveria virar hoje, não vazio")
	}
}

// --- largura dos índices ----------------------------------------------------

// A largura errada faz a biblioteca não encontrar a seção, em silêncio: foi
// assim que [autXML001] passou a ser ignorado onde ela lê [autXML01].
func TestSeq3(t *testing.T) {
	for n, quero := range map[int]string{0: "000", 1: "001", 42: "042", 999: "999", 1000: "1000"} {
		if got := Seq3(n); got != quero {
			t.Errorf("Seq3(%d) = %q, quero %q", n, got, quero)
		}
	}
}
