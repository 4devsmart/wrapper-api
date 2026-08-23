package cte

import "strconv"

// PedidoCancelamento é o corpo de POST /v1/cte/eventos/cancelamento (ACBr.API).
// A chave e o protocolo vêm no envelope do evento, não daqui: sem estado no
// servidor, quem os guarda é o cliente.
type PedidoCancelamento struct {
	Justificativa string `json:"justificativa"`
	// nSeqEvento é opcional (default 1).
	NSeqEvento int `json:"nSeqEvento,omitempty"`
}

// eventoHeader monta o cabeçalho do evento no formato INDEXADO que o
// ACBrCTe.LerFromIni espera: [EVENTO] (idLote) + [EVENTO001] com os campos comuns.
// cOrgao = 2 primeiros dígitos da chave (cUF). No fluxo CarregarEventoINI a lib
// NÃO auto-preenche cOrgao/CNPJ/dhEvento, então informamos tudo.
// Ver .claude/skills/acbr-especialista/referencia/cte-eventos.md.
func eventoHeader(b *iniBuilder, chave, cnpj, dhEvento, tpEvento string, nSeq int) {
	cOrgao := ""
	if len(chave) >= 2 {
		cOrgao = chave[:2]
	}
	b.section("EVENTO")
	b.kv("idLote", "1")
	b.section("EVENTO001")
	b.kv("chCTe", chave)
	b.kvOpt("cOrgao", cOrgao)
	b.kvOpt("CNPJ", cnpj)
	b.kvOpt("dhEvento", dhEvento)
	b.kv("tpEvento", tpEvento)
	b.kv("nSeqEvento", strconv.Itoa(defaultInt(nSeq, 1)))
}

// ToINICancelamento monta o INI do evento de cancelamento (tpEvento 110111) no
// formato indexado consumido pela ACBrLibCTe. chCTe/nProt vêm do documento
// autorizado; cnpj é o emitente; dhEvento é a data/hora do evento (local).
func ToINICancelamento(chave, cnpj, protocolo, dhEvento string, p PedidoCancelamento) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110111", p.NSeqEvento)
	b.kvOpt("nProt", protocolo)
	b.kv("xJust", p.Justificativa)
	return b.String()
}

// Cancelamento é o resultado estruturado de um cancelamento (cStat 135 =
// evento registrado e vinculado).
type Cancelamento struct {
	Sucesso   bool       `json:"sucesso"`
	CStat     string     `json:"cstat,omitempty"`
	XMotivo   string     `json:"xmotivo,omitempty"`
	Protocolo string     `json:"protocolo,omitempty"`
	DataHora  string     `json:"data_hora,omitempty"`
	Erros     []Mensagem `json:"erros,omitempty"`
}

// statusEventoOK são os cStat de sucesso de um evento de cancelamento.
var statusEventoOK = map[string]bool{"135": true, "136": true, "155": true}

// ParseCancelamento interpreta a resposta INI do cancelamento.
func ParseCancelamento(resp string) Cancelamento {
	e := ParseEnvio(resp) // reaproveita a varredura genérica (cStat/xMotivo/erros)
	c := Cancelamento{
		CStat:     e.CStat,
		XMotivo:   e.XMotivo,
		Protocolo: e.Protocolo,
		Erros:     e.Erros,
	}
	c.Sucesso = statusEventoOK[e.CStat]
	return c
}

// StatusCancelamento mapeia o resultado para o status do evento persistido.
func StatusCancelamento(c Cancelamento) string {
	switch {
	case c.Sucesso:
		return "concluido"
	case len(c.Erros) > 0 || c.CStat != "":
		return "rejeitado"
	default:
		return "erro"
	}
}
