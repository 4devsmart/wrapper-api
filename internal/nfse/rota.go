package nfse

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
)

// Rota é por onde uma operação da NFS-e sai. É aqui, e só aqui, que se decide
// isso: os handlers recebem a rota pronta.
//
// Existe porque o caminho deixou de ser função só do município. A lib escolhe a
// classe do provedor pelo CodigoMunicipio da sessão, e em Brasília o mesmo
// município sai pelo ISSNet ou pelo Ambiente Nacional conforme o emitente, a
// competência e o sistema onde a nota já está. Daí os dois campos: o município
// do documento não muda, o da sessão pode mudar.
type Rota struct {
	// Municipio é o do documento, o que vai no cLocEmi e na chave.
	Municipio string
	// Sessao é o que abre a sessão na lib e, com isso, escolhe a classe.
	Sessao string
	// Layout é o da entrada: decide o INI e o caminho dos eventos.
	Layout Layout
	// Motivo diz por que a rota é esta. Não decide nada: é para o log.
	Motivo MotivoRota
}

// MotivoRota é o que levou a operação à rota.
type MotivoRota string

const (
	motivoTabela       MotivoRota = "tabela"       // o provedor do município
	motivoVigencia     MotivoRota = "vigencia"     // uma linha de vigencias
	motivoChave        MotivoRota = "chave"        // a chave diz que a nota é nacional
	motivoConsultaDPS  MotivoRota = "consulta-dps" // a DPS foi procurada no nacional
	motivoDistribuicao MotivoRota = "distribuicao" // sem distribuição no provedor
)

// RotaDoMunicipio é o ponto de partida: o caminho que a tabela informa para o
// município. Falso se ele não tem provedor conhecido.
func RotaDoMunicipio(cmun string) (Rota, bool) {
	c := fiscal.SoDigitos(cmun)
	l, ok := LayoutDoMunicipio(c)
	return Rota{Municipio: c, Sessao: c, Layout: l, Motivo: motivoTabela}, ok
}

// Provedor é quem de fato atende por esta rota.
func (r Rota) Provedor() string { return provedorDoMunicipio(r.Sessao) }

func (r Rota) nacional(motivo MotivoRota) Rota {
	r.Sessao, r.Layout, r.Motivo = SessaoNacional, LayoutPadraoNacional, motivo
	return r
}

// DaEmissao é a rota de uma DPS pelo pedido.
func (r Rota) DaEmissao(p DPSPedido) Rota {
	if dpsPeloAmbienteNacional(r.Municipio, p) {
		return r.nacional(motivoVigencia)
	}
	return r
}

// DaDPSPronta é a rota da transmissão, que só recebe a DPS pronta. O regime e a
// competência estão no próprio documento, então o cliente não informa nada a
// mais, e uma DPS gerada antes desta regra também acerta o caminho.
func (r Rota) DaDPSPronta(xml string) Rota {
	op, dCompet := 0, ""
	if m := reOpSimpNac.FindStringSubmatch(xml); len(m) == 2 {
		op, _ = strconv.Atoi(m[1])
	}
	if m := reDCompet.FindStringSubmatch(xml); len(m) == 2 {
		dCompet = m[1]
	}
	if PeloAmbienteNacional(r.Municipio, op, dCompet) {
		return r.nacional(motivoVigencia)
	}
	return r
}

// DaNota é a rota de uma nota já emitida. Num município da lista convivem notas
// nos dois sistemas: as do Simples até outubro e as de quem não é do Simples no
// provedor, as novas no Sistema Nacional. É a chave que diz onde a nota está, sem
// nada a mais no pedido.
func (r Rota) DaNota(chave string) Rota {
	chave = strings.TrimSpace(chave)
	if !temDoisCaminhos(r.Municipio) || len(chave) != 50 || fiscal.SoDigitos(chave) != chave {
		return r
	}
	if chave[7] == ambGerSistemaNacional {
		return r.nacional(motivoChave)
	}
	return r
}

