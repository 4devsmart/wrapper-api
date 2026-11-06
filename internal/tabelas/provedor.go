package tabelas

import (
	_ "embed"
	"strings"
	"sync"
)

// municipiosProvedorTSV mapeia código IBGE → provedor de NFS-e, derivado do
// ACBrNFSeXServicos.ini do FONTE OFICIAL do ACBr (SVN trunk2), a MESMA tabela
// que é compilada dentro da `.so` e usada pelo ACBrNFSeX para selecionar o
// provedor pelo município. Só entram municípios COM provedor configurado;
// "PadraoNacional" é o Padrão Nacional ADN/SEFIN.
//
// Regenerar (em lockstep com a `.so`, ao bumpar a revisão): `make acbr-tabelas`,
// que roda scripts/gerar-tabelas-nfse.py sobre o fonte em acbr-source/. Ele
// recusa emitir a tabela se algum provedor não tiver família classificada.
//
//go:embed municipios_provedor.tsv
var municipiosProvedorTSV string

// MunicipioNFSe é um município cujo provedor de NFS-e está configurado na
// ACBrLib (nome/UF vêm da base canônica do IBGE).
type MunicipioNFSe struct {
	Codigo   string `json:"codigo"` // IBGE, 7 dígitos
	Nome     string `json:"nome"`
	UF       string `json:"uf"`
	Provedor string `json:"provedor"` // ex.: "PadraoNacional", "Betha", "Fiorilli"
	// Versao é o layout que ESTE município usa dentro do provedor: vazio quando
	// o provedor não versiona. Importa porque o mesmo provedor serve layouts
	// diferentes por cidade: o fintelISS atende Juiz de Fora em 2.00 e Itatiba em
	// 2.02, e só o 2.02 exige lista de itens. A superfície real do NFS-e são os
	// 122 pares (provedor, versão), não os 101 provedores.
	Versao string `json:"versao,omitempty"`
}

var (
	nfseOnce     sync.Once
	nfseByCod    map[string]string // código IBGE → provedor
	nfseVerByCod map[string]string // código IBGE -> versão do layout
	nfseAPIByCod map[string]bool   // código IBGE -> a lib usa a classe APIPropria do provedor
	nfseDFeByCod map[string]bool   // código IBGE -> URL própria de Distribuição DF-e
)

func carregarNFSe() {
	nfseOnce.Do(func() {
		nfseByCod = make(map[string]string)
		nfseVerByCod = make(map[string]string)
		nfseAPIByCod = make(map[string]bool)
		nfseDFeByCod = make(map[string]bool)
		for _, l := range strings.Split(municipiosProvedorTSV, "\n") {
			l = strings.TrimRight(l, "\r")
			if l == "" {
				continue
			}
			c := strings.Split(l, "\t")
			if len(c) < 2 || c[0] == "" || c[1] == "" {
				continue
			}
			nfseByCod[c[0]] = c[1]
			if len(c) > 2 {
				nfseVerByCod[c[0]] = c[2]
			}
			if len(c) > 3 && c[3] == "1" {
				nfseAPIByCod[c[0]] = true
			}
			if len(c) > 4 && c[4] == "1" {
				nfseDFeByCod[c[0]] = true
			}
		}
	})
}

// ProvedorNFSe devolve o provedor de NFS-e configurado para o código IBGE, ou
// "" se o município não tem provedor configurado na ACBrLib.
func ProvedorNFSe(codigo string) string {
	carregarNFSe()
	return nfseByCod[codigo]
}

// APIPropriaNFSe diz se, neste município, a lib instancia a classe APIPropria
// do provedor em vez da ABRASF ou da própria. O gravador dessas classes
// descende do Padrão Nacional e gera DPS: Brasília está no ISSNet, mas emite
// DPS 1.01 pelo webservice nacional do ISSNet. A marca sai do fonte do ACBr
// (scripts/gerar-tabelas-nfse.py), com a mesma regra que a lib aplica.
func APIPropriaNFSe(codigo string) bool {
	carregarNFSe()
	return nfseAPIByCod[codigo]
}

// DistribuicaoPropriaNFSe diz se o município sobrepõe, no
// ACBrNFSeXServicos.ini, a URL da Distribuição DF-e do provedor. Nele a fila é
// a do sefin da prefeitura (Caxias do Sul, Canoas e outros do Infisc), com NSU
// próprio, e não a do ADN.
func DistribuicaoPropriaNFSe(codigo string) bool {
	carregarNFSe()
	return nfseDFeByCod[codigo]
}

// MunicipiosNFSe devolve os municípios com provedor configurado, na ordem da
// tabela do IBGE.
func MunicipiosNFSe() []MunicipioNFSe {
	carregarNFSe()
	out := make([]MunicipioNFSe, 0, len(nfseByCod))
	for _, m := range Municipios() {
		if p := nfseByCod[m.Codigo]; p != "" {
			out = append(out, MunicipioNFSe{Codigo: m.Codigo, Nome: m.Nome, UF: m.UF,
				Provedor: p, Versao: nfseVerByCod[m.Codigo]})
		}
	}
	return out
}
