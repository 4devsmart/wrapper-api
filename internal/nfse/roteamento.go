package nfse

import (
	"encoding/xml"
	"io"
	"regexp"
	"strings"

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
	reCLocEmi      = regexp.MustCompile(abre + `cLocEmi>([0-9]{7})<`)
	reCLocIncid    = regexp.MustCompile(abre + `cLocIncid>([0-9]{7})<`)
	reOrgaoGerador = regexp.MustCompile(`(?s)` + abre + `OrgaoGerador>.*?` + abre + `CodigoMunicipio>([0-9]{7})<`)
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
