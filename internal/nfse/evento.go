package nfse

import (
	"cmp"
	"strings"
	"unicode/utf8"

	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// tpEvento do Pedido de Registro de Evento do Padrão Nacional
// (TtpEventoArrayStrings, ACBrNFSeXConversao.pas:602).
const (
	TpEventoCancelamento = "e101101"
)

// Limites de xMotivo (TSMotivo no tiposSimples_v1.01.xsd). O campo é
// OBRIGATÓRIO no e101101 e tem mínimo de 15 caracteres: um motivo vazio ou
// curto é recusado pelo ADN na validação de schema, com mensagem que não diz
// qual campo faltou. Recusar aqui troca isso por uma frase.
const (
	motivoMin = 15
	motivoMax = 255
)

// codigosCancelamentoPN são os cMotivo aceitos no evento de cancelamento
// (TSCodJustCanc). NÃO são os mesmos da substituição, que usa TSCodJustSubst
// com dois dígitos: confundir os dois manda um código válido no lugar errado.
var codigosCancelamentoPN = map[string]string{
	"1": "erro na emissão",
	"2": "serviço não prestado",
	"9": "outros",
}

// EventoPN é o Pedido de Registro de Evento do Padrão Nacional, no vocabulário
// da seção [Evento] que a lib lê (TInfEvento.LerFromIni,
// ACBrNFSeXWebserviceBase.pas:2249).
//
// O autor do evento (CNPJAutor/CPFAutor) NÃO está aqui: o provedor o tira do
// Emitente.CNPJ da sessão (PadraoNacional.Provider.pas:689).
type EventoPN struct {
	Chave      string // chNFSe: a NFS-e a que o evento se vincula
	TpEvento   string // default: cancelamento
	TpAmb      string // 1 = produção, 2 = homologação
	VerAplic   string // default: a versão deste emissor
	DhEvento   string // default: agora, no fuso do documento
	CodMotivo  string // cMotivo
	Motivo     string // xMotivo
	Substituta string // chSubstituta: só nos eventos de substituição
}

// ToINIEvento monta o INI [Evento] consumido por NFSE_EnviarEvento.
func ToINIEvento(e EventoPN) string {
	var b iniBuilder
	b.Secao("Evento")
	b.KV("tpAmb", e.TpAmb)
	b.KV("verAplic", cmp.Or(e.VerAplic, versao.Emissor()))
	// O fuso fica no default (Brasília) de propósito: o provedor concatena o
	// offset a partir da UF configurada no componente, que este produto não
	// preenche, e ali o default também é Brasília. Escrever a hora de Manaus e
	// receber "-03:00" colado nela seria pior do que os dois errados juntos.
	b.KV("dhEvento", b.DataHora(e.DhEvento))
	b.KV("chNFSe", e.Chave)
	b.KV("tpEvento", cmp.Or(e.TpEvento, TpEventoCancelamento))
	b.KV("cMotivo", e.CodMotivo)
	b.KV("xMotivo", e.Motivo)
	b.KVOpt("chSubstituta", e.Substituta)
	return b.String()
}

// ValidarCancelamentoPN confere o que o schema do evento exige ANTES de
// transmitir, e devolve a frase do erro (vazia se está tudo certo).
func ValidarCancelamentoPN(p CancelamentoPedido) string {
	if _, ok := codigosCancelamentoPN[cmp.Or(p.Codigo, "1")]; !ok {
		return "evento.codigo inválido para o Padrão Nacional: use 1 (erro na emissão), 2 (serviço não prestado) ou 9 (outros)"
	}
	motivo := strings.TrimSpace(p.Motivo)
	if n := utf8.RuneCountInString(motivo); n < motivoMin || n > motivoMax {
		return "evento.motivo é obrigatório no Padrão Nacional e precisa ter de 15 a 255 caracteres"
	}
	return ""
}

// EventoRegistrado é o resultado de um Pedido de Registro de Evento, extraído
// da seção [EnviarEvento] da resposta (TEnviarEventoResposta).
//
// Não há protocolo aqui: a resposta do evento não traz um. O que identifica o
// registro é o XML devolvido, e dele sai a data de processamento.
type EventoRegistrado struct {
	Sucesso  bool
	Situacao string // DescSituacao ("Nota Cancelada")
	XML      string // XmlRetorno: o procEveNFSe devolvido pelo ADN
	Erros    []Mensagem
	Alertas  []Mensagem
}

// ParseEvento interpreta a resposta INI do ACBr (NFSE_EnviarEvento).
func ParseEvento(resp string) EventoRegistrado {
	var e EventoRegistrado
	e.Erros, e.Alertas = lerRespostaINI(resp, func(secao, key, val string) {
		if secao != "EnviarEvento" {
			return
		}
		switch key {
		case "SucessoCanc":
			e.Sucesso = val == "1"
		case "DescSituacao":
			e.Situacao = val
		case "XmlRetorno":
			e.XML = val
		}
	})
	return e
}

// StatusEvento traduz o registro no vocabulário comum aos módulos.
//
// SucessoCanc só é preenchido pelo provedor no cancelamento (nos demais eventos
// ele fica falso mesmo quando o registro deu certo), então o que decide é a
// ausência de erros; o campo apenas confirma.
func StatusEvento(e EventoRegistrado) string {
	if len(e.Erros) > 0 {
		return "rejeitado"
	}
	return "concluido"
}
