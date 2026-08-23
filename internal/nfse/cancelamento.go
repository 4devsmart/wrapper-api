package nfse

import (
	"bufio"
	"strings"
)

// CancelamentoPedido é o corpo do pedido de cancelamento (espelha o
// NfsePedidoCancelamento do ACBr.API). Códigos/motivos exigidos variam por
// prefeitura; no Padrão Nacional o código default é 1 (erro na emissão).
type CancelamentoPedido struct {
	Codigo string `json:"codigo,omitempty"` // CodCancelamento (default "1")
	Motivo string `json:"motivo,omitempty"` // MotCancelamento
}

// Cancelamento é o resultado estruturado de um cancelamento, extraído da
// resposta INI do ACBr (NFSE_Cancelar).
type Cancelamento struct {
	Sucesso   bool       `json:"-"`
	DataHora  string     `json:"data_hora,omitempty"`
	Protocolo string     `json:"protocolo,omitempty"`
	Erros     []Mensagem `json:"mensagens,omitempty"`
	Alertas   []Mensagem `json:"alertas,omitempty"`
}

// ToINICancelamento monta o INI [CancelarNFSe] consumido por NFSE_Cancelar.
// chave é a chave de acesso da NFSe; cMun é o código IBGE do município de
// incidência (default = município do prestador, resolvido pela lib se vazio).
func ToINICancelamento(chave, cMun string, p CancelamentoPedido) string {
	var b iniBuilder
	b.Secao("CancelarNFSe")
	b.KV("ChaveNFSe", chave)
	// CodCancelamento: default 1 (erro na emissão), o código nacional mais comum.
	codigo := p.Codigo
	if codigo == "" {
		codigo = "1"
	}
	b.KV("CodCancelamento", codigo)
	b.KVOpt("MotCancelamento", p.Motivo)
	b.KVOpt("CodMunicipio", cMun)
	return b.String()
}

// ParseCancelamento interpreta a resposta INI do ACBr (NFSE_Cancelar). A
// resposta traz uma seção [Cancelamento] (ou [Envio]) com Sucesso/Data/Protocolo
// e seções [ErroN]/[AlertaN].
func ParseCancelamento(resp string) Cancelamento {
	resp = strings.TrimSpace(resp)
	var c Cancelamento

	if resp == "" {
		return c
	}
	if !strings.HasPrefix(resp, "[") {
		c.Erros = append(c.Erros, Mensagem{Descricao: resp})
		return c
	}

	var secao string
	var msg Mensagem
	flush := func() {
		switch {
		case strings.HasPrefix(secao, "Erro") && (msg.Codigo != "" || msg.Descricao != ""):
			c.Erros = append(c.Erros, msg)
		case strings.HasPrefix(secao, "Alerta") && (msg.Codigo != "" || msg.Descricao != ""):
			c.Alertas = append(c.Alertas, msg)
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
		case "Cancelamento", "Envio":
			switch key {
			case "Sucesso":
				c.Sucesso = val == "1"
			case "Protocolo":
				c.Protocolo = val
			case "DataHora", "Data", "DhRecbto":
				if val != "" {
					c.DataHora = val
				}
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
	return c
}

// StatusCancelamento mapeia o resultado para o enum do ACBr.API
// (pendente | concluido | rejeitado | erro).
func StatusCancelamento(c Cancelamento) string {
	switch {
	case c.Sucesso:
		return "concluido"
	case len(c.Erros) > 0:
		return "rejeitado"
	default:
		return "erro"
	}
}
