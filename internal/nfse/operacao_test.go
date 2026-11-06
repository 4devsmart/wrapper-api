package nfse

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// capturarLog troca o logger padrão e devolve o que saiu, como o serviço loga:
// em JSON.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &buf
}

// linhaDaOperacao devolve a única linha "nfse" do buffer.
func linhaDaOperacao(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var achadas []map[string]any
	for l := range strings.Lines(buf.String()) {
		var campos map[string]any
		if err := json.Unmarshal([]byte(l), &campos); err != nil {
			t.Fatalf("linha fora do JSON: %s", l)
		}
		if campos["msg"] == "nfse" {
			achadas = append(achadas, campos)
		}
	}
	if len(achadas) != 1 {
		t.Fatalf("%d linhas de operação, quero 1:\n%s", len(achadas), buf)
	}
	return achadas[0]
}

func conferirCampos(t *testing.T, got map[string]any, quero map[string]any) {
	t.Helper()
	for k, v := range quero {
		if got[k] != v {
			t.Errorf("%s = %v, quero %v", k, got[k], v)
		}
	}
}

func transmissaoDe(cmun, dCompet string, op int) map[string]any {
	xml := base64.StdEncoding.EncodeToString([]byte(dpsDe(cmun, dCompet, op)))
	return envelope(cmun, map[string]any{"xml_b64": xml})
}

func muxDoServico(lib acbr.NFSeServico) *http.ServeMux {
	mux := http.NewServeMux()
	NovoModulo(lib).Registrar(rotasEm{mux})
	return mux
}

func TestOperacao_RegistraRotaEMotivo(t *testing.T) {
	const novembro, outubro = "2026-11-01", "2026-10-31"
	consulta := func(chave string) map[string]any {
		return envelope(munAPIPropria, map[string]any{"chave": chave})
	}
	casos := []struct {
		nome    string
		caminho string
		corpo   map[string]any
		quero   map[string]any
	}{
		{
			"Simples na vigência vai ao nacional", "/nfse/transmissao",
			transmissaoDe(munAPIPropria, novembro, opSimpNacMEEPP),
			map[string]any{
				"operacao": "transmissao", "municipio": munAPIPropria, "sessao": SessaoNacional,
				"provedor": "PadraoNacional", "motivo": "vigencia", "ambiente": "homologacao",
			},
		},
		{
			"competência anterior fica no provedor", "/nfse/transmissao",
			transmissaoDe(munAPIPropria, outubro, opSimpNacMEEPP),
			map[string]any{
				"operacao": "transmissao", "municipio": munAPIPropria, "sessao": munAPIPropria,
				"provedor": "ISSNet", "motivo": "tabela",
			},
		},
		{
			"nota nacional segue a chave", "/nfse/consulta", consulta(chaveNacionalDeBrasilia),
			map[string]any{
				"operacao": "consulta", "municipio": munAPIPropria, "sessao": SessaoNacional,
				"motivo": "chave",
			},
		},
		{
			"nota do provedor fica no provedor", "/nfse/consulta", consulta(chaveDeBrasilia),
			map[string]any{"operacao": "consulta", "sessao": munAPIPropria, "motivo": "tabela"},
		},
		{
			"evento leva o tipo", "/nfse/eventos/cancelamento",
			envelope(munAPIPropria, map[string]any{
				"chave":  chaveNacionalDeBrasilia,
				"evento": map[string]any{"codigo": "1", "motivo": "erro na emissão da nota"},
			}),
			map[string]any{
				"operacao": "evento", "tipo": "cancelamento", "sessao": SessaoNacional,
				"motivo": "chave",
			},
		},
		{
			"distribuição sem município cai no nacional", "/nfse/distribuicao",
			envelope("", nil),
			map[string]any{"operacao": "distribuicao", "sessao": SessaoNacional, "motivo": "distribuicao"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			lib := &libQueRegistra{libFake: &libFake{}}
			buf := capturarLog(t)
			post(t, muxDoServico(lib), c.caminho, c.corpo)
			conferirCampos(t, linhaDaOperacao(t, buf), c.quero)
		})
	}
}

