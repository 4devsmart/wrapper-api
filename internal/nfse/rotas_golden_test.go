package nfse

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/tabelas"
)

// libQueRegistra anota cada chamada à lib: método, município da sessão e se as
// credenciais de prefeitura foram junto. É isso que define por onde uma nota sai,
// e nenhum teste de contrato enxerga: o JSON é o mesmo em qualquer caminho.
type libQueRegistra struct {
	*libFake
	chamadas []string
}

func (l *libQueRegistra) anota(metodo string, t acbr.Sessao) {
	cred := ""
	if cfgDaSessao(t, "Emitente.WSUser") != "" {
		cred = "+cred"
	}
	l.chamadas = append(l.chamadas, metodo+"@"+cfgDaSessao(t, "CodigoMunicipio")+cred)
}

func (l *libQueRegistra) MontarXML(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("MontarXML", t)
	if lay, _ := LayoutDoMunicipio(cfgDaSessao(t, "CodigoMunicipio")); lay != LayoutPadraoNacional {
		return acbr.Result{XML: xmlRpsFixture}, nil
	}
	return acbr.Result{XML: xmlFixture("2")}, nil
}

func (l *libQueRegistra) Transmitir(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("Transmitir", t)
	return acbr.Result{Resposta: "[Envio]\nSucesso=1\n", XML: "<NFSe/>"}, nil
}

func (l *libQueRegistra) Emitir(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("Emitir", t)
	return acbr.Result{Resposta: "[Envio]\nSucesso=1\n", XML: "<NFSe/>"}, nil
}

func (l *libQueRegistra) SubstituirNFSe(t acbr.Sessao, _ string, _ acbr.SubstituicaoNFSe) (acbr.Result, error) {
	l.anota("SubstituirNFSe", t)
	return acbr.Result{Resposta: "[Envio]\nSucesso=1\n", XML: "<NFSe/>"}, nil
}

func (l *libQueRegistra) EnviarEvento(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("EnviarEvento", t)
	return acbr.Result{Resposta: "[Evento]\nSucesso=1\n"}, nil
}

func (l *libQueRegistra) Cancelar(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("Cancelar", t)
	return acbr.Result{Resposta: "[Cancelamento]\nSucesso=1\n"}, nil
}

func (l *libQueRegistra) Consultar(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("Consultar", t)
	return acbr.Result{Resposta: "ok"}, nil
}

// A DPS não é achada: o caminho que importa é o que percorre os dois sistemas.
func (l *libQueRegistra) ConsultarDPSPorChave(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("ConsultarDPSPorChave", t)
	return acbr.Result{Resposta: "ok"}, nil
}

func (l *libQueRegistra) ConsultarPorNumero(t acbr.Sessao, _ string, _ int) (acbr.Result, error) {
	l.anota("ConsultarPorNumero", t)
	return acbr.Result{Resposta: "ok"}, nil
}

func (l *libQueRegistra) ConsultarDFe(t acbr.Sessao, _ string, _ int) (acbr.Result, error) {
	l.anota("ConsultarDFe", t)
	return acbr.Result{Resposta: "ok"}, nil
}

func (l *libQueRegistra) ObterPDF(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("ObterPDF", t)
	return acbr.Result{PDF: []byte("%PDF")}, nil
}

func (l *libQueRegistra) RenderizarPDF(t acbr.Sessao, _ string) (acbr.Result, error) {
	l.anota("RenderizarPDF", t)
	return acbr.Result{PDF: []byte("%PDF")}, nil
}

// chaveDe monta uma chave de 50 dígitos no município e no ambiente gerador dados.
func chaveDe(cmun, ambGer string) string {
	return cmun + ambGer + "2" + "12345678000190" + "0000000000123" + "2610" + "179121173" + "0"
}

func dpsDe(cmun, dCompet string, opSimpNac int) string {
	return `<DPS xmlns="http://www.sped.fazenda.gov.br/nfse"><infDPS Id="DPS` + cmun + `"><tpAmb>2</tpAmb>` +
		`<dCompet>` + dCompet + `</dCompet><cLocEmi>` + cmun + `</cLocEmi><prest><regTrib><opSimpNac>` +
		fmt.Sprint(opSimpNac) + `</opSimpNac></regTrib></prest></infDPS></DPS>`
}

var reProvedorDaResposta = regexp.MustCompile(`"provedor":"([^"]*)"`)

