package acbr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// O que este processo manipula de mais sensível chega no corpo da requisição e
// vive só em memória: o certificado A1 de terceiros, a senha dele, e as
// credenciais de webservice do banco. Log é a única forma realista de um deles
// escapar daqui, então a redação é a defesa, e defesa sem teste é intenção.
//
// Vale para os DOIS caminhos, e por motivos diferentes:
//   - String() cobre fmt (%v, %+v, e a struct que contém a struct);
//   - LogValue() cobre o slog, e é o que realmente importa aqui: o serviço loga
//     em JSON, e o manipulador JSON NÃO consulta Stringer, ele serializa campo
//     a campo. Só com String(), o segredo sai inteiro.

const (
	segredoPFX   = "conteudo-do-pfx-que-nao-pode-vazar"
	segredoSenha = "s3nh4-do-certificado"
	segredoINI   = "ClientSecret=nao-pode-vazar"
	segredoChave = "-----BEGIN PRIVATE KEY----- material-mTLS"
)

func tenantDeTeste() TenantConfig {
	return TenantConfig{
		CNPJ:      "99999999000191",
		PFXBase64: segredoPFX,
		SenhaPFX:  segredoSenha,
		Config:    []ConfigKV{{Section: "DFe", Key: "Senha", Value: segredoSenha}},
	}
}

func boletoDeTeste() BoletoOnline {
	return BoletoOnline{
		INI:       segredoINI,
		ConfigINI: segredoINI,
		CertCRT:   []byte("cert"),
		CertKEY:   []byte(segredoChave),
	}
}

func vazou(t *testing.T, onde, texto string, segredos ...string) {
	t.Helper()
	for _, s := range segredos {
		if strings.Contains(texto, s) {
			t.Errorf("%s vazou:\n%s", onde, texto)
			return
		}
	}
}

func TestTenantConfig_NaoVazaEmFormatacao(t *testing.T) {
	c := tenantDeTeste()
	for _, f := range []string{"%v", "%+v", "%s"} {
		vazou(t, "fmt "+f, fmt.Sprintf(f, c), segredoPFX, segredoSenha)
	}
	// O caso real: alguém loga a struct que CONTÉM o tenant, para depurar.
	dentro := struct {
		Rota   string
		Tenant TenantConfig
	}{"/v1/cte/transmissao", c}
	vazou(t, "fmt %+v de struct que contém o tenant", fmt.Sprintf("%+v", dentro), segredoPFX, segredoSenha)
}

func TestTenantConfig_NaoVazaNoSlog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("abrindo sessão", "tenant", tenantDeTeste())
	log.Info("abrindo sessão", slog.Any("tenant", tenantDeTeste()))

	vazou(t, "slog", buf.String(), segredoPFX, segredoSenha)
	if !strings.Contains(buf.String(), "redigido") {
		t.Errorf("o slog não usou LogValue: %s", buf.String())
	}
}

func TestBoletoOnline_NaoVaza(t *testing.T) {
	o := boletoDeTeste()
	for _, f := range []string{"%v", "%+v", "%s"} {
		vazou(t, "fmt "+f, fmt.Sprintf(f, o), segredoINI, segredoChave)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("registrando", "op", o)
	vazou(t, "slog", buf.String(), segredoINI, segredoChave)
	if !strings.Contains(buf.String(), "redigido") {
		t.Errorf("o slog não usou LogValue: %s", buf.String())
	}
}

// O JSON é o caminho legítimo: é por ele que o tenant atravessa o socket até o
// worker. A redação é contra LOG, não contra transporte, e confundir as duas
// quebraria a transmissão inteira. Este teste existe para a distinção ficar
// explícita para quem mexer aqui.
func TestRedacao_NaoAtrapalhaOTransporte(t *testing.T) {
	b, err := json.Marshal(tenantDeTeste())
	if err != nil {
		t.Fatal(err)
	}
	for _, quero := range []string{segredoPFX, segredoSenha} {
		if !strings.Contains(string(b), quero) {
			t.Errorf("o tenant precisa atravessar o JSON inteiro: %s", b)
		}
	}

	b, err = json.Marshal(boletoDeTeste())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), segredoINI) {
		t.Errorf("a operação de boleto precisa atravessar o JSON inteiro: %s", b)
	}
}
