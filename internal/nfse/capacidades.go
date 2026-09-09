package nfse

import "strings"

// Capacidades é o que o provedor de NFS-e do município DE FATO expõe, lido da
// própria lib (NFSE_ObterInformacoesProvedor).
//
// Existe porque a capacidade não é do município, é da implementação do provedor
// dentro da biblioteca fiscal. Dizer "o município não oferece cancelamento"
// quando quem não oferece é a lib manda o cliente ligar para a prefeitura
// errada.
type Capacidades struct {
	Provedor   string   `json:"provedor,omitempty"`
	Servicos   []string `json:"servicos,omitempty"`
	Autenticac []string `json:"autenticacoes,omitempty"`
}

// Serviços de ServicosDisponibilizados que este módulo consulta pelo nome
// (ACBrLibNFSeRespostas.pas:624 em diante).
const (
	ServicoCancelar      = "CancelarNfse"
	ServicoSubstituir    = "SubstituirNfse"
	ServicoEnviarEvento  = "EnviarEvento"
	ServicoConsultarNFSe = "ConsultarNfse"
)

// Oferece diz se o provedor declara o serviço.
func (c Capacidades) Oferece(servico string) bool {
	for _, s := range c.Servicos {
		if strings.EqualFold(s, servico) {
			return true
		}
	}
	return false
}

// ParseCapacidades interpreta a seção [ObterInformacoesProvedor]. Os três
// campos são listas separadas por "|", e o de identificação vem como
// "Nome:x|Versão:y|Layout:z".
func ParseCapacidades(resp string) Capacidades {
	var c Capacidades
	lerRespostaINI(resp, func(secao, key, val string) {
		if secao != "ObterInformacoesProvedor" {
			return
		}
		switch key {
		case "IdentificacaoProvedor":
			c.Provedor = strings.TrimPrefix(primeiroCampo(val), "Nome:")
		case "ServicosDisponibilizados":
			c.Servicos = campos(val)
		case "AutenticacoesRequeridas":
			c.Autenticac = campos(val)
		}
	})
	return c
}

// Operacoes diz quais eventos DESTA API funcionam no município: o que a lib
// declara, cruzado com o caminho que cada layout usa.
//
// Repassar a lista crua do provedor daria a resposta errada justamente onde o
// bug estava. O Padrão Nacional declara CancelarNfse=False, e é verdade: ele
// não tem esse webservice. Mas cancela, por evento. Quem lesse só a lista
// concluiria que Porto Alegre não cancela, que é exatamente a frase que
// mandamos consertar.
func Operacoes(layout Layout, c Capacidades) map[string]bool {
	if layout == LayoutPadraoNacional {
		return map[string]bool{
			"cancelamento": c.Oferece(ServicoEnviarEvento),
			// A substituição do PN não é webservice nenhum: é o grupo subst
			// dentro da DPS, e vai junto com a emissão da nota nova.
			"substituicao": true,
		}
	}
	return map[string]bool{
		"cancelamento": c.Oferece(ServicoCancelar),
		"substituicao": c.Oferece(ServicoSubstituir),
	}
}

// campos quebra a lista "A|B|C|" da lib, descartando o vazio do separador final.
func campos(val string) []string {
	var out []string
	for _, s := range strings.Split(val, "|") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func primeiroCampo(val string) string {
	if c := campos(val); len(c) > 0 {
		return c[0]
	}
	return ""
}
