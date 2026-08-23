package nfse

import (
	"regexp"
	"strings"
)

// reIDdps casa o atributo Id do infDPS no XML montado pela lib. O layout do
// Padrão Nacional é Id="DPS" + chave (PadraoNacional.GravarXml.pas:473).
var reIDdps = regexp.MustCompile(`Id="(DPS[0-9]+)"`)

// IDdoXML extrai o identificador da DPS do XML montado (com o prefixo "DPS").
//
// É o equivalente da chave de acesso para a NFS-e, e cumpre o mesmo papel: o
// cliente o grava ANTES de qualquer coisa ir para o provedor, e é com ele que
// descobre o desfecho de uma transmissão cuja resposta se perdeu — via
// ConsultarDPSPorChave. Sem ele, uma transmissão perdida é irrecuperável.
func IDdoXML(xml string) string {
	if m := reIDdps.FindStringSubmatch(xml); len(m) == 2 {
		return m[1]
	}
	return ""
}

// ChaveDPS normaliza o identificador para o que o webservice espera: a chave SEM
// o prefixo "DPS".
//
// O atributo Id do XML vem com o prefixo, mas o ADN atende GET /dps/{chave}
// (PadraoNacional.Provider.pas:503) com a chave nua. Aceitar as duas formas
// poupa o cliente de saber dessa diferença — ele devolve o que recebeu da fase 1.
func ChaveDPS(id string) string {
	return strings.TrimPrefix(strings.TrimSpace(id), "DPS")
}

var reTpAmb = regexp.MustCompile(`<tpAmb>\s*([12])\s*</tpAmb>`)

// AmbienteDoXML devolve "producao"/"homologacao" a partir do tpAmb do XML, ou
// "" se não achar. O tpAmb da SEFAZ é 1=produção e 2=homologação — o oposto do
// ordinal da ACBrLib, que é 0=produção.
func AmbienteDoXML(xml string) string {
	m := reTpAmb.FindStringSubmatch(xml)
	if len(m) != 2 {
		return ""
	}
	if m[1] == "1" {
		return "producao"
	}
	return "homologacao"
}
