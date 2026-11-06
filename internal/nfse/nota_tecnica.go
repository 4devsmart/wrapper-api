package nfse

import (
	"strings"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
)

// NotaTecnica registra qual leiaute vale num ambiente de um provedor.
//
// A tabela diz quando, a lib diz como: é o ACBrNFSeXServicos.ini compilado na
// `.so` que liga a NT, por ambiente e sem data, e o gravador precisa existir na
// lib. Mudar uma linha aqui não muda o XML. Ela existe para que ligar uma NT seja
// decisão registrada: um bump que traga NT nova reprova
// TestNotasTecnicas_CobremOQueALibAplica até alguém escrever a linha, com a base.
type NotaTecnica struct {
	Provedor string
	// Ambiente é fiscal.AmbienteProducao ou fiscal.AmbienteHomologacao.
	Ambiente string
	// Versao é a do .ini, como em "009.1.01".
	Versao string
	// Desde é a data de obrigatoriedade, em AAAA-MM-DD. Vazia enquanto o Portal
	// Nacional não publicar: não se presume data.
	Desde string
	// Base é o que justifica a linha.
	Base string
}

// notasTecnicas fica no fonte pelo mesmo motivo de vigencias.
var notasTecnicas = []NotaTecnica{
	{
		Provedor: "PadraoNacional", Ambiente: fiscal.AmbienteHomologacao, Versao: "009.1.01",
		Base: "NT SE/CGNFS-e 009 v1.01; o ACBr a liga só em homologação desde a r48803",
	},
}

// NotaTecnicaDe é a NT em vigor na rota, vazia se não houver nenhuma além do
// leiaute de base.
func NotaTecnicaDe(provedor, ambiente string) string {
	for _, n := range notasTecnicas {
		if strings.EqualFold(n.Provedor, provedor) && n.Ambiente == ambiente {
			return n.Versao
		}
	}
	return ""
}
