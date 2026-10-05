package nfse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
)

// NotaDoEvento identifica uma NFS-e envolvida num evento com os mesmos campos em
// qualquer provedor: a nota que o evento atingiu e, na substituição, a que
// nasceu no lugar dela.
//
// É acréscimo ao retorno, e não troca de nenhum campo dele. O `chave` de
// primeiro nível já tem dono: no cancelamento é a nota cancelada, na
// substituição é a substituta, e no ABRASF é o link da nota. Mudar isso
// quebraria quem já lê. Aqui cada nota tem o seu grupo e cada campo diz uma
// coisa só.
//
// Cada campo sai do que houver: o pedido, a chave de acesso (que carrega o
// número da NFS-e no Padrão Nacional) e o XML devolvido. O que não tem de onde
// sair fica fora do JSON, em vez de vir vazio.
type NotaDoEvento struct {
	// Chave de acesso da NFS-e, 50 dígitos no Padrão Nacional. No ABRASF é o link
	// da nota, quando o provedor manda um.
	Chave string `json:"chave,omitempty"`
	// Número da NFS-e atribuído pelo provedor.
	Numero string `json:"numero,omitempty"`
	// Código de verificação da NFS-e, nos provedores que o usam.
	CodigoVerificacao string `json:"codigo_verificacao,omitempty"`
	// Identificador da DPS, com o prefixo "DPS", no Padrão Nacional.
	IDDPS string `json:"id_dps,omitempty"`
	// Número do RPS (nDPS) que originou a nota.
	NumeroRps string `json:"numero_rps,omitempty"`
	// Série do RPS que originou a nota.
	SerieRps string `json:"serie_rps,omitempty"`
}

// reChaveNFSe casa o atributo Id do infNFSe: "NFS" seguido dos 50 dígitos da
// chave de acesso (leiaute do Padrão Nacional).
var reChaveNFSe = regexp.MustCompile(`Id="NFS([0-9]{50})"`)

// ChaveDoXML extrai a chave de acesso do XML de uma NFS-e do Padrão Nacional.
//
// Há provedor que autoriza sem preencher o Link da resposta da lib, que é de
// onde ParseEnvio tira a chave: o ISSNet de Brasília no leiaute nacional é um
// deles. A chave continua no documento.
func ChaveDoXML(xml string) string {
	if m := reChaveNFSe.FindStringSubmatch(xml); len(m) == 2 {
		return m[1]
	}
	return ""
}

// NumeroDaChave extrai o número da NFS-e da chave de acesso do Padrão Nacional:
// município (7), ambiente gerador (1), tipo de inscrição (1), inscrição (14),
// número da NFS-e (13), ano e mês (4), código numérico (9) e DV (1). Qualquer
// outra coisa, como o link do ABRASF, devolve vazio.
func NumeroDaChave(chave string) string {
	chave = strings.TrimSpace(chave)
	if len(chave) != 50 || fiscal.SoDigitos(chave) != chave {
		return ""
	}
	return strings.TrimLeft(chave[23:36], "0")
}

// notaCancelada identifica a nota do cancelamento pelo que o pedido trouxe, com
// o número tirado da chave quando o cliente não o mandou.
func notaCancelada(chave string, e CancelamentoPedido) *NotaDoEvento {
	numeroRps := ""
	if e.NumeroRps > 0 {
		numeroRps = strconv.Itoa(e.NumeroRps)
	}
	return notaOuNada(NotaDoEvento{
		Chave:             strings.TrimSpace(chave),
		Numero:            fiscal.Primeiro(strings.TrimSpace(e.Numero), NumeroDaChave(chave)),
		CodigoVerificacao: strings.TrimSpace(e.CodigoVerificacao),
		NumeroRps:         numeroRps,
		SerieRps:          strings.TrimSpace(e.SerieRps),
	})
}

// notaSubstituida identifica a nota antiga da substituição pelo pedido.
func notaSubstituida(s NFSeSubstituidaRef) *NotaDoEvento {
	return notaOuNada(NotaDoEvento{
		Chave:             strings.TrimSpace(s.Chave),
		Numero:            fiscal.Primeiro(strings.TrimSpace(s.Numero), NumeroDaChave(s.Chave)),
		CodigoVerificacao: strings.TrimSpace(s.CodigoVerificacao),
	})
}

// notaSubstituta identifica a nota nova. Só existe quando o provedor aceitou a
// substituição: na recusa não há nota nova, e um grupo com o RPS dela daria a
// entender o contrário.
func notaSubstituta(em Emissao, xml string, dps DPSPedido) *NotaDoEvento {
	status := statusEmissao(em)
	if status != "autorizado" && status != "processando" {
		return nil
	}
	chave := chaveDaEmissao(em, xml)
	return notaOuNada(NotaDoEvento{
		Chave:             chave,
		Numero:            fiscal.Primeiro(em.Numero, NumeroDaChave(chave)),
		CodigoVerificacao: em.CodigoVerificacao,
		IDDPS:             IDdoXML(xml),
		NumeroRps:         strings.TrimSpace(dps.InfDPS.NDPS),
		SerieRps:          strings.TrimSpace(dps.InfDPS.Serie),
	})
}

// chaveDaEmissao é a chave que a lib informou e, quando ela não informa, a do
// XML da nota autorizada. Só completa: o que a lib manda continua valendo, e
// sem autorização não há nota de onde tirar chave.
func chaveDaEmissao(em Emissao, xml string) string {
	if em.Chave != "" || !em.Sucesso {
		return em.Chave
	}
	return ChaveDoXML(xml)
}

// reJaCancelada casa a recusa por nota já cancelada, do jeito que as
// prefeituras escrevem: "NFS-e já está cancelada" (EM097 no Padrão Nacional),
// "Nota fiscal já cancelada", "já se encontra cancelada", "encontra-se
// cancelada". Vai pelo texto porque o código muda de provedor para provedor.
var reJaCancelada = regexp.MustCompile(`(?i)(j[aá]\s+(est[aá]\s+|foi\s+|se\s+encontra\s+)?|encontra-se\s+)cancelad`)

// JaCancelada diz se alguma das mensagens recusa o cancelamento porque a nota
// já estava cancelada.
func JaCancelada(erros []Mensagem) bool {
	for _, m := range erros {
		if strings.EqualFold(strings.TrimSpace(m.Codigo), "EM097") || reJaCancelada.MatchString(m.Descricao) {
			return true
		}
	}
	return false
}

func notaOuNada(n NotaDoEvento) *NotaDoEvento {
	if n == (NotaDoEvento{}) {
		return nil
	}
	return &n
}
