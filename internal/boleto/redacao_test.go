package boleto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// ContaWS já redigia a si mesma, e não tinha teste. Redação sem teste é
// intenção: basta alguém remover o LogValue num refactor e nada acusa.
const (
	segredoSecret = "client-secret-que-nao-pode-vazar"
	segredoChave  = "-----BEGIN PRIVATE KEY----- material-mTLS"
)

func contaWSDeTeste() ContaWS {
	return ContaWS{
		ClientID:     "id-publico",
		ClientSecret: segredoSecret,
		CertKEY:      segredoChave,
	}
}

func vazou(t *testing.T, onde, texto string) {
	t.Helper()
	for _, s := range []string{segredoSecret, segredoChave} {
		if strings.Contains(texto, s) {
			t.Errorf("%s vazou as credenciais do banco:\n%s", onde, texto)
			return
		}
	}
}

func TestContaWS_NaoVaza(t *testing.T) {
	c := contaWSDeTeste()
	for _, f := range []string{"%v", "%+v", "%s"} {
		vazou(t, "fmt "+f, fmt.Sprintf(f, c))
	}
	// O caso real: alguém loga a conta inteira, que CONTÉM as credenciais.
	dentro := struct {
		Banco string
		WS    ContaWS
	}{"237", c}
	vazou(t, "fmt %+v da conta", fmt.Sprintf("%+v", dentro))

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("registrando", "ws", c)
	log.Info("registrando", slog.Any("ws", c))
	vazou(t, "slog", buf.String())
	if !strings.Contains(buf.String(), "redigido") {
		t.Errorf("o slog não usou LogValue: %s", buf.String())
	}
}

// O JSON é o caminho legítimo: é por ele que as credenciais chegam do cliente e
// vão para o worker registrar o boleto.
func TestContaWS_JSONContinuaCarregandoOValor(t *testing.T) {
	b, err := json.Marshal(contaWSDeTeste())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), segredoSecret) {
		t.Error("as credenciais precisam atravessar o JSON: é como chegam e como vão ao worker")
	}
}
