//go:build integracao

package nfse

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// TestRaizDoXMLPorProvedor gera a DPS contra a lib real, um município por
// combinação de provedor, versão e API própria, e cobra que a raiz do XML seja
// do layout que a tabela informa para o município.
//
// Os testes de unidade só enxergam o lado de cá: a tabela e o construtor de
// INI. Quem escolhe o gravador é a lib, e foi a discordância entre os dois que
// fez Brasília receber DPS com valores de ABRASF. Isto precisa de uma API com
// worker de pé:
//
//	WRAPPER_API_URL=http://127.0.0.1:8080 WRAPPER_API_TOKEN=... make test-integracao-nfse
//
// Erro que não seja de layout (provedor que recusa o pedido mínimo, lib que
// não monta) é registrado e não reprova: o que se cobra aqui é o leiaute.
func TestRaizDoXMLPorProvedor(t *testing.T) {
	base := strings.TrimRight(os.Getenv("WRAPPER_API_URL"), "/")
	if base == "" {
		t.Skip("WRAPPER_API_URL não definido")
	}
	token := os.Getenv("WRAPPER_API_TOKEN")
	cli := &http.Client{Timeout: 90 * time.Second}

	for _, m := range umPorCombinacao() {
		nome := m.Provedor + "/" + cmpVazio(m.Versao) + "/" + m.Codigo
		if tabelas.APIPropriaNFSe(m.Codigo) {
			nome += "/api-propria"
		}
		t.Run(nome, func(t *testing.T) {
			layout, ok := LayoutDoMunicipio(m.Codigo)
			if !ok {
				t.Fatalf("município sem layout na tabela")
			}
			corpo, _ := json.Marshal(pedidoIntegracao(m.Codigo, m.UF))
			req, _ := http.NewRequest(http.MethodPost, base+"/v1/nfse/xml", bytes.NewReader(corpo))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := cli.Do(req)
			if err != nil {
				t.Fatalf("chamada: %v", err)
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)

			var r struct {
				XMLBase64 string `json:"xml_b64"`
				Layout    string `json:"layout"`
				Erro      *struct {
					Codigo   string `json:"codigo"`
					Mensagem string `json:"mensagem"`
				} `json:"erro"`
			}
			if err := json.Unmarshal(b, &r); err != nil {
				t.Fatalf("resposta não é JSON (%d): %s", resp.StatusCode, b)
			}
			if r.Erro != nil {
				if r.Erro.Codigo == "layout_divergente" {
					t.Fatalf("layout_divergente: %s", b)
				}
				t.Logf("não montou (%d %s): %s", resp.StatusCode, r.Erro.Codigo, r.Erro.Mensagem)
				return
			}
			x, _ := base64.StdEncoding.DecodeString(r.XMLBase64)
			raiz, _ := RaizDoXML(string(x))
			t.Logf("layout %s, raiz %s em %q", layout, raiz.Nome, raiz.Namespace)
			if r.Layout != string(layout) || !LayoutConfere(layout, raiz) {
				t.Errorf("layout %s (resposta: %s), raiz %s em %q", layout, r.Layout, raiz.Nome, raiz.Namespace)
			}
		})
	}
}

// umPorCombinacao escolhe o menor código de cada (provedor, versão, API
// própria): a superfície real do NFS-e são essas combinações, não os 4.740
// municípios.
func umPorCombinacao() []tabelas.MunicipioNFSe {
	todos, _ := tabelas.ListarMunicipiosNFSe(tabelas.FiltroMunicipioNFSe{Limit: 500})
	for off := 500; ; off += 500 {
		pag, total := tabelas.ListarMunicipiosNFSe(tabelas.FiltroMunicipioNFSe{Limit: 500, Offset: off})
		todos = append(todos, pag...)
		if off+500 >= total {
			break
		}
	}
	escolhido := map[string]tabelas.MunicipioNFSe{}
	for _, m := range todos {
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

func cmpVazio(v string) string {
	if v == "" {
		return "sem-versao"
	}
	return v
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