func TestOperacao_RegistraRejeicaoPeloCodigo(t *testing.T) {
	buf := capturarLog(t)
	f := &libFake{resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=0\n" +
		"[Erro1]\nCodigo=E0314\nDescricao=Tributação 601 inexistente\n"}}

	post(t, muxDe(f), "/nfse/transmissao", transmissaoDe(munAPIPropria, "2026-11-01", opSimpNacMEEPP))

	got := linhaDaOperacao(t, buf)
	conferirCampos(t, got, map[string]any{"desfecho": "rejeitado", "motivo": "vigencia"})
	if r, _ := got["rejeicoes"].([]any); !slices.Equal(r, []any{"E0314"}) {
		t.Errorf("rejeicoes = %v", got["rejeicoes"])
	}
}

// O caso que mais importa achar no log: a nota pode ter saído.
func TestOperacao_DesfechoIndeterminado(t *testing.T) {
	buf := capturarLog(t)
	f := &libFake{errTransmitir: acbr.ErrIndeterminado}

	post(t, muxDe(f), "/nfse/transmissao", transmissaoDe(munAPIPropria, "2026-11-01", opSimpNacMEEPP))

	conferirCampos(t, linhaDaOperacao(t, buf), map[string]any{
		"desfecho": "desfecho_indeterminado", "http": float64(http.StatusBadGateway),
	})
}

func TestOperacao_MunicipioSemProvedorNaoTemRota(t *testing.T) {
	buf := capturarLog(t)

	post(t, muxDe(&libFake{}), "/nfse/transmissao", transmissaoDe(munDesconhecido, "2026-11-01", 3))

	got := linhaDaOperacao(t, buf)
	conferirCampos(t, got, map[string]any{"desfecho": "provedor_nao_suportado"})
	if _, ok := got["sessao"]; ok {
		t.Errorf("sessão na linha de quem não teve rota: %v", got)
	}
}

// O corpo carrega certificado, e a resposta do provedor pode repetir o documento.
// Nada disso pode chegar ao log, nem disfarçado de código de rejeição.
func TestOperacao_NaoVazaDadoPessoal(t *testing.T) {
	const (
		cnpj = "12345678000190"
		cpf  = "12345678909"
	)
	buf := capturarLog(t)
	f := &libFake{
		resTransmitir: acbr.Result{Resposta: "[Envio]\nSucesso=0\n" +
			"[Erro1]\nCodigo=" + cnpj + "\nDescricao=prestador " + cnpj + " irregular\n" +
			"[Erro2]\nCodigo=" + cpf + "\nDescricao=tomador " + cpf + " irregular\n" +
			"[Erro3]\nCodigo=E0001\nDescricao=" + cpf + "\n"},
	}
	corpo := transmissaoDe(munAPIPropria, "2026-11-01", opSimpNacMEEPP)
	corpo["credenciais"] = map[string]any{"usuario": "u", "senha": segredoSenha, "token": segredoToken}

	post(t, muxDe(f), "/nfse/transmissao", corpo)
	post(t, muxDe(f), "/nfse/consulta", envelope(munAPIPropria, map[string]any{"chave": chaveDeBrasilia}))

	log := buf.String()
	vazou(t, "log da operação", log)
	for _, s := range []string{cnpj, cpf, chaveDeBrasilia, "irregular", "ACME", "9876"} {
		if strings.Contains(log, s) {
			t.Errorf("%q chegou ao log:\n%s", s, log)
		}
	}
	if !strings.Contains(log, "E0001") {
		t.Errorf("o código válido se perdeu:\n%s", log)
	}
}

func TestOperacao_LimitaRejeicoes(t *testing.T) {
	buf := capturarLog(t)
	var resp strings.Builder
	resp.WriteString("[Envio]\nSucesso=0\n")
	for range maxRejeicoesNoLog + 3 {
		resp.WriteString("[Erro1]\nCodigo=E1\n")
	}
	f := &libFake{resTransmitir: acbr.Result{Resposta: resp.String()}}

	post(t, muxDe(f), "/nfse/transmissao", transmissaoDe(munAPIPropria, "2026-11-01", opSimpNacMEEPP))

	if r, _ := linhaDaOperacao(t, buf)["rejeicoes"].([]any); len(r) != maxRejeicoesNoLog {
		t.Errorf("%d rejeições, quero %d", len(r), maxRejeicoesNoLog)
	}
}
