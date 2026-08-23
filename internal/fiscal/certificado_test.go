package fiscal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// O certificado A1 e a senha dele chegam no corpo da requisição e vivem só na
// memória do processo. O jeito realista de vazarem é alguém logar a struct que
// os carrega, então a redação é a defesa, e defesa sem teste é intenção.

const (
	segredoPFX   = "conteudo-do-pfx-que-nao-pode-vazar"
	segredoSenha = "s3nh4-do-certificado"
)

func certDeTeste() Certificado {
	return Certificado{
		PFXBase64: base64.StdEncoding.EncodeToString([]byte(segredoPFX)),
		Senha:     segredoSenha,
	}
}

func vazou(t *testing.T, onde, texto string) {
	t.Helper()
	for _, s := range []string{segredoSenha, base64.StdEncoding.EncodeToString([]byte(segredoPFX))} {
		if strings.Contains(texto, s) {
			t.Errorf("%s vazou o certificado:\n%s", onde, texto)
			return
		}
	}
}

func TestCertificado_NaoVazaEmFormatacao(t *testing.T) {
	c := certDeTeste()

	// Direto: %v, %+v, %s e o próprio String().
	for _, f := range []string{"%v", "%+v", "%s", "%#v"} {
		if f == "%#v" {
			continue // %#v ignora Stringer por definição; não é caminho de log
		}
		vazou(t, "fmt "+f, fmt.Sprintf(f, c))
	}

	// Dentro de outra struct, que é o caso real: alguém loga o pedido inteiro.
	pedido := struct {
		Chave       string
		Certificado Certificado
	}{"35240812345678000190570010000000011000000010", c}
	vazou(t, "fmt %+v de struct que contém o certificado", fmt.Sprintf("%+v", pedido))
	vazou(t, "fmt %v de struct que contém o certificado", fmt.Sprintf("%v", pedido))
}

func TestCertificado_NaoVazaNoSlog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	c := certDeTeste()
	log.Info("transmitindo", "certificado", c)
	log.Info("transmitindo", slog.Any("certificado", c))
	vazou(t, "slog", buf.String())

	if !strings.Contains(buf.String(), "redigido") {
		t.Errorf("o slog não usou LogValue: %s", buf.String())
	}
}

// O JSON é o caminho legítimo: é assim que o certificado chega do cliente e vai
// para o worker. A redação é contra LOG, não contra serialização, e este teste
// existe para que a distinção fique explícita para quem mexer aqui.
func TestCertificado_JSONContinuaCarregandoOValor(t *testing.T) {
	b, err := json.Marshal(certDeTeste())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), segredoSenha) {
		t.Error("o certificado precisa atravessar o JSON: é como ele chega e como vai ao worker")
	}
}

func TestCertificado_Validar(t *testing.T) {
	valido := certDeTeste()
	casos := map[string]struct {
		cert Certificado
		erro string // trecho esperado; "" = sem erro
	}{
		"completo":       {valido, ""},
		"sem pfx":        {Certificado{Senha: "x"}, "obrigatório"},
		"pfx só espaços": {Certificado{PFXBase64: "   ", Senha: "x"}, "obrigatório"},
		"pfx não base64": {Certificado{PFXBase64: "não é base64!!", Senha: "x"}, "base64"},
		"pfx vazio":      {Certificado{PFXBase64: base64.StdEncoding.EncodeToString(nil), Senha: "x"}, "obrigatório"},
		"sem senha":      {Certificado{PFXBase64: valido.PFXBase64}, "senha"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			err := c.cert.Validar()
			switch {
			case c.erro == "" && err != nil:
				t.Errorf("esperava passar, veio: %v", err)
			case c.erro != "" && err == nil:
				t.Errorf("esperava erro contendo %q, passou", c.erro)
			case c.erro != "" && !strings.Contains(err.Error(), c.erro):
				t.Errorf("erro %q não menciona %q", err, c.erro)
			}
		})
	}
}

// O teto existe para o corpo absurdo não virar CPU gasta decodificando. Um A1 de
// verdade tem poucos KB.
func TestCertificado_ValidarRecusaPFXGigante(t *testing.T) {
	grande := Certificado{
		PFXBase64: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("a"), tamanhoMaxPFX+1)),
		Senha:     "x",
	}
	err := grande.Validar()
	if err == nil {
		t.Fatal("PFX acima do teto foi aceito")
	}
	if !strings.Contains(err.Error(), "limite") {
		t.Errorf("a mensagem precisa dizer que estourou o limite: %v", err)
	}
	vazou(t, "mensagem de erro", err.Error())
}

func TestCertificado_Informado(t *testing.T) {
	casos := map[Certificado]bool{
		{}:                          false,
		{PFXBase64: "   "}:          false,
		{PFXBase64: "x"}:            true,
		{PFXBase64: "", Senha: "s"}: false,
	}
	for c, quero := range casos {
		if got := c.Informado(); got != quero {
			t.Errorf("Informado(%+v) = %v, quero %v", c, got, quero)
		}
	}
}

// --- a sessão nativa --------------------------------------------------------

func TestTenant_LevaCertificadoEAmbiente(t *testing.T) {
	c := certDeTeste()
	tn := Tenant("12345678000190", "CTe", "producao", c)

	if tn.CNPJ != "12345678000190" {
		t.Errorf("CNPJ = %q", tn.CNPJ)
	}
	if tn.PFXBase64 != c.PFXBase64 || tn.SenhaPFX != c.Senha {
		t.Error("o certificado não chegou à sessão")
	}
	var ambiente string
	for _, kv := range tn.Config {
		if kv.Section == "CTe" && kv.Key == "Ambiente" {
			ambiente = kv.Value
		}
	}
	if ambiente != "0" {
		t.Errorf("Ambiente = %q, esperava 0 (produção no enum da lib)", ambiente)
	}
}

// A geração não assina e não fala com a SEFAZ: o certificado não deve nem
// existir na sessão. É a garantia que sustenta o contrato de duas chamadas.
func TestTenant_GeracaoNaoCarregaCertificado(t *testing.T) {
	tn := Tenant("12345678000190", "CTe", "homologacao", Certificado{})
	if tn.PFXBase64 != "" || tn.SenhaPFX != "" {
		t.Errorf("sessão de geração com certificado: %+v", tn)
	}
}

func TestTenant_EspacoNoPFXNaoVaiParaALib(t *testing.T) {
	c := Certificado{PFXBase64: "  " + certDeTeste().PFXBase64 + "  ", Senha: "x"}
	if tn := Tenant("1", "CTe", "", c); strings.HasPrefix(tn.PFXBase64, " ") {
		t.Error("espaço em volta do base64 chegou à lib")
	}
}
