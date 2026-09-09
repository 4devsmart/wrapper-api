package nfse

// CancelamentoPedido é o corpo do pedido de cancelamento (espelha o
// NfsePedidoCancelamento do ACBr.API). Códigos e motivos exigidos variam por
// prefeitura; no Padrão Nacional o código default é 1 (erro na emissão).
type CancelamentoPedido struct {
	Codigo string `json:"codigo,omitempty"` // CodCancelamento (default "1")
	Motivo string `json:"motivo,omitempty"` // MotCancelamento
	// Identificação da nota nos provedores que NÃO são o Padrão Nacional. Eles
	// cancelam por NÚMERO, não por chave, e o ABRASF v2 recusa de saída, sem ir
	// ao webservice, quando o número vem vazio ("Número da NFSe não informado").
	// Alguns ainda exigem a série ou o código de verificação, conforme a
	// configuração do provedor.
	Numero            string `json:"numero,omitempty"`
	Serie             string `json:"serie,omitempty"`
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	// NumeroLote é o número do lote em que a nota foi enviada, exigido por
	// alguns provedores para localizar o documento a cancelar.
	NumeroLote string `json:"numero_lote,omitempty"`
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
//
// NÃO é o caminho do Padrão Nacional: o provedor dele não implementa este
// webservice, e o cancelamento de lá é um evento (ver ToINIEvento).
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
	b.KVOpt("NumeroNFSe", p.Numero)
	b.KVOpt("SerieNFSe", p.Serie)
	b.KVOpt("CodVerificacao", p.CodigoVerificacao)
	b.KVOpt("NumeroLote", p.NumeroLote)
	return b.String()
}

// ParseCancelamento interpreta a resposta INI do ACBr (NFSE_Cancelar). A
// resposta traz uma seção [Cancelamento] (ou [Envio]) com Sucesso/Data/Protocolo
// e seções [ErroN]/[AlertaN].
func ParseCancelamento(resp string) Cancelamento {
	var c Cancelamento
	c.Erros, c.Alertas = lerRespostaINI(resp, func(secao, key, val string) {
		if secao != "Cancelamento" && secao != "Envio" {
			return
		}
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
	})
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
