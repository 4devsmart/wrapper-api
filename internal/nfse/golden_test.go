package nfse

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// atualizarGolden regrava os arquivos em testdata em vez de comparar. Use só
// depois de entender a diferença, e revise o diff antes de commitar.
var atualizarGolden = flag.Bool("atualizar-golden", false, "regrava os goldens de testdata")

// conferirGolden compara obtido com testdata/nome, apontando a primeira linha
// diferente: num arquivo grande, o diff inteiro esconde o que importa.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	// Parte dos gravadores emite CRLF, e o Git guarda LF: sem isto o golden
	// passaria aqui e falharia em toda cópia nova do repositório.
	obtido = strings.ReplaceAll(obtido, "\r\n", "\n")
	caminho := filepath.Join("testdata", nome)
	if *atualizarGolden {
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("sem golden (rode com -atualizar-golden): %v", err)
	}
	guardado := strings.ReplaceAll(string(b), "\r\n", "\n")
	if guardado == obtido {
		return
	}
	quero, got := strings.Split(guardado, "\n"), strings.Split(obtido, "\n")
	for i := 0; i < len(quero) && i < len(got); i++ {
		if quero[i] != got[i] {
			t.Fatalf("%s difere na linha %d:\n quero: %s\n  veio: %s", nome, i+1, quero[i], got[i])
		}
	}
	t.Fatalf("%s difere no tamanho: %d linhas, quero %d", nome, len(got), len(quero))
}

// umPorCombinacao escolhe o menor código de cada (provedor, versão, API
// própria): a superfície real do NFS-e são essas combinações, não os 4.740
// municípios.
func umPorCombinacao() []tabelas.MunicipioNFSe {
	escolhido := map[string]tabelas.MunicipioNFSe{}
	for _, m := range tabelas.MunicipiosNFSe() {
		k := m.Provedor + "\t" + m.Versao
		if tabelas.APIPropriaNFSe(m.Codigo) {
			k += "\tapi"
		}
		if e, ok := escolhido[k]; !ok || m.Codigo < e.Codigo {
			escolhido[k] = m
		}
	}
	out := make([]tabelas.MunicipioNFSe, 0, len(escolhido))
	for _, m := range escolhido {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Codigo < out[j].Codigo })
	return out
}

// pedidoIntegracao é o pedido do caso de Brasília, com dados fictícios e o
// município trocado. Leva os campos dos dois leiautes, como o cliente manda.
func pedidoIntegracao(cmun, uf string) map[string]any {
	return map[string]any{
		"ambiente": "homologacao",
		"infDPS": map[string]any{
			"serie": "5", "nDPS": "4", "dCompet": "2026-09-01", "tpEmit": 1,
			"verAplic": "integracao", "cLocEmi": cmun,
			"prest": map[string]any{
				"CNPJ": "12345678000190", "xNome": "EMPRESA DE TESTE LTDA",
				"email": "teste@example.com", "telefone": "5130000000",
				"regTrib": map[string]any{"opSimpNac": 3, "regApTribSN": 1, "regEspTrib": 0},
				"cMun":    cmun, "UF": uf, "CEP": "70040010",
				"logradouro": "RUA DE TESTE", "numero": "1", "bairro": "CENTRO",
			},
			"toma": map[string]any{
				"CPF": "11144477735", "xNome": "Tomador de Teste", "cMun": "4309209",
				"UF": "RS", "CEP": "94170068", "logradouro": "Rua de Teste",
				"numero": "2", "bairro": "Centro",
			},
			"serv": map[string]any{
				"cMunPrestacao": cmun, "cServ": "010501", "cTribMun": "101", "xDescServ": "teste de layout",
				"codigoCnae": "6201501", "itemListaServico": "01.05",
				"cNBS": "1.1106.20.00", "municipioIncidencia": cmun,
			},
			"valores": map[string]any{
				"vServ": 1, "vDeducoes": 0, "vDescIncond": 0, "vDescCond": 0,
				"tribISSQN": 1, "iss_retido": 2,
				"tribMun": map[string]any{"tribISSQN": 1, "tpRetISSQN": 1},
				"totTrib": map[string]any{"pTotTribSN": 9},
			},
		},
	}
}
