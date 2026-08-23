package inifmt

import (
	"strings"
	"testing"
	"time"
)

// Este pacote existe para que a formatação dos INIs seja decidida UMA vez. Os
// quatro módulos passam por aqui, então um bug daqui sai em CT-e, MDF-e, NFS-e e
// boleto ao mesmo tempo. Antes os testes viviam nos módulos, exercitando estas
// funções através de aliases de uma linha: cobertura aparente, sem garantia.

// --- injeção no INI ---------------------------------------------------------

// Cada par chave=valor é uma linha. Um valor com quebra de linha forja seção,
// então uma razão social com "\n[Emitente]" reescreveria o documento.
func TestSanitize_QuebraDeLinhaNaoForjaSecao(t *testing.T) {
	casos := map[string]string{
		"Empresa\nLTDA":                    "Empresa LTDA",
		"Empresa\r\nLTDA":                  "Empresa  LTDA",
		"x\n[Emitente]\nCNPJ=00000000000":  "x [Emitente] CNPJ=00000000000",
		"sem quebra":                       "sem quebra",
		"":                                 "",
		"tab\tpermanece":                   "tab\tpermanece",
		"Rua das Flores, 100 - Sala\r\n42": "Rua das Flores, 100 - Sala  42",
	}
	for entrada, quero := range casos {
		if got := Sanitize(entrada); got != quero {
			t.Errorf("Sanitize(%q) = %q, quero %q", entrada, got, quero)
		}
	}
	for _, s := range []string{Sanitize("a\nb"), Sanitize("a\rb")} {
		if strings.ContainsAny(s, "\r\n") {
			t.Errorf("sobrou quebra de linha em %q", s)
		}
	}
}

// --- moeda ------------------------------------------------------------------

// O ACBr lê decimal com vírgula: "1234.50" seria lido como zero, e o documento
// sairia com o valor errado sem nenhum erro.
func TestMoney_VirgulaDecimalEDuasCasas(t *testing.T) {
	casos := map[float64]string{
		1500.5:   "1500,50",
		0:        "0,00",
		1234.567: "1234,57",
		-42.1:    "-42,10",
		1e6:      "1000000,00",
	}
	for v, quero := range casos {
		if got := Money(v); got != quero {
			t.Errorf("Money(%v) = %q, quero %q", v, got, quero)
		}
		if strings.Contains(Money(v), ".") {
			t.Errorf("Money(%v) deixou ponto decimal: %q", v, Money(v))
		}
	}
}

// MoneyOpt é para campo opcional: zero some do INI em vez de virar "0,00", que
// para alguns grupos é diferente de não informar.
func TestMoneyOpt_ZeroSome(t *testing.T) {
	if got := MoneyOpt(0); got != "" {
		t.Errorf("MoneyOpt(0) = %q, quero vazio", got)
	}
	if got := MoneyOpt(0.01); got != "0,01" {
		t.Errorf("MoneyOpt(0.01) = %q", got)
	}
}

// --- índices de seção -------------------------------------------------------

// As seções repetidas do INI são indexadas com zero à esquerda ([det001],
// [Comp01]). Errar a largura faz a lib não encontrar a seção.
func TestSeq(t *testing.T) {
	for n, quero := range map[int]string{0: "00", 1: "01", 9: "09", 10: "10", 99: "99", 100: "100"} {
		if got := Seq2(n); got != quero {
			t.Errorf("Seq2(%d) = %q, quero %q", n, got, quero)
		}
	}
	for n, quero := range map[int]string{0: "0000", 1: "0001", 42: "0042", 1234: "1234", 12345: "12345"} {
		if got := Seq4(n); got != quero {
			t.Errorf("Seq4(%d) = %q, quero %q", n, got, quero)
		}
	}
}

// --- fuso do emitente -------------------------------------------------------

