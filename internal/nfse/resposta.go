package nfse

import "strings"

// Emissao é o resultado estruturado de uma emissão, extraído da resposta do
// ACBr (formato INI com seções [Envio] e [ErroN]/[AlertaN]).
type Emissao struct {
	Sucesso           bool   `json:"sucesso"`
	Numero            string `json:"numero,omitempty"`
	Chave             string `json:"chave,omitempty"` // chave de acesso (Link)
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	Protocolo         string `json:"protocolo,omitempty"`
	Situacao          string `json:"situacao,omitempty"`
	DataProcessamento string `json:"data_processamento,omitempty"`
	// ModoEnvio é o modo que o provedor de fato usou, já resolvido pela lib
	// ("Enviar Lote Assíncrono", "Gerar NFSe"...), e não o pedido, que é sempre
	// automático.
	ModoEnvio string     `json:"modo_envio,omitempty"`
	Erros     []Mensagem `json:"erros,omitempty"`
	Alertas   []Mensagem `json:"alertas,omitempty"`
}

// AguardaProcessamento diz que o provedor só RECEBEU o lote. No envio
// assíncrono (GISS 2.04, entre outros) o Sucesso=1 da lib quer dizer "lote
// aceito, eis o protocolo", e a nota nasce, ou é recusada, depois. Sem número de
// nota, chamar isso de autorizado grava como emitida uma nota que talvez nunca
// exista. O desfecho sai de POST /transmissao/lote, pelo protocolo.
func (e Emissao) AguardaProcessamento() bool {
	return e.Numero == "" && strings.Contains(strings.ToLower(e.ModoEnvio), "lote ass")
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
		case "ModoEnvio":
			e.ModoEnvio = val
		}
	})
	return e
}

// LoteRps é o desfecho de um lote consultado pelo protocolo (NFSE_ConsultarLoteRps).
type LoteRps struct {
	// Situacao é a do ABRASF: 1 não recebido, 2 não processado, 3 processado com
	// erro, 4 processado com sucesso. Vazia quando o provedor não a informa.
	Situacao          string
	DescSituacao      string
	Protocolo         string
	Numero            string
	CodigoVerificacao string
	Erros             []Mensagem
	Alertas           []Mensagem
}

// ParseLoteRps lê a seção [ConsultaLoteRps] e a [Arquivo1], que é onde a lib
// põe o número e o código de verificação da nota que o lote gerou. O envio
// desta API leva um RPS por lote, então há no máximo um arquivo.
func ParseLoteRps(resp string) LoteRps {
	var l LoteRps
	l.Erros, l.Alertas = lerRespostaINI(resp, func(secao, key, val string) {
		switch secao {
		case "ConsultaLoteRps":
			switch key {
			case "Situacao":
				l.Situacao = val
			case "DescSituacao":
				l.DescSituacao = val
			case "Protocolo":
				l.Protocolo = val
			case "CodVerificacao":
				if val != "" {
					l.CodigoVerificacao = val
				}
			}
		case "Arquivo1":
			switch key {
			case "NumeroNota":
				l.Numero = val
			case "CodigoVerificacao":
				if val != "" {
					l.CodigoVerificacao = val
				}
			}
		}
	})
	return l
}

// Status traduz o lote no vocabulário da transmissão.
//
// A nota que voltou decide antes da situação: é ela que prova a autorização.
// Situação 4 sem nota não é sucesso que se possa gravar, e situação ausente não
// diz nada; os dois caem em erro, que não conclui a nota do lado do cliente.
func (l LoteRps) Status() string {
	switch {
	case l.Numero != "":
		return "autorizado"
	case l.Situacao == "1" || l.Situacao == "2":
		return "processando"
	case l.Situacao == "3":
		return "rejeitado"
	default:
		return "erro"
	}
}
