package nfse

import (
	"bufio"
	"strings"
)

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
// implementado para este provedor" — ou seja, o provedor de NFS-e do município
// não oferece a operação (cancelamento/substituição/etc.) via webservice.
//
// É detectado em RUNTIME, não por uma tabela estática: a engine multi-provedor
// resolve a capacidade no momento da chamada (cada provedor implementa um
// subconjunto de operações; substituição, p.ex., existe em parte dos provedores
// ABRASF e não em outros). O marcador é a mensagem de "não implementado".
func OperacaoNaoSuportada(resp string) bool {
	s := strings.ToLower(resp)
	return strings.Contains(s, "implementado") && strings.Contains(s, "provedor")
}

// ParseEnvio interpreta a resposta INI do ACBr (NFSE_Emitir) em uma Emissao.
// Quando a resposta não é INI (ex.: mensagem de erro pura), devolve um erro
// genérico no campo Erros.
func ParseEnvio(resp string) Emissao {
	resp = strings.TrimSpace(resp)
	var e Emissao

	if resp == "" {
		return e
	}
	if !strings.HasPrefix(resp, "[") {
		// Resposta não-INI (ex.: exceção em texto): trata como erro único.
		e.Erros = append(e.Erros, Mensagem{Descricao: resp})
		return e
	}

	var secao string
	var msg Mensagem
	flush := func() {
		switch {
		case strings.HasPrefix(secao, "Erro") && (msg.Codigo != "" || msg.Descricao != ""):
			e.Erros = append(e.Erros, msg)
		case strings.HasPrefix(secao, "Alerta") && (msg.Codigo != "" || msg.Descricao != ""):
			e.Alertas = append(e.Alertas, msg)
		}
		msg = Mensagem{}
	}

	sc := bufio.NewScanner(strings.NewReader(resp))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			secao = line[1 : len(line)-1]
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)

		switch secao {
		case "Envio":
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
		default:
			if strings.HasPrefix(secao, "Erro") || strings.HasPrefix(secao, "Alerta") {
				switch key {
				case "Codigo":
					msg.Codigo = val
				case "Descricao":
					msg.Descricao = val
				}
			}
		}
	}
	flush()
	return e
}
