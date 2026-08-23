// Package inifmt centraliza a escrita do arquivo intermediário que a biblioteca
// fiscal consome, e a formatação dos valores que vão nele.
//
// Os quatro documentos escreviam o mesmo construtor e os mesmos ajudantes,
// copiados: sanitização, moeda com vírgula decimal, largura dos índices de
// seção, datas no fuso do emitente. Um ajuste precisava ser feito quatro vezes,
// e a cópia esquecida fazia um documento sair diferente dos outros sem sintoma.
// Já tinha acontecido: a escolha entre CNPJ e CPF descartava um CNPJ em branco
// num lugar e o aceitava em outro.
//
// O que mora aqui é o COMO escrever. O que escrever é de cada documento, e
// continua lá.
package inifmt

import (
	"strconv"
	"strings"
	"time"
)

// iniValReplacer troca CR/LF por espaço: no INI cada par chave=valor é uma linha,
// então um valor com quebra de linha corromperia/forjaria a seção (injeção).
var iniValReplacer = strings.NewReplacer("\r", " ", "\n", " ")

// Sanitize remove CR/LF de um valor de INI (defesa anti-injeção, ponto único).
func Sanitize(s string) string { return iniValReplacer.Replace(s) }

// Money formata um float como decimal com vírgula e 2 casas (1500.5 → "1500,50").
func Money(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 2, 64), ".", ",", 1)
}

// MoneyOpt é como Money, mas devolve "" para zero (campo opcional omitido).
func MoneyOpt(v float64) string {
	if v == 0 {
		return ""
	}
	return Money(v)
}

// Seq2 formata n com zero-padding a 2 dígitos.
func Seq2(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		s = "0" + s
	}
	return s
}

// Seq3 formata n com zero-padding a 3 dígitos. É a largura de quase toda seção
// indexada dos documentos fiscais. Quase: autXML e infCteComp usam DUAS casas,
// e [autXML001] é ignorado em silêncio pela biblioteca. Por isso a largura é
// escolhida na chamada, e não adivinhada aqui.
func Seq3(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

// DataBR é a data (sem hora) no formato do arquivo intermediário, no fuso do
// emitente. Entrada não reconhecida passa adiante: a biblioteca reclama com
// mensagem própria, o que diz mais ao cliente do que uma data trocada em
// silêncio.
//
// O fuso é parâmetro, e não o do servidor, porque o container roda em UTC: uma
// nota emitida às 22h no horário de Brasília ganharia a data do dia SEGUINTE, e
// a diferença só apareceria em quem emite à noite.
func DataBR(s string, loc *time.Location) string {
	if s == "" {
		return time.Now().In(loc).Format("02/01/2006")
	}
	if dt := DataHoraNoFuso(s, loc); len(dt) >= 10 {
		return dt[:10]
	}
	return s
}

// DataBROpt é como DataBR, e vazio continua vazio: a chave some.
func DataBROpt(s string, loc *time.Location) string {
	if s == "" {
		return ""
	}
	return DataBR(s, loc)
}

// Seq4 formata n com zero-padding a 4 dígitos.
func Seq4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// --- fuso horário dos documentos fiscais ------------------------------------

// offsetPorUF é a tabela oficial de fuso por UF usada pelo ACBr (GetUTCUF em
// ACBrUtil.DateTime.pas). O Brasil não observa horário de verão desde 2019, então
// os offsets são fixos; documentos com data anterior a isso não são o caso de uso
// deste serviço.
var offsetPorUF = map[string]int{
	"AC": -5,
	"AM": -4, "RR": -4, "RO": -4, "MT": -4, "MS": -4,
}

// ufPorCodigo mapeia o código IBGE da UF (o campo cUF dos documentos) para a
// sigla. É a mesma correspondência do CodigoUFparaUF do ACBr.
var ufPorCodigo = map[int]string{
	11: "RO", 12: "AC", 13: "AM", 14: "RR", 15: "PA", 16: "AP", 17: "TO",
	21: "MA", 22: "PI", 23: "CE", 24: "RN", 25: "PB", 26: "PE", 27: "AL", 28: "SE", 29: "BA",
	31: "MG", 32: "ES", 33: "RJ", 35: "SP",
	41: "PR", 42: "SC", 43: "RS",
	50: "MS", 51: "MT", 52: "GO", 53: "DF",
}

// LocalDoCodigoUF devolve o fuso a partir do CÓDIGO da UF (campo cUF). É este o
// caminho a usar nos documentos: a lib carimba o offset com GetUTC(cUF), então
// derivar o nosso fuso de qualquer outro campo (UFIni, por exemplo) produziria
// um instante diferente do que o XML declara. Código ausente ou desconhecido cai
// em Brasília, igual ao ACBr.
func LocalDoCodigoUF(cuf int) *time.Location { return LocalDaUF(ufPorCodigo[cuf]) }

// LocalDaUF devolve o fuso do estado (UF de duas letras). Qualquer UF fora das
// exceções, e UF vazia ou desconhecida: cai em Brasília (-03:00), que é o mesmo
// default do ACBr.
//
// Existe para uma razão específica: o cliente informa data-hora em RFC 3339, com
// offset, e o documento fiscal precisa do MESMO INSTANTE expresso no fuso do
// emitente. Sem converter, um pedido em UTC ("11:00Z") viraria "11:00" no fuso do
// estado: três horas adiantado, sem erro nenhum.
func LocalDaUF(uf string) *time.Location {
	h, ok := offsetPorUF[strings.ToUpper(strings.TrimSpace(uf))]
	if !ok {
		h = -3
	}
	return time.FixedZone(strings.ToUpper(uf), h*3600)
}

// DataHoraNoFuso converte s para o fuso informado e devolve no formato do INI
// ("dd/mm/aaaa hh:mm:ss"). Entradas SEM offset são lidas como hora local daquele
// fuso (o cliente escreveu o relógio de parede); entradas COM offset denotam um
// instante e são convertidas. Vazio devolve "" (o chamador decide o default).
func DataHoraNoFuso(s string, loc *time.Location) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	const saida = "02/01/2006 15:04:05"
	// Com offset explícito a entrada é um instante: converte para o fuso do documento.
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.In(loc).Format(saida)
		}
	}
	// Sem offset: relógio de parede, já no fuso do documento.
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "02/01/2006 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.Format(saida)
		}
	}
	return s
}
