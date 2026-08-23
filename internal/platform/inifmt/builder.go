package inifmt

import (
	"strconv"
	"strings"
	"time"
)

// Builder acumula o arquivo intermediário que a biblioteca fiscal consome.
//
// Os quatro documentos escreviam o MESMO construtor, copiado: seção, par
// chave=valor, variantes opcionais, e as datas no fuso do emitente. Copiado
// quatro vezes, um ajuste na sanitização ou no formato de data precisava ser
// feito quatro vezes, e a chance de esquecer uma é a chance de um documento
// sair diferente dos outros sem ninguém notar. As diferenças de DOMÍNIO
// continuam em cada pacote, que é onde elas significam alguma coisa.
type Builder struct {
	sb    strings.Builder
	local *time.Location
}

// EmUF fixa o fuso do documento a partir do código IBGE da UF (o campo cUF).
// Sem isso as datas caem em Brasília, que é o default da biblioteca.
func (b *Builder) EmUF(cuf int) { b.local = LocalDoCodigoUF(cuf) }

// Local devolve o fuso do documento; Brasília quando não definido.
func (b *Builder) Local() *time.Location {
	if b.local == nil {
		b.local = LocalDaUF("")
	}
	return b.local
}

// Secao abre uma seção. A linha em branco antes é só legibilidade para quem
// depura; a biblioteca não se importa.
func (b *Builder) Secao(nome string) {
	if b.sb.Len() > 0 {
		b.sb.WriteByte('\n')
	}
	b.sb.WriteString("[" + nome + "]\n")
}

// KV escreve o par, SEMPRE, inclusive com valor vazio: há campo que a
// biblioteca espera encontrar mesmo em branco.
//
// O valor passa por Sanitize porque cada par é uma linha: um texto livre do
// cliente com quebra de linha forjaria seção, e uma razão social com
// "\n[emit]" reescreveria o documento.
func (b *Builder) KV(chave, valor string) {
	b.sb.WriteString(chave + "=" + Sanitize(valor) + "\n")
}

// KVOpt omite a chave quando o valor é vazio. Para campo opcional em que
// "ausente" e "em branco" não significam a mesma coisa.
func (b *Builder) KVOpt(chave, valor string) {
	if valor != "" {
		b.KV(chave, valor)
	}
}

// KVInt escreve sempre, inclusive zero. Para campo obrigatório cujo zero é
// valor válido.
func (b *Builder) KVInt(chave string, v int) { b.KV(chave, strconv.Itoa(v)) }

// KVIntOpt omite o zero, que no contrato é a ausência do campo.
func (b *Builder) KVIntOpt(chave string, v int) {
	if v != 0 {
		b.KV(chave, strconv.Itoa(v))
	}
}

// DataHora formata data-hora no fuso do documento. Vazio vira o instante atual,
// porque o campo é obrigatório e a biblioteca recusaria em branco.
func (b *Builder) DataHora(s string) string {
	if s == "" {
		return time.Now().In(b.Local()).Format("02/01/2006 15:04:05")
	}
	return DataHoraNoFuso(s, b.Local())
}

// DataHoraOpt é como DataHora, e vazio continua vazio: a chave some.
func (b *Builder) DataHoraOpt(s string) string {
	if s == "" {
		return ""
	}
	return DataHoraNoFuso(s, b.Local())
}

// Data é a data sem hora, no fuso do documento; vazio continua vazio.
func (b *Builder) Data(s string) string {
	if dt := b.DataHoraOpt(s); len(dt) >= 10 {
		return dt[:10]
	}
	return ""
}

func (b *Builder) String() string { return b.sb.String() }
