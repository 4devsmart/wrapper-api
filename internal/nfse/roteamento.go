package nfse

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// Layout é a família de layout de entrada resolvida para um município.
type Layout string

const (
	LayoutPadraoNacional Layout = "padrao_nacional"
	LayoutAbrasf         Layout = "abrasf"
	LayoutProprio        Layout = "proprio"
)

// LayoutDoMunicipio resolve o layout de entrada pelo código IBGE do município.
//
// NFS-e é o único documento multi-provedor: cada município escolhe o seu, e o
// layout de entrada muda. A cadeia é município → provedor → família → layout
// (internal/tabelas), salvo onde a lib usa a API própria do provedor, que gera
// DPS e por isso é Padrão Nacional na entrada. Município sem provedor conhecido
// é recusado ANTES de transmitir: mandar XML inválido para a prefeitura só
// troca um erro claro por um obscuro.
func LayoutDoMunicipio(cmun string) (Layout, bool) {
	switch tabelas.LayoutPorMunicipio(cmun) {
	case "padrao_nacional":
		return LayoutPadraoNacional, true
	case "abrasf":
		return LayoutAbrasf, true
	case "proprio":
		// Provedores próprios consomem o MESMO INI de RPS genérico: o LerIni do
		// NFSeX é único, e o que é proprietário é só o XML de saída, gerado pela
		// engine.
		return LayoutProprio, true
	default:
		return "", false
	}
}

// MunicipioADN é o município pelo qual a sessão nativa fala com o ADN
// (Ambiente de Dados Nacional) quando a distribuição não pode sair pelo
// município do cliente.
//
// A lib escolhe a classe do provedor pelo CodigoMunicipio, e a Distribuição
// DF-e só existe em algumas: a do Padrão Nacional e as de API própria de
// Betha, Fiorilli, Infisc, ISSNet e outras. As demais levantam ERR_NAO_IMP
// antes de qualquer byte sair, e quem emite num município ABRASF ou próprio
// ficava sem a fila do ADN. Nem toda classe que implementa fala com o ADN: nos
// municípios Infisc do RS (Caxias do Sul, Canoas e outros) a URL é o sefin da
// prefeitura, com NSU próprio, por isso o município do pedido continua tendo
// preferência e este só entra no lugar do ERR_NAO_IMP.
//
// Qualquer município atendido pelo PadraoNacional puro serve, porque o ADN é o
// mesmo para todos. O código é o do Rio de Janeiro, e um teste garante que a
// tabela continua a classificá-lo assim.
const MunicipioADN = "3304557"

// competenciaSimplesNacional é a primeira competência em que ME e EPP do Simples
// não emitem mais no sistema próprio do município (Resolução CGSN 191/2026).
const competenciaSimplesNacional = "2026-11-01"

// opSimpNacMEEPP é o opSimpNac de ME e EPP optantes do Simples Nacional.
const opSimpNacMEEPP = 3

// ambGerSistemaNacional é o ambiente gerador das notas do Sistema Nacional, na
// posição 8 da chave de acesso. O 1 é o sistema da prefeitura.
const ambGerSistemaNacional = '2'

// simplesSaiPeloAmbienteNacional são os municípios onde o emitente do Simples
// sai pelo Ambiente Nacional sem que a tabela da lib diga isso. A tabela é por
// município, e aqui o caminho depende do emitente: em Brasília quem não é do
// Simples continua no ISSNet, que recusa o Simples com EM110.
//
// Só entra aqui município com a recusa comprovada. A regra é nacional, mas tirar
// um município do provedor que hoje autoriza é trocar nota que sai por aposta.
var simplesSaiPeloAmbienteNacional = map[string]bool{"5300108": true}

// PeloAmbienteNacional diz se a nota deste emitente sai pelo Ambiente Nacional
// em vez do provedor do município. A competência decide, e não a data de
// emissão: nota de outubro emitida em novembro continua no provedor.
func PeloAmbienteNacional(cmun string, opSimpNac int, dCompet string) bool {
	return simplesSaiPeloAmbienteNacional[fiscal.SoDigitos(cmun)] &&
		opSimpNac == opSimpNacMEEPP && len(dCompet) >= 10 &&
		dCompet[:10] >= competenciaSimplesNacional
}

