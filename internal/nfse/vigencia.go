package nfse

import "github.com/4devsmart/wrapper-api/internal/fiscal"

// opSimpNacMEEPP é o opSimpNac de ME e EPP optantes do Simples Nacional.
const opSimpNacMEEPP = 3

// Vigencia diz que, a partir de uma competência, um regime de emitente deixa o
// provedor do município e passa a sair pelo Ambiente Nacional.
//
// A tabela de municípios da lib não tem como dizer isso: ela é por município, e
// aqui o caminho depende do emitente e da competência. Quem não está na linha
// continua no provedor.
type Vigencia struct {
	// Municipio é o código IBGE.
	Municipio string
	// OpSimpNac é o regime do emitente, como na DPS.
	OpSimpNac int
	// Desde é a primeira competência, em AAAA-MM-DD. A competência decide, e não
	// a data de emissão: nota de outubro emitida em novembro fica no provedor.
	Desde string
	// Base é o que justifica a linha: a norma e a recusa observada.
	Base string
}

// vigencias fica no fonte de propósito: mudar a rota de um município é mudança
// fiscal, e passa por PR, golden e release.
//
// Só entra município com a recusa comprovada. A regra pode ser nacional, mas tirar
// um município do provedor que hoje autoriza é trocar nota que sai por aposta.
var vigencias = []Vigencia{
	{
		Municipio: "5300108", OpSimpNac: opSimpNacMEEPP, Desde: "2026-11-01",
		Base: "Resolução CGSN 191/2026; ISSNet de Brasília recusa com EM110",
	},
}

// temDoisCaminhos diz se notas do município podem estar em dois sistemas.
func temDoisCaminhos(cmun string) bool {
	for _, v := range vigencias {
		if v.Municipio == cmun {
			return true
		}
	}
	return false
}

// PeloAmbienteNacional diz se a nota deste emitente sai pelo Ambiente Nacional em
// vez do provedor do município.
func PeloAmbienteNacional(cmun string, opSimpNac int, dCompet string) bool {
	if len(dCompet) < 10 {
		return false
	}
	cmun, dCompet = fiscal.SoDigitos(cmun), dCompet[:10]
	for _, v := range vigencias {
		if v.Municipio == cmun && v.OpSimpNac == opSimpNac && dCompet >= v.Desde {
			return true
		}
	}
	return false
}