func TestLocalDoCodigoUF(t *testing.T) {
	casos := map[int]int{
		35: -3, // SP
		33: -3, // RJ
		12: -5, // AC
		13: -4, // AM
		51: -4, // MT
		50: -4, // MS
		0:  -3, // ausente: Brasília, igual ao ACBr
		99: -3, // desconhecido
	}
	for cuf, horas := range casos {
		_, off := time.Now().In(LocalDoCodigoUF(cuf)).Zone()
		if off != horas*3600 {
			t.Errorf("cUF %d: offset %d, quero %d", cuf, off/3600, horas)
		}
	}
}

// A lib carimba o offset com GetUTC(cUF). Derivar o fuso de outro campo daria um
// instante diferente do que o próprio XML declara.
func TestLocalDaUF_DefaultEBrasilia(t *testing.T) {
	for _, uf := range []string{"", "  ", "XX", "sp", "SP"} {
		_, off := time.Now().In(LocalDaUF(uf)).Zone()
		quero := -3 * 3600
		if off != quero {
			t.Errorf("UF %q: offset %d, quero %d", uf, off/3600, quero/3600)
		}
	}
	if _, off := time.Now().In(LocalDaUF("ac")).Zone(); off != -5*3600 {
		t.Errorf("a UF precisa ser reconhecida em minúscula: offset %d", off/3600)
	}
}

// --- data e hora ------------------------------------------------------------

// Com offset a entrada é um INSTANTE: precisa ser convertida. Sem converter, um
// pedido em UTC ("11:00Z") viraria "11:00" no fuso do estado, três horas
// adiantado, e sem erro nenhum.
func TestDataHoraNoFuso_ComOffsetConverteOInstante(t *testing.T) {
	sp := LocalDaUF("SP") // -03
	casos := map[string]string{
		"2026-05-01T11:00:00Z":      "01/05/2026 08:00:00",
		"2026-05-01T11:00:00-03:00": "01/05/2026 11:00:00",
		"2026-05-01T00:00:00Z":      "30/04/2026 21:00:00", // vira o dia anterior
		"2026-05-01T11:00:00+02:00": "01/05/2026 06:00:00",
	}
	for entrada, quero := range casos {
		if got := DataHoraNoFuso(entrada, sp); got != quero {
			t.Errorf("DataHoraNoFuso(%q) = %q, quero %q", entrada, got, quero)
		}
	}

	// O mesmo instante em UF de fuso diferente dá relógio diferente: é o ponto.
	if got := DataHoraNoFuso("2026-05-01T11:00:00Z", LocalDaUF("AC")); got != "01/05/2026 06:00:00" {
		t.Errorf("no Acre (-05) deveria dar 06:00:00, veio %q", got)
	}
}

// Sem offset o cliente escreveu o relógio de parede do emitente: nada a
// converter, só a reformatar.
func TestDataHoraNoFuso_SemOffsetEhRelogioDeParede(t *testing.T) {
	sp := LocalDaUF("SP")
	casos := map[string]string{
		"2026-05-01T14:30:00": "01/05/2026 14:30:00",
		"2026-05-01 14:30:00": "01/05/2026 14:30:00",
		"01/05/2026 14:30:00": "01/05/2026 14:30:00",
		"2026-05-01":          "01/05/2026 00:00:00",
	}
	for entrada, quero := range casos {
		if got := DataHoraNoFuso(entrada, sp); got != quero {
			t.Errorf("DataHoraNoFuso(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

func TestDataHoraNoFuso_VazioEIlegivel(t *testing.T) {
	sp := LocalDaUF("SP")
	if got := DataHoraNoFuso("   ", sp); got != "" {
		t.Errorf("vazio deveria continuar vazio (o chamador decide o default), veio %q", got)
	}
	// Formato que não reconhecemos passa adiante em vez de virar zero: a lib
	// reclama com mensagem própria, o que diz mais ao cliente do que uma data
	// silenciosamente trocada.
	if got := DataHoraNoFuso("ontem à tarde", sp); got != "ontem à tarde" {
		t.Errorf("entrada ilegível = %q, deveria passar adiante intacta", got)
	}
}
