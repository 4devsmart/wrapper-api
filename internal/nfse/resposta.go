package nfse

import "strings"

// Emissao é o resultado estruturado de uma emissão, extraído da resposta do
// ACBr (formato INI com seções [Envio] e [ErroN]/[AlertaN]).
type Emissao struct {
	Sucesso           bool       `json:"sucesso"`
	Numero            string     `json:"numero,omitempty"`
	Chave             string     `json:"chave,omitempty"` // chave de acesso (Link)
	CodigoVerificacao string     `json:"codigo_verificacao,omitempty"`
	Protocolo         string     `json:"protocolo,omitempty"`
	Situacao          string     `json:"situacao,omitempty"`
	DataProcessamento string     `json:"data_processamento,omitempty"`
	Erros             []Mensagem `json:"erros,omitempty"`
	Alertas           []Mensagem `json:"alertas,omitempty"`
}

// Mensagem é um erro ou alerta com código e descrição.
type Mensagem struct {
	Codigo    string `json:"codigo,omitempty"`
	Descricao string `json:"descricao,omitempty"`
}

// OperacaoNaoSuportada indica que a resposta da lib é o erro de "serviço não
// implementado para este provedor": a CLASSE do provedor, dentro da biblioteca
// fiscal, não implementa aquela operação.
//
// Duas mensagens caem aqui, e as duas são decisão LOCAL, tomada antes de
// qualquer byte sair para a prefeitura:
//
//   - ERR_NAO_IMP, "Serviço %s não implementado para este provedor", levantada
//     como exceção pelo provedor próprio (ACBrNFSeXProviderProprio.pas);
//   - Desc001, "Serviço não implementado pelo Provedor", que vem como [Erro1]
//     na resposta INI de quem sobrescreve o método só para recusar.
//
// É o que responde em ~300 ms e não deve ser confundido com recusa da
// prefeitura. Nunca conclua daqui que o MUNICÍPIO não oferece o serviço: no
// Padrão Nacional o cancelamento existe, por evento, e era esta mensagem que
// aparecia quando chamávamos o webservice errado.
func OperacaoNaoSuportada(resp string) bool {
	s := strings.ToLower(resp)
	return strings.Contains(s, "implementado") && strings.Contains(s, "provedor")
}

// ParseEnvio interpreta a resposta INI do ACBr (NFSE_Emitir) em uma Emissao.
// Quando a resposta não é INI (ex.: mensagem de erro pura), devolve um erro
// genérico no campo Erros.
func ParseEnvio(resp string) Emissao {
	var e Emissao
	e.Erros, e.Alertas = lerRespostaINI(resp, func(secao, key, val string) {
		if secao != "Envio" {
			return
		}
		switch key {
		case "Sucesso":
			e.Sucesso = val == "1"
		case "NumeroNota":
			e.Numero = val
		case "Link":
			e.Chave = val
		case "CodigoVerificacao":
			e.CodigoVerificacao = val
		case "Protocolo":
			e.Protocolo = val
		case "Situacao", "DescSituacao":
			if val != "" {
				e.Situacao = val
			}
		case "Data":
			e.DataProcessamento = val
		}
	})
	return e
}
