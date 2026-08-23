package mdfe

import (
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// PedidoEncerramento é o corpo de POST /v1/mdfe/eventos/encerramento (ACBr.API).
// chMDFe e nProt vêm do documento autorizado.
type PedidoEncerramento struct {
	DataEncerramento string `json:"data_encerramento,omitempty" fmt:"data"` // ISO; default hoje
	CUF              int    `json:"cUF,omitempty"`                          // UF do encerramento
	CMun             string `json:"cMun"`                                   // município do encerramento
}

// PedidoCancelamento é o corpo de POST /v1/mdfe/eventos/cancelamento (ACBr.API).
type PedidoCancelamento struct {
	Justificativa string `json:"justificativa"`
}

// PedidoInclusaoCondutor: POST /v1/mdfe/eventos/inclusao-condutor (evento 110114).
type PedidoInclusaoCondutor struct {
	Nome       string `json:"nome"`
	CPF        string `json:"cpf"`
	NSeqEvento int    `json:"nSeqEvento,omitempty"`
}

// PedidoInclusaoDFe: POST /v1/mdfe/eventos/inclusao-dfe (evento 110115). Inclui novos
// documentos (NF-e) ao manifesto em trânsito, por município de descarga.
type PedidoInclusaoDFe struct {
	CMunCarrega string        `json:"cMunCarrega,omitempty"`
	XMunCarrega string        `json:"xMunCarrega,omitempty"`
	Documentos  []DocInclusao `json:"documentos"`
	NSeqEvento  int           `json:"nSeqEvento,omitempty"`
}

// DocInclusao é uma NF-e a incluir (seção [infDocNNNN]).
type DocInclusao struct {
	ChNFe        string `json:"chNFe"`
	CMunDescarga string `json:"cMunDescarga"`
	XMunDescarga string `json:"xMunDescarga"`
}

// eventoHeader monta o cabeçalho INDEXADO que o MDFe.LerFromIni espera:
// [EVENTO] (idLote) + [EVENTO001] com os campos comuns (TODO o detEvento fica na
// própria [EVENTO001]; não há [detEvento]). cOrgao = 2 primeiros dígitos da
// chave. No fluxo CarregarEventoINI a lib NÃO auto-preenche cOrgao/CNPJCPF/
// dhEvento. Ver .claude/skills/acbr-especialista/referencia/mdfe-eventos.md.
func eventoHeader(b *iniBuilder, chave, cnpj, dhEvento, tpEvento string, nSeq int) {
	cOrgao := ""
	if len(chave) >= 2 {
		cOrgao = chave[:2]
	}
	b.section("EVENTO")
	b.kv("idLote", "1")
	b.section("EVENTO001")
	b.kv("chMDFe", chave)
	b.kvOpt("cOrgao", cOrgao)
	b.kvOpt("CNPJCPF", cnpj)
	b.kvOpt("dhEvento", dhEvento)
	b.kv("tpEvento", tpEvento)
	if nSeq <= 0 {
		nSeq = 1
	}
	b.kv("nSeqEvento", strconv.Itoa(nSeq))
}

// ToINIEncerramento monta o INI do encerramento (tpEvento 110112), indexado.
func ToINIEncerramento(chave, cnpj, protocolo, dhEvento string, p PedidoEncerramento) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110112", 1)
	b.kv("dtEnc", b.data(p.DataEncerramento))
	b.kvIntOpt("cUF", p.CUF)
	b.kvOpt("cMun", p.CMun)
	b.kvOpt("nProt", protocolo)
	return b.String()
}

// ToINICancelamento monta o INI do cancelamento (tpEvento 110111), indexado.
func ToINICancelamento(chave, cnpj, protocolo, dhEvento string, p PedidoCancelamento) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110111", 1)
	b.kvOpt("nProt", protocolo)
	b.kv("xJust", p.Justificativa)
	return b.String()
}

// ToINIInclusaoCondutor monta o INI do evento 110114 (xNome/CPF na [EVENTO001]).
func ToINIInclusaoCondutor(chave, cnpj, dhEvento string, p PedidoInclusaoCondutor) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110114", p.NSeqEvento)
	b.kv("xNome", p.Nome)
	b.kv("CPF", p.CPF)
	return b.String()
}

// ToINIInclusaoDFe monta o INI do evento 110115: cMunCarrega/xMunCarrega na
// [EVENTO001] + uma seção [infDoc0001..] (4 dígitos) por NF-e incluída.
func ToINIInclusaoDFe(chave, cnpj, dhEvento string, p PedidoInclusaoDFe) string {
	var b iniBuilder
	eventoHeader(&b, chave, cnpj, dhEvento, "110115", p.NSeqEvento)
	b.kvOpt("cMunCarrega", p.CMunCarrega)
	b.kvOpt("xMunCarrega", p.XMunCarrega)
	for i, d := range p.Documentos {
		b.section("infDoc" + seq4(i+1))
		b.kv("chNFe", d.ChNFe)
		b.kvOpt("cMunDescarga", d.CMunDescarga)
		b.kvOpt("xMunDescarga", d.XMunDescarga)
	}
	return b.String()
}

// seq4 formata índice com 4 dígitos (ex.: [infDoc0001]).
func seq4(n int) string { return inifmt.Seq4(n) }

// Evento é o resultado estruturado de um evento (encerramento/cancelamento).
// cStat 135 = evento registrado e vinculado ao MDF-e.
type Evento struct {
	Sucesso   bool       `json:"sucesso"`
	CStat     string     `json:"cstat,omitempty"`
	XMotivo   string     `json:"xmotivo,omitempty"`
	Protocolo string     `json:"protocolo,omitempty"`
	Erros     []Mensagem `json:"erros,omitempty"`
}

var statusEventoOK = map[string]bool{"135": true, "136": true}

// ParseEvento interpreta a resposta INI de um evento.
func ParseEvento(resp string) Evento {
	e := ParseEnvio(resp) // reaproveita a varredura (cStat/xMotivo/nProt/erros)
	ev := Evento{CStat: e.CStat, XMotivo: e.XMotivo, Protocolo: e.Protocolo, Erros: e.Erros}
	ev.Sucesso = statusEventoOK[e.CStat]
	return ev
}

// StatusEvento mapeia o resultado para o status do evento persistido.
func StatusEvento(e Evento) string {
	switch {
	case e.Sucesso:
		return "concluido"
	case len(e.Erros) > 0 || e.CStat != "":
		return "rejeitado"
	default:
		return "erro"
	}
}

// data é a data (sem hora) no fuso do documento. Vazio = hoje.
func (b *iniBuilder) data(s string) string {
	dt := b.dataHora(s) // "DD/MM/YYYY HH:MM:SS"
	if len(dt) >= 10 {
		return dt[:10]
	}
	return dt
}
