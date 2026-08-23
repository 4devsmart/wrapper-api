package cte

import "regexp"

// reChaveID casa o atributo Id do infCte no XML montado pela lib. Vale para
// CT-e, CT-e OS, CT-e Simplificado e GTV-e: todos usam infCte com Id="CTe"+44.
var reChaveID = regexp.MustCompile(`Id="CTe(\d{44})"`)

// ChaveDoXML extrai a chave de acesso do XML montado.
//
// É o que torna a fase 1 útil: o cliente grava a chave ANTES de qualquer coisa
// ir para a SEFAZ, e por isso consegue descobrir o desfecho de uma transmissão
// cuja resposta se perdeu. Sem a chave, uma transmissão perdida é irrecuperável.
func ChaveDoXML(xml string) string {
	if m := reChaveID.FindStringSubmatch(xml); len(m) == 2 {
		return m[1]
	}
	return ""
}

// reTpAmb casa a tag tpAmb do XML.
var reTpAmb = regexp.MustCompile(`<tpAmb>\s*([12])\s*</tpAmb>`)

// AmbienteDoXML devolve "producao"/"homologacao" a partir do tpAmb do XML, ou
// "" se não achar.
//
// Existe para a fase 2 NÃO depender de o cliente repetir o ambiente: o XML já
// diz em qual ele foi montado, e divergir disso é rejeição 252 na certa — ou,
// pior, um documento de teste indo para o webservice de produção. O tpAmb da
// SEFAZ é 1=produção e 2=homologação (o oposto do ordinal da ACBrLib, que é
// 0=produção; a conversão fica em fiscal.AmbienteOrdinal).
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