// dpsPeloAmbienteNacional é PeloAmbienteNacional a partir do pedido.
func dpsPeloAmbienteNacional(cmun string, p DPSPedido) bool {
	rt := p.InfDPS.Prest.RegTrib
	return rt != nil && PeloAmbienteNacional(cmun, rt.OpSimpNac, p.InfDPS.DCompet)
}

// MunicipioDaSessao é o município que abre a sessão da DPS: o da nota, salvo
// quando ela sai pelo Ambiente Nacional. A lib escolhe a classe pelo município
// da sessão, e o documento continua levando o município de verdade: cLocEmi e
// a chave da DPS vêm do INI (patch 0004 do ACBr).
func MunicipioDaSessao(cmun string, p DPSPedido) string {
	if dpsPeloAmbienteNacional(cmun, p) {
		return MunicipioADN
	}
	return cmun
}

// MunicipioDaSessaoDoXML é o mesmo para a transmissão, que só recebe a DPS
// pronta. O regime e a competência estão no próprio documento, então o cliente
// não precisa informar nada a mais, e uma DPS gerada antes desta regra também
// acerta o caminho.
func MunicipioDaSessaoDoXML(cmun, xml string) string {
	op, dCompet := 0, ""
	if m := reOpSimpNac.FindStringSubmatch(xml); len(m) == 2 {
		op, _ = strconv.Atoi(m[1])
	}
	if m := reDCompet.FindStringSubmatch(xml); len(m) == 2 {
		dCompet = m[1]
	}
	if PeloAmbienteNacional(cmun, op, dCompet) {
		return MunicipioADN
	}
	return cmun
}

// MunicipioDaNota é o município que abre a sessão para uma nota já emitida. Num
// município da lista convivem notas nos dois sistemas: as do Simples até
// outubro e as de quem não é do Simples no provedor, as novas no Sistema
// Nacional. É a chave que diz onde a nota está, sem nada a mais no pedido.
func MunicipioDaNota(cmun, chave string) string {
	chave = strings.TrimSpace(chave)
	if !simplesSaiPeloAmbienteNacional[fiscal.SoDigitos(cmun)] ||
		len(chave) != 50 || fiscal.SoDigitos(chave) != chave {
		return cmun
	}
	if chave[7] == ambGerSistemaNacional {
		return MunicipioADN
	}
	return cmun
}

// MunicipioDoEvento é o município da sessão de um evento. Na substituição a
// nota nova decide primeiro: se ela só pode sair pelo Ambiente Nacional, é por
// lá que a substituição tem de ir, qualquer que seja a antiga. Evento malformado
// fica na regra da chave, e a recusa sai na validação do próprio evento.
func MunicipioDoEvento(tipo string, p PedidoEvento) string {
	chave := p.Chave
	if tipo == "substituicao" {
		var e SubstituicaoPedido
		if json.Unmarshal(p.Evento, &e) == nil {
			if dpsPeloAmbienteNacional(p.Municipio, e.DPS) {
				return MunicipioADN
			}
			chave = fiscal.Primeiro(e.Substituida.Chave, chave)
		}
	}
	return MunicipioDaNota(p.Municipio, chave)
}

// ToINIDoLayout escolhe o construtor de INI conforme o layout. Padrão Nacional
// usa o construtor de DPS; ABRASF e próprios usam o mesmo construtor de RPS.
func ToINIDoLayout(l Layout, p DPSPedido) string {
	if l == LayoutPadraoNacional || l == "" {
		return ToINI(p)
	}
	return ToINIAbrasf(p)
}

// MunicipioDoPedido resolve o município emissor: cLocEmi quando informado,
// senão o do prestador. É ele que decide o provedor.
func MunicipioDoPedido(p DPSPedido) string {
	if c := p.InfDPS.CLocEmi; c != "" {
		return c
	}
	return p.InfDPS.Prest.CMun
}

// provedorDoMunicipio devolve o nome do provedor de NFS-e do município (vazio se
// desconhecido). Serve para a resposta dizer QUEM atende, não só se é atendido:
// é a informação que o cliente leva ao suporte da prefeitura.
func provedorDoMunicipio(cmun string) string { return tabelas.ProvedorNFSe(cmun) }

