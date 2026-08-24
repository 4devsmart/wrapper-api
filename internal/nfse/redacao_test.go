package nfse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// Credenciais tinha String() e NÃO tinha LogValue(), e parecia protegida.
//
// Não estava: o serviço loga em JSON, e o manipulador JSON do slog não consulta
// Stringer, ele serializa a struct campo a campo. A senha e o token do
// webservice do município saíam inteiros no log. Este teste é o que impede a
// meia proteção de voltar.
const (
	segredoSenha = "s3nh4-do-webservice"
	segredoToken = "token-que-nao-pode-vazar"
)

func credenciaisDeTeste() Credenciais {
	return Credenciais{Usuario: "usuario", Senha: segredoSenha, Token: segredoToken}
}

func vazou(t *testing.T, onde, texto string) {
	t.Helper()
	for _, s := range []string{segredoSenha, segredoToken} {
		if strings.Contains(texto, s) {
			t.Errorf("%s vazou as credenciais:\n%s", onde, texto)
			return
		}
	}
}

func TestCredenciais_NaoVazamEmFormatacao(t *testing.T) {
	c := credenciaisDeTeste()
	for _, f := range []string{"%v", "%+v", "%s"} {
		vazou(t, "fmt "+f, fmt.Sprintf(f, c))
	}
	dentro := struct {
		Municipio   string
		Credenciais Credenciais
	}{"3550308", c}
	vazou(t, "fmt %+v de struct que contém as credenciais", fmt.Sprintf("%+v", dentro))
}

func TestCredenciais_NaoVazamNoSlog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("transmitindo", "credenciais", credenciaisDeTeste())
	log.Info("transmitindo", slog.Any("credenciais", credenciaisDeTeste()))

	vazou(t, "slog", buf.String())
	if !strings.Contains(buf.String(), "redigido") {
		t.Errorf("o slog não usou LogValue: %s", buf.String())
	}
}

// O JSON continua carregando tudo: é como as credenciais chegam do cliente e
// vão para o worker. A redação é contra LOG, não contra transporte.
func TestCredenciais_JSONContinuaCarregandoOValor(t *testing.T) {
	b, err := json.Marshal(credenciaisDeTeste())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), segredoSenha) {
		t.Error("as credenciais precisam atravessar o JSON: é como chegam e como vão ao worker")
	}
}
