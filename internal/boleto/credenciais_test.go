package boleto

import (
	"net/http"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// A conta de boleto carrega as credenciais de webservice do banco: ClientSecret
// e o par de mTLS, com a CHAVE PRIVADA. Elas entram no pedido, viram arquivo
// intermediário e vão ao worker; o que não podem é voltar para o cliente.
//
// Existia uma função Pedido.SemCredenciais para isso, herdada do projeto de
// origem, onde ela sanitizava o registro de auditoria antes de GRAVAR. Aqui não
// há persistência, ninguém a chamava, e o comentário dela prescrevia um uso
// ("em qualquer ponto que devolva o Pedido") que rota nenhuma fazia. Função
// morta não protege nada: o que protege é conferir a resposta de verdade.
//
// Este teste percorre TODAS as rotas do módulo, nos dois desfechos, e cobra que
// nenhum byte de credencial saia no corpo.

const (
	segredoOAuth = "SEGREDO-OAUTH-DO-BANCO"
	segredoIDCli = "client-id-do-banco"
	// O par de mTLS chega em base64, e o handler recusa o que não for: as
	// sentinelas precisam ser base64 de verdade, senão a rota de registro para
	// num 400 e o teste passaria sem exercitar o caminho que interessa.
	segredoMTLS = "Y2hhdmUtcHJpdmFkYS1tdGxzLWRvLWJhbmNv" // base64 de "chave-privada-mtls-do-banco"
	segredoCert = "Y2VydGlmaWNhZG8tbXRscy1kby1iYW5jbw==" // base64 de "certificado-mtls-do-banco"
)

func contaComCredenciais() map[string]any {
	c := contaMinima()
	c["ws"] = map[string]any{
		"clientID": segredoIDCli, "clientSecret": segredoOAuth,
		"certCRT": segredoCert, "certKEY": segredoMTLS,
	}
	return c
}

// rotasDoModulo descobre as rotas pelo próprio Registrar: rota nova entra na
// conferência sem ninguém precisar lembrar de acrescentá-la aqui.
func rotasDoModulo(t *testing.T) []string {
	t.Helper()
	var achadas []string
	NovoModulo(&libFake{}).Registrar(coletor{&achadas})
	if len(achadas) < 4 {
		t.Fatalf("só %d rotas encontradas; a coleta quebrou", len(achadas))
	}
	return achadas
}

type coletor struct{ em *[]string }

func (c coletor) Handle(p string, _ http.Handler)         { c.registrar(p) }
func (c coletor) HandleFunc(p string, _ http.HandlerFunc) { c.registrar(p) }
func (c coletor) registrar(padrao string) {
	_, caminho, _ := strings.Cut(padrao, " ")
	*c.em = append(*c.em, "/boletos"+caminho)
}

// corpoDaRota monta um payload plausível para cada rota, sempre com as
// credenciais dentro.
func corpoDaRota(rota string) any {
	conta := contaComCredenciais()
	switch {
	case strings.HasSuffix(rota, "/retorno"):
		return map[string]any{"conta": conta, "arquivo": "0237CONTEUDO-CNAB"}
	case strings.HasSuffix(rota, "/remessa"):
		return map[string]any{"conta": conta, "titulos": []any{tituloMinimo()}, "numeroArquivo": 7}
	case strings.HasSuffix(rota, "/registro"):
		return map[string]any{"conta": conta, "titulos": []any{tituloMinimo()}, "operacao": 0}
	default:
		return map[string]any{"conta": conta, "titulos": []any{tituloMinimo()}}
	}
}

func TestNenhumaRotaDevolveCredencialDoBanco(t *testing.T) {
	desfechos := map[string]*libFake{
		"sucesso": {
			resPDF:     acbr.Result{PDF: []byte("%PDF-1.4 boleto")},
			resRemessa: acbr.Result{Resposta: "REMESSA-OK"},
			resRetorno: acbr.Result{Resposta: "RETORNO-OK"},
			resOnline:  acbr.Result{Resposta: "registrado"},
		},
		// O caminho de erro é o que mais tenta ser generoso com o detalhe, e é
		// onde um payload ecoado apareceria.
		"falha da lib":  {err: acbr.ErrUnavailable},
		"banco recusou": {resOnline: acbr.Result{Codigo: -7, Resposta: "titulo ja registrado"}},
	}

	for _, rota := range rotasDoModulo(t) {
		for nome, f := range desfechos {
			t.Run(rota+" ("+nome+")", func(t *testing.T) {
				rec := post(t, muxDe(f), rota, corpoDaRota(rota))
				corpo := rec.Body.String()
				for _, segredo := range []string{segredoOAuth, segredoMTLS, segredoCert, segredoIDCli} {
					if strings.Contains(corpo, segredo) {
						t.Errorf("a resposta (HTTP %d) devolveu credencial do banco ao cliente:\n%s", rec.Code, corpo)
					}
				}
			})
		}
	}
}

// A contraprova: as credenciais PRECISAM chegar ao worker, senão nada é
// registrado. O que se proíbe é o caminho de volta, não o de ida.
func TestCredenciaisChegamAoWorker(t *testing.T) {
	f := &libFake{resOnline: acbr.Result{Resposta: "registrado"}}
	rec := post(t, muxDe(f), "/boletos/registro", corpoDaRota("/boletos/registro"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(f.online.INI, segredoOAuth) {
		t.Errorf("o ClientSecret não chegou ao worker: sem ele o banco recusa\n%s", f.online.INI)
	}
	if string(f.online.CertKEY) != "chave-privada-mtls-do-banco" {
		t.Errorf("a chave do mTLS não chegou ao worker decodificada: %q", f.online.CertKEY)
	}
}
