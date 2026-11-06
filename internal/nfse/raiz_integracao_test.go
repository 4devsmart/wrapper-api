//go:build integracao

package nfse

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
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

func cmpVazio(v string) string {
	if v == "" {
		return "sem-versao"
	}
	return v
}

// naoDeterminista são os provedores cujo gravador muda o XML entre duas execuções
// do mesmo pedido. O ISSMap cifra os valores, e o resultado varia.
var naoDeterminista = map[string]bool{"ISSMap": true}

// TestGoldenXML compara o XML gerado pela lib real com o guardado em
// testdata/golden_xml, um arquivo por combinação de provedor, versão e API
// própria, mais a nota de Brasília que sai pelo Ambiente Nacional.
//
// Diferença aqui não é necessariamente erro: num bump do ACBr é justamente o que
// se quer ler. Se a mudança é esperada, regere com `make golden-nfse` e revise o
// diff. dhEmi vai só com a data para o golden não depender do relógio.
func TestGoldenXML(t *testing.T) {
	base := strings.TrimRight(os.Getenv("WRAPPER_API_URL"), "/")
	if base == "" {
		t.Skip("WRAPPER_API_URL não definido")
	}
	token := os.Getenv("WRAPPER_API_TOKEN")
	cli := &http.Client{Timeout: 90 * time.Second}

	gerar := func(t *testing.T, p map[string]any) string {
		t.Helper()
		corpo, _ := json.Marshal(p)
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
			Erro      *struct {
				Codigo string `json:"codigo"`
			} `json:"erro"`
		}
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatalf("resposta não é JSON (%d): %s", resp.StatusCode, b)
		}
		if r.Erro != nil {
			return "ERRO " + r.Erro.Codigo + "\n"
		}
		x, _ := base64.StdEncoding.DecodeString(r.XMLBase64)
		return strings.ReplaceAll(string(x), "><", ">\n<") + "\n"
	}

	pedido := func(cmun, uf, dCompet string) map[string]any {
		p := pedidoIntegracao(cmun, uf)
		inf := p["infDPS"].(map[string]any)
		inf["dCompet"], inf["dhEmi"] = dCompet, dCompet
		return p
	}

	for _, m := range umPorCombinacao() {
		if naoDeterminista[m.Provedor] {
			continue
		}
		t.Run(m.Provedor+"/"+cmpVazio(m.Versao)+"/"+m.Codigo, func(t *testing.T) {
			conferirGolden(t, "golden_xml/"+m.Codigo+".xml", gerar(t, pedido(m.Codigo, m.UF, "2026-09-01")))
		})
	}
	t.Run("Brasília pelo Ambiente Nacional", func(t *testing.T) {
		conferirGolden(t, "golden_xml/5300108-nacional.xml", gerar(t, pedido("5300108", "DF", "2026-11-05")))
	})
}
