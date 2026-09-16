package nfse

import (
	"strings"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

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
	// Identificação pelo RPS que originou a nota, em vez da NFS-e: é assim que
	// parte dos provedores localiza o documento a cancelar.
	NumeroRps int    `json:"numero_rps,omitempty"`
	SerieRps  string `json:"serie_rps,omitempty"` // Série do RPS de origem.
	// Conferências: há provedor que recusa o cancelamento quando a data de
	// emissão ou o valor não batem com a nota que está lá.
	DataEmissao string  `json:"data_emissao,omitempty"`
	Valor       float64 `json:"valor,omitempty"` // Valor total da nota cancelada.
	// Documento do tomador da nota, mais uma conferência pedida por alguns
	// provedores.
	CNPJCPFTomador string `json:"cnpj_cpf_tomador,omitempty"`
	// Código do serviço da nota cancelada.
	CodigoServico string `json:"codigo_servico,omitempty"`
	// Cancelamento COM substituição em uma chamada só, que alguns provedores
	// oferecem aqui. O caminho normal deste produto é o endpoint de
	// substituição, que emite a nota nova; estes campos existem para o
	// município que só aceita a forma combinada.
	NumeroSubstituta string `json:"numero_substituta,omitempty"`
	SerieSubstituta  string `json:"serie_substituta,omitempty"` // Série da nota substituta.
	// Endereço que recebe o aviso de cancelamento, nos provedores que o disparam.
	Email string `json:"email,omitempty"`
}

// Cancelamento é o resultado estruturado de um cancelamento, extraído da
// resposta INI do ACBr (NFSE_Cancelar).
type Cancelamento struct {
	Sucesso bool `json:"-"`
	// XmlRetorno é a resposta do provedor ao pedido ([CancelarNFSe]), que no
	// ABRASF traz o <NfseCancelamento> com a confirmação: é o documento do evento.
	XmlRetorno string     `json:"-"`
	DataHora   string     `json:"data_hora,omitempty"`
	Protocolo  string     `json:"protocolo,omitempty"`
	Erros      []Mensagem `json:"mensagens,omitempty"`
	Alertas    []Mensagem `json:"alertas,omitempty"`
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
	// O resto da seção: identificação pelo RPS, conferências e o cancelamento
	// com substituição. Cada provedor pede um subconjunto disto, e o que não
	// for informado não vai ao INI.
	b.KVIntOpt("NumeroRps", p.NumeroRps)
	b.KVOpt("SerieRps", p.SerieRps)
	b.KVOpt("DataEmissaoNFSe", b.Data(p.DataEmissao))
	b.KVOpt("ValorNFSe", inifmt.MoneyOpt(p.Valor))
	b.KVOpt("CNPJCPFTomador", p.CNPJCPFTomador)
	b.KVOpt("CodServ", p.CodigoServico)
	b.KVOpt("NumeroNFSeSubst", p.NumeroSubstituta)
	b.KVOpt("SerieNFSeSubst", p.SerieSubstituta)
	b.KVOpt("email", p.Email)
	return b.String()
}

// ParseCancelamento interpreta a resposta INI do ACBr (NFSE_Cancelar), com as
// seções [ErroN]/[AlertaN].
//
// O desfecho vem em [RetCancelamento]: é onde a ACBrLib (TCancelarNFSeResposta)
// põe Sucesso, Situacao e DataHora, e o ABRASF v2 grava Sucesso=Sim quando o
// provedor devolve a confirmação. Ler só [Cancelamento]/[Envio] com Sucesso=1
// fazia todo cancelamento ABRASF bem-sucedido voltar como erro 502: a nota era
// cancelada no GISS e o cliente ouvia "desfecho indeterminado".
// As duas seções antigas continuam lidas.
func ParseCancelamento(resp string) Cancelamento {
	var c Cancelamento
	c.Erros, c.Alertas = lerRespostaINI(resp, func(secao, key, val string) {
		switch secao {
		case "CancelarNFSe":
			if key == "XmlRetorno" {
				c.XmlRetorno = val
			}
		case "RetCancelamento":
			switch key {
			case "Sucesso":
				c.Sucesso = c.Sucesso || sucessoDaLib(val)
			case "DataHora":
				if dataPreenchida(val) {
					c.DataHora = val
				}
			}
		case "Cancelamento", "Envio":
			switch key {
			case "Sucesso":
				c.Sucesso = c.Sucesso || val == "1"
			case "Protocolo":
				c.Protocolo = val
			case "DataHora", "Data", "DhRecbto":
				if val != "" {
					c.DataHora = val
				}
			}
		}
	})
	return c
}

// sucessoDaLib aceita as formas que o campo texto Sucesso assume nos provedores:
// "Sim" no ABRASF v2, "1" e "True" em outros.
func sucessoDaLib(val string) bool {
	return strings.EqualFold(val, "sim") || val == "1" || strings.EqualFold(val, "true")
}

// dataPreenchida descarta o TDateTime zerado, que a lib escreve como 30/12/1899.
func dataPreenchida(val string) bool {
	return val != "" && val != "0" && !strings.HasPrefix(val, "30/12/1899")
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