// DoEvento é a rota de um evento. Na substituição a nota nova decide primeiro: se
// ela só pode sair pelo Ambiente Nacional, é por lá que a substituição tem de ir,
// qualquer que seja a antiga. Evento malformado fica na regra da chave, e a recusa
// sai na validação do próprio evento.
func (r Rota) DoEvento(tipo string, p PedidoEvento) Rota {
	chave := p.Chave
	if tipo == "substituicao" {
		var e SubstituicaoPedido
		if json.Unmarshal(p.Evento, &e) == nil {
			if dpsPeloAmbienteNacional(r.Municipio, e.DPS) {
				return r.nacional(motivoVigencia)
			}
			chave = fiscal.Primeiro(e.Substituida.Chave, chave)
		}
	}
	return r.DaNota(chave)
}

// RotasDaConsultaDPS são as rotas onde procurar uma DPS, em ordem. A chave da
// DPS não diz em que sistema ela foi: só a da NFS-e diz. Num município com os dois
// caminhos, o Ambiente Nacional responde primeiro.
func (r Rota) RotasDaConsultaDPS() []Rota {
	if temDoisCaminhos(r.Municipio) {
		return []Rota{r.nacional(motivoConsultaDPS), r}
	}
	return []Rota{r}
}

// rota resolve o município ou responde o erro. Recusar aqui é melhor do que
// mandar um XML que a prefeitura rejeitaria com mensagem obscura.
func (m *Modulo) rota(w http.ResponseWriter, cmun string) (Rota, bool) {
	c := fiscal.SoDigitos(cmun)
	if c == "" {
		registrarDesfecho(w, "campo_obrigatorio", nil)
		httpx.ErroJSON(w, http.StatusBadRequest, "campo_obrigatorio",
			"município (código IBGE) é obrigatório: é ele que decide o provedor de NFS-e")
		return Rota{}, false
	}
	r, ok := RotaDoMunicipio(c)
	if !ok {
		registrarDesfecho(w, "provedor_nao_suportado", nil)
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "provedor_nao_suportado",
			"o município "+c+" não tem provedor de NFS-e conhecido nesta versão da tabela",
			map[string]any{"municipio": c})
		return Rota{}, false
	}
	registrarRota(w, r, "")
	return r, true
}

// SessaoNacional é o município que abre a sessão quando a operação tem de sair
// pelo Ambiente Nacional e o município do documento é atendido por outra classe.
//
// A lib escolhe a classe do provedor pelo CodigoMunicipio, e não há como pedir o
// Padrão Nacional de outro jeito. Por isso a sessão fala em nome deste município
// em dois casos: a emissão de quem a tabela de vigências manda para o nacional, e
// a Distribuição DF-e, que só existe em algumas classes (a do Padrão Nacional e
// as de API própria de Betha, Fiorilli, Infisc, ISSNet e outras). As demais
// levantam ERR_NAO_IMP antes de qualquer byte sair.
//
// Nem toda classe que implementa a distribuição fala com o ADN: nos municípios
// Infisc do RS (Caxias do Sul, Canoas e outros) a URL é o sefin da prefeitura, com
// NSU próprio, por isso ali o município do pedido continua tendo preferência e
// este só entra no lugar do ERR_NAO_IMP.
//
// Qualquer município atendido pelo PadraoNacional puro serve. O código é o do Rio
// de Janeiro, e um teste garante que a tabela continua a classificá-lo assim. A
// nota não sai em nome do Rio: o documento leva o cLocEmi real, e é dele que sai a
// chave (patch 0004 da lib).
const SessaoNacional = "3304557"

// ambGerSistemaNacional é o ambiente gerador das notas do Sistema Nacional, na
// posição 8 da chave de acesso. O 1 é o sistema da prefeitura.
const ambGerSistemaNacional = '2'

// dpsPeloAmbienteNacional é PeloAmbienteNacional a partir do pedido.
func dpsPeloAmbienteNacional(cmun string, p DPSPedido) bool {
	rt := p.InfDPS.Prest.RegTrib
	return rt != nil && PeloAmbienteNacional(cmun, rt.OpSimpNac, p.InfDPS.DCompet)
}

var (
	reOpSimpNac = regexp.MustCompile(abre + `opSimpNac>([0-9])<`)
	reDCompet   = regexp.MustCompile(abre + `dCompet>([0-9-]{10})<`)
	// reChaveNaResposta é a chave de uma NFS-e na resposta da consulta da DPS.
	reChaveNaResposta = regexp.MustCompile(`[0-9]{50}`)
)
