package mdfe

import "regexp"

// reChaveID casa o atributo Id do infMDFe no XML montado pela lib.
var reChaveID = regexp.MustCompile(`Id="MDFe(\d{44})"`)

// ChaveDoXML extrai a chave de acesso do XML montado.
//
// É o que torna a geração útil: o cliente grava a chave ANTES de qualquer coisa
// ir para a SEFAZ, e por isso consegue descobrir o desfecho de uma transmissão
// cuja resposta se perdeu. Sem a chave, uma transmissão perdida é irrecuperável.
func ChaveDoXML(xml string) string {
	if m := reChaveID.FindStringSubmatch(xml); len(m) == 2 {
		return m[1]
	}
	return ""
}

// AmbienteDoXML devolve "producao"/"homologacao" a partir do tpAmb do XML, ou
// "" se não achar.
//
// Existe para a transmissão NÃO depender de o cliente repetir o ambiente: o XML já
// diz em qual ele foi montado, e divergir disso é rejeição 252 na certa: ou,
// pior, um documento de teste indo para o webservice de produção. O tpAmb da
// SEFAZ é 1=produção e 2=homologação (o oposto do ordinal da ACBrLib).
