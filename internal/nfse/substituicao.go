package nfse

import (
	"strings"
	"unicode/utf8"
)

// SubstituicaoPedido é o corpo de POST /v1/nfse/eventos/substituicao: a DPS substituta
// (nova nota) + os identificadores da NFS-e antiga + o motivo do cancelamento.
//
// São dois caminhos, e quem escolhe é o provedor do município. No Padrão
// Nacional não existe webservice de substituição: a nota nova aponta a chave da
// substituída, e é a emissão dela que substitui a antiga; identifique a antiga
// em substituida.chave. Nos provedores ABRASF e próprios que oferecem o
// webservice de substituição a nota antiga é identificada por número, série e
// código de verificação.
type SubstituicaoPedido struct {
	DPS         DPSPedido          `json:"dps"`         // nota nova (substituta)
	Substituida NFSeSubstituidaRef `json:"substituida"` // identifica a NFS-e antiga
	// Codigo é o código da justificativa da substituição. As listas são
	// DIFERENTES por caminho. No Padrão Nacional é obrigatório e tem dois dígitos: 01
	// (desenquadramento do Simples Nacional), 02 (enquadramento no Simples
	// Nacional), 03 (inclusão retroativa de imunidade ou isenção), 04 (exclusão
	// retroativa), 05 (rejeição pelo tomador ou intermediário) ou 99 (outros).
	// Nos demais provedores é o código de cancelamento da prefeitura, e o
	// default é 1.
	Codigo string `json:"codigo,omitempty"`
	Motivo string `json:"motivo,omitempty"`
}

// NFSeSubstituidaRef identifica a NFS-e que está sendo substituída. Os valores
// vêm da emissão original (número/série/cód. verificação retornados pelo provedor).
//
// No Padrão Nacional nada disso existe: lá a substituída é apontada pela chave
// de acesso, e só por ela.
type NFSeSubstituidaRef struct {
	Chave             string `json:"chave,omitempty"`
	Numero            string `json:"numero"`
	Serie             string `json:"serie,omitempty"`
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	NumeroLote        string `json:"numero_lote,omitempty"`
}

// codigosSubstituicaoPN são os cMotivo do grupo subst (TSCodJustSubst). NÃO são
// os do evento de cancelamento: aqui vão dois dígitos e a lista é outra, então
// um "1" copiado de lá viraria "01", que é um motivo DIFERENTE e válido.
var codigosSubstituicaoPN = map[string]string{
	"01": "desenquadramento do Simples Nacional",
	"02": "enquadramento no Simples Nacional",
	"03": "inclusão retroativa de imunidade/isenção",
	"04": "exclusão retroativa de imunidade/isenção",
	"05": "rejeição pelo tomador ou intermediário",
	"99": "outros",
}

// ToINISubstituicaoPN monta o intermediário da DPS substituta do Padrão
// Nacional: a nota nova inteira mais o grupo [NFSeSubstituicao], que a
// biblioteca lê para gerar o elemento subst do XML (ACBrNFSeX.LerIni.pas:494).
//
// A emissão desta DPS É a substituição: não há chamada de evento depois, e é o
// ADN que cancela a nota antiga por substituição.
func ToINISubstituicaoPN(p SubstituicaoPedido) string {
	var b iniBuilder
	b.Secao("NFSeSubstituicao")
	b.KV("chSubstda", p.Substituida.Chave)
	b.KV("cMotivo", p.Codigo)
	b.KVOpt("xMotivo", strings.TrimSpace(p.Motivo))
	return ToINI(p.DPS) + "\n" + b.String()
}

// ValidarSubstituicaoPN confere o que o schema do grupo subst exige ANTES de
// transmitir, e devolve a frase do erro (vazia se está tudo certo).
//
// O código não tem default de propósito. A lista é de motivos fiscais
// específicos, e o StrTocMotivo da lib converte o que não reconhece no PRIMEIRO
// da enumeração ('01', desenquadramento do Simples): escolher por conta própria
// aqui seria emitir uma nota com uma justificativa que ninguém pediu.
func ValidarSubstituicaoPN(p SubstituicaoPedido) string {
	if strings.TrimSpace(p.Substituida.Chave) == "" {
		return "evento.substituida.chave (chave de acesso da NFS-e substituída) é obrigatória no Padrão Nacional"
	}
	if _, ok := codigosSubstituicaoPN[p.Codigo]; !ok {
		return "evento.codigo é obrigatório no Padrão Nacional: 01 (desenquadramento do Simples), " +
			"02 (enquadramento no Simples), 03 (inclusão retroativa de imunidade/isenção), " +
			"04 (exclusão retroativa), 05 (rejeição pelo tomador ou intermediário) ou 99 (outros)"
	}
	if motivo := strings.TrimSpace(p.Motivo); motivo != "" {
		if n := utf8.RuneCountInString(motivo); n < motivoMin || n > motivoMax {
			return "evento.motivo, quando informado, precisa ter de 15 a 255 caracteres"
		}
	}
	return ""
}