// TestGoldenRotas congela por onde cada operação sai, em cada combinação de
// provedor, versão e API própria. Roda sem a lib, e por isso vale em todo PR.
//
// É a rede da quebra invisível: mandar uma nota pelo caminho errado não muda
// nenhum JSON. Diferença aqui é mudança de rota: só regere o arquivo se ela foi
// a intenção, com `go test ./internal/nfse -run TestGoldenRotas -atualizar-golden`.
func TestGoldenRotas(t *testing.T) {
	const out, nov = "2026-10-31", "2026-11-01"
	var linhas []string

	municipios := umPorCombinacao()
	if !temMunicipio(municipios, "5300108") {
		municipios = append(municipios, tabelas.MunicipioNFSe{Codigo: "5300108", UF: "DF", Provedor: "ISSNet"})
	}
	for _, m := range municipios {
		c := m.Codigo
		cred := map[string]any{"usuario": "u", "senha": "s", "token": "t"}
		envelopeDe := func(extra map[string]any) map[string]any {
			e := envelope(c, map[string]any{"credenciais": cred})
			for k, v := range extra {
				e[k] = v
			}
			return e
		}
		pedido := func(dCompet string, op int) map[string]any {
			p := pedidoIntegracao(c, m.UF)
			inf := p["infDPS"].(map[string]any)
			inf["dCompet"] = dCompet
			inf["prest"].(map[string]any)["regTrib"].(map[string]any)["opSimpNac"] = op
			return p
		}
		transmissao := func(dCompet string, op int) map[string]any {
			return envelopeDe(map[string]any{"xml_b64": base64.StdEncoding.EncodeToString([]byte(dpsDe(c, dCompet, op)))})
		}
		cancelamento := func(ambGer string) map[string]any {
			return envelopeDe(map[string]any{"chave": chaveDe(c, ambGer), "evento": map[string]any{
				"codigo": "1", "motivo": "erro na emissão da nota", "numero": "123"}})
		}
		substituicao := func(dCompet, ambGer string) map[string]any {
			return envelopeDe(map[string]any{"evento": map[string]any{
				"dps": pedido(dCompet, 3), "codigo": "99", "motivo": "correção dos dados da nota",
				"substituida": map[string]any{"chave": chaveDe(c, ambGer), "numero": "123"}}})
		}
		pdfDoXML := func(ambGer string) map[string]any {
			x := `<NFSe><infNFSe Id="NFS` + chaveDe(c, ambGer) + `"><cLocEmi>` + c + `</cLocEmi></infNFSe></NFSe>`
			return map[string]any{"xml_b64": base64.StdEncoding.EncodeToString([]byte(x))}
		}

		for _, cen := range []struct {
			nome, caminho string
			corpo         map[string]any
		}{
			{"xml simples out", "/nfse/xml", pedido(out, 3)},
			{"xml simples nov", "/nfse/xml", pedido(nov, 3)},
			{"xml nao optante nov", "/nfse/xml", pedido(nov, 1)},
			{"xml mei nov", "/nfse/xml", pedido(nov, 2)},
			{"transmissao simples out", "/nfse/transmissao", transmissao(out, 3)},
			{"transmissao simples nov", "/nfse/transmissao", transmissao(nov, 3)},
			{"transmissao nao optante nov", "/nfse/transmissao", transmissao(nov, 1)},
			{"cancelamento prefeitura", "/nfse/eventos/cancelamento", cancelamento("1")},
			{"cancelamento nacional", "/nfse/eventos/cancelamento", cancelamento("2")},
			{"substituicao nova out antiga prefeitura", "/nfse/eventos/substituicao", substituicao(out, "1")},
			{"substituicao nova out antiga nacional", "/nfse/eventos/substituicao", substituicao(out, "2")},
			{"substituicao nova nov antiga prefeitura", "/nfse/eventos/substituicao", substituicao(nov, "1")},
			{"consulta prefeitura", "/nfse/consulta", envelopeDe(map[string]any{"chave": chaveDe(c, "1")})},
			{"consulta nacional", "/nfse/consulta", envelopeDe(map[string]any{"chave": chaveDe(c, "2")})},
			{"consulta dps", "/nfse/consulta-dps", envelopeDe(map[string]any{"chave": "DPS" + c + "24003012200014500003000000000017653"})},
			{"consulta numero", "/nfse/consultas/numero", envelopeDe(map[string]any{"numero": "123"})},
			{"distribuicao", "/nfse/distribuicao", envelopeDe(nil)},
			{"pdf chave prefeitura", "/nfse/pdf", envelopeDe(map[string]any{"chave": chaveDe(c, "1")})},
			{"pdf chave nacional", "/nfse/pdf", envelopeDe(map[string]any{"chave": chaveDe(c, "2")})},
			{"pdf xml prefeitura", "/nfse/pdf", pdfDoXML("1")},
			{"pdf xml nacional", "/nfse/pdf", pdfDoXML("2")},
		} {
			lib := &libQueRegistra{libFake: &libFake{}}
			mux := http.NewServeMux()
			NovoModulo(lib).Registrar(rotasEm{mux})
			b, _ := json.Marshal(cen.corpo)
			rec := post(t, mux, cen.caminho, json.RawMessage(b))
			provedor := ""
			if s := reProvedorDaResposta.FindStringSubmatch(rec.Body.String()); len(s) == 2 {
				provedor = s[1]
			}
			linhas = append(linhas, strings.Join([]string{c, m.Provedor, cen.nome, fmt.Sprint(rec.Code),
				provedor, strings.Join(lib.chamadas, " ")}, "\t"))
		}
	}
	conferirGolden(t, "rotas.golden.tsv", strings.Join(linhas, "\n")+"\n")
}

func temMunicipio(ms []tabelas.MunicipioNFSe, codigo string) bool {
	for _, m := range ms {
		if m.Codigo == codigo {
			return true
		}
	}
	return false
}