// MunicipioDoXML descobre o município emissor a partir do XML de uma NFS-e já
// autorizada. É o par de MunicipioDoPedido para quem só tem o documento.
//
// Serve ao DANFSE por XML: quem LÊ o XML é a classe do provedor, e o provedor
// só é selecionado pelo CodigoMunicipio (ACBrNFSeXConfiguracoes.pas:686,
// SetCodigoMunicipio → LerParamsMunicipio → SetProvider). Sem ele a lib recusa o
// documento sem sequer olhá-lo: "Nenhum provedor selecionado" (ERR_SEM_PROVEDOR
// em ACBrNFSeX.pas:48). Exigir o município do cliente resolveria, mas ele já
// está dentro do XML: pedir de novo é atrito à toa.
//
// A ordem segue quem responde "que prefeitura emitiu": cLocEmi no Padrão
// Nacional; o CodigoMunicipio do OrgaoGerador no ABRASF (o do prestador e o da
// prestação também aparecem no documento, e nem sempre são o mesmo município);
// e, por último, cLocIncid, que é o município da incidência do ISS.
func MunicipioDoXML(xml string) string {
	for _, re := range []*regexp.Regexp{reCLocEmi, reOrgaoGerador, reCLocIncid} {
		if m := re.FindStringSubmatch(xml); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

// Tags com prefixo de namespace (<ns2:CodigoMunicipio>) aparecem em provedores
// ABRASF, então o prefixo é opcional em todos os padrões.
const abre = `<(?:[A-Za-z0-9_.-]+:)?` // abertura de tag, com prefixo opcional

var (
	reCLocEmi         = regexp.MustCompile(abre + `cLocEmi>([0-9]{7})<`)
	reCLocIncid       = regexp.MustCompile(abre + `cLocIncid>([0-9]{7})<`)
	reOpSimpNac       = regexp.MustCompile(abre + `opSimpNac>([0-9])<`)
	reDCompet         = regexp.MustCompile(abre + `dCompet>([0-9-]{10})<`)
	reChaveNaResposta = regexp.MustCompile(`[0-9]{50}`)
	reOrgaoGerador    = regexp.MustCompile(`(?s)` + abre + `OrgaoGerador>.*?` + abre + `CodigoMunicipio>([0-9]{7})<`)
)

// nsNFSeNacional é o namespace da DPS e da NFS-e do Padrão Nacional.
const nsNFSeNacional = "http://www.sped.fazenda.gov.br/nfse"

// RaizXML é o primeiro elemento de um XML: o que identifica o leiaute.
type RaizXML struct {
	Nome      string `json:"nome"`
	Namespace string `json:"namespace"`
}

// RaizDoXML lê só o primeiro elemento. Falso se não houver elemento, o que é
// legítimo fora do Padrão Nacional: há provedor próprio que monta JSON ou TXT.
func RaizDoXML(doc string) (RaizXML, bool) {
	d := xml.NewDecoder(strings.NewReader(doc))
	// Só o nome do primeiro elemento importa, e ele é ASCII: a declaração
	// ISO-8859-1 de alguns provedores não precisa de conversão.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := d.Token()
		if err != nil {
			return RaizXML{}, false
		}
		if se, ok := tok.(xml.StartElement); ok {
			return RaizXML{Nome: se.Name.Local, Namespace: se.Name.Space}, true
		}
	}
}

// PadraoNacional diz se a raiz é de um gravador do Padrão Nacional: DPS, ou
// qualquer raiz no namespace nacional (Citta, Digifred, SilTecnologia e
// DBSeller pela API própria geram NFSe). O namespace sozinho não basta: o
// Fiorilli pela API própria grava DPS em http://www.fiorilli.com.br/nfse-nacional.
func (r RaizXML) PadraoNacional() bool {
	return r.Nome == "DPS" || r.Namespace == nsNFSeNacional
}

// LayoutConfere diz se a raiz do XML gerado é do layout resolvido para o
// município.
//
// É a guarda contra a classe de defeito em que a tabela de roteamento e a lib
// discordam. A lib escolhe o gravador por município; se o layout daqui disser
// outra coisa, o XML sai com o envelope de um leiaute e os valores de outro, e
// nenhum dos dois o aceita. Brasília saiu assim: DPS com cTribNac "01.05".
func LayoutConfere(l Layout, r RaizXML) bool {
	return (l == LayoutPadraoNacional) == r.PadraoNacional()
}

// reCTribMunISSNet é o que o gravador do ISSNet aceita sem trocar por 0: só
// dígitos, não zero, e no máximo 9 significativos, porque o StrToIntDef
// converte para Integer de 32 bits e um valor acima de 2147483647 também vira 0.
var reCTribMunISSNet = regexp.MustCompile(`^0*[1-9][0-9]{0,8}$`)

// reCTribMunNacional é o cTribMun do gravador nacional: opcional, e quando vem
// tem exatamente 3 posições (PadraoNacional.GravarXml.pas, GerarXMLCodigoServico).
var reCTribMunNacional = regexp.MustCompile(`^[0-9]{3}$`)

// ValidarDPSDoMunicipio confere o que o gravador do município exige e a lib não
// acusa, e devolve a frase do erro (vazia se está tudo certo).
//
// O caso é o ISSNet pela API própria (Brasília, Goiânia e outros): o gravador
// emite cTribMun obrigatório como inteiro, StrToIntDef(..., 0)
// (ISSNet.GravarXml.pas, GerarXMLCodigoServico). Sem o campo, ou com texto, a
// DPS sai com <cTribMun>0</cTribMun>, sem alerta da lib, e Brasília recusou
// com EM020, E244 e EM076. No Padrão Nacional o campo é opcional, e nos ABRASF
// que o exigem (ISSGoiania, Centi) a lib já sinaliza a tag vazia.
//
// Vale também o tomador, em todo município de layout padrao_nacional: ver
// validarTomadorPN.
//
// A nota que sai pelo Ambiente Nacional segue as regras dele, mesmo num município
// do ISSNet: o cTribMun é opcional, mas com o tamanho que o gravador exige.
func ValidarDPSDoMunicipio(cmun string, p DPSPedido) string {
	nacional := dpsPeloAmbienteNacional(cmun, p)
	if c := strings.TrimSpace(p.InfDPS.Serv.CTribMun); nacional && c != "" && !reCTribMunNacional.MatchString(c) {
		return "serv.cTribMun tem exatamente 3 dígitos na emissão pelo Ambiente Nacional"
	}
	issnet := !nacional && strings.EqualFold(tabelas.ProvedorNFSe(cmun), "ISSNet") && tabelas.APIPropriaNFSe(cmun)
	if issnet && !reCTribMunISSNet.MatchString(strings.TrimSpace(p.InfDPS.Serv.CTribMun)) {
		return "serv.cTribMun é obrigatório neste município (ISSNet): o código de tributação municipal, " +
			"numérico, como cadastrado para o prestador na prefeitura"
	}
	if l, _ := LayoutDoMunicipio(cmun); l == LayoutPadraoNacional {
		return validarTomadorPN(p.InfDPS.Toma, issnet)
	}
	return ""
}

// validarTomadorPN recusa o toma que o gravador do Padrão Nacional (e os da
// API própria, que descendem dele) transformaria em outra coisa sem avisar
// (PadraoNacional.GravarXml.pas, GerarXMLTomador):
//
//   - sem xNome o grupo <toma> inteiro some, documento e endereço junto;
//   - sem CPF, CNPJ nem NIF sai <cNaoNIF>, e sem a chave o LerIni presume 0
//     ("não informado na nota de origem"). O motivo é de tomador no exterior,
//     não de consumidor sem CPF, e o XSD do ISSNet (Brasília) nem tem o 0:
//     aceita só 1 e 2.
//
// O consumidor não identificado é a DPS sem toma, que os dois XSDs aceitam
// (minOccurs=0) e o DANFSE imprime como "TOMADOR/ADQUIRENTE DA OPERAÇÃO NÃO
// IDENTIFICADO NA NFS-e". No ABRASF o tomador sem documento é legítimo, e esta
// regra não vale lá.
func validarTomadorPN(toma *Tomador, issnet bool) string {
	if toma == nil {
		return ""
	}
	const semTomador = "; para consumidor não identificado, omita toma"
	if strings.TrimSpace(toma.XNome) == "" {
		return "toma.xNome é obrigatório quando toma é informado" + semTomador
	}
	if strings.TrimSpace(toma.CPF+toma.CNPJ+toma.NIF) != "" {
		return ""
	}
	switch strings.TrimSpace(toma.CNaoNIF) {
	case "":
		return "toma sem CPF, CNPJ ou NIF: informe o documento" + semTomador +
			" (cNaoNIF é o motivo de tomador no exterior sem NIF)"
	case "0":
		if issnet {
			return "toma.cNaoNIF=0 não é aceito neste município (ISSNet): use 1 (dispensado) ou 2 (não exigido)" +
				semTomador
		}
	}
	return ""
}
