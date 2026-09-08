package nfse

import (
	"regexp"

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
// (internal/tabelas). Município sem provedor conhecido é recusado ANTES de
// transmitir: mandar XML inválido para a prefeitura só troca um erro claro por
// um obscuro.
func LayoutDoMunicipio(cmun string) (Layout, bool) {
	switch tabelas.FamiliaPorMunicipio(cmun).LayoutBase() {
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
