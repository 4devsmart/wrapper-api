package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestProducaoExigeToken(t *testing.T) {
	t.Setenv("MODO", "producao")
	if _, err := Load(); err == nil {
		t.Fatal("produção sem API_TOKEN deveria falhar: a API aceitaria qualquer chamador")
	}
	t.Setenv("API_TOKEN", "abc")
	if _, err := Load(); err != nil {
		t.Fatalf("produção com API_TOKEN deveria carregar: %v", err)
	}
}

// O log de nível 3+ da ACBr grava XML e CERTIFICADO em disco. Num serviço cuja
// promessa é não persistir nada, deixá-lo ligado em produção transforma a
// promessa em mentira, e sem sintoma. Por isso é erro de boot, não warning.
func TestProducaoRecusaLogQueGravaCertificado(t *testing.T) {
	t.Setenv("MODO", "producao")
	t.Setenv("API_TOKEN", "abc")
	t.Setenv("ACBR_LOG_PATH", "/tmp/acbr")

	for _, nivel := range []string{"3", "4"} {
		t.Setenv("ACBR_LOG_NIVEL", nivel)
		_, err := Load()
		if err == nil {
			t.Errorf("ACBR_LOG_NIVEL=%s em produção deveria falhar", nivel)
			continue
		}
		if !strings.Contains(err.Error(), "certificado") {
			t.Errorf("a mensagem deveria dizer o motivo (certificado em disco): %v", err)
		}
	}

	// Níveis baixos não gravam o certificado e seguem permitidos.
	t.Setenv("ACBR_LOG_NIVEL", "2")
	if _, err := Load(); err != nil {
		t.Errorf("nível 2 deveria ser permitido: %v", err)
	}
	// Sem LogPath o log nem é escrito, então o nível é inócuo.
	t.Setenv("ACBR_LOG_NIVEL", "4")
	t.Setenv("ACBR_LOG_PATH", "")
	if _, err := Load(); err != nil {
		t.Errorf("nível alto sem LogPath não grava nada; deveria passar: %v", err)
	}
}

func TestHomologacaoNaoExigeToken(t *testing.T) {
	t.Setenv("MODO", "homologacao")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("homologação sem token deveria carregar: %v", err)
	}
	if cfg.EmProducao() {
		t.Error("EmProducao() verdadeiro em homologação")
	}
}

func TestDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.MaxBodyBytes <= 0 {
		t.Errorf("MaxBodyBytes = %d, precisa de teto (o corpo carrega certificado)", cfg.MaxBodyBytes)
	}
	if cfg.ACBr.WorkerSlots != 1 {
		t.Errorf("WorkerSlots = %d, quero 1 (isolamento máximo por padrão)", cfg.ACBr.WorkerSlots)
	}
	if len(cfg.ACBr.Workers) != 0 {
		t.Errorf("Workers = %v, quero vazio por padrão", cfg.ACBr.Workers)
	}
}

func TestEnvList(t *testing.T) {
	t.Setenv("ACBR_WORKERS", " /run/a.sock , ,/run/b.sock ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	quero := []string{"/run/a.sock", "/run/b.sock"}
	if len(cfg.ACBr.Workers) != len(quero) {
		t.Fatalf("Workers = %v, quero %v", cfg.ACBr.Workers, quero)
	}
	for i, w := range quero {
		if cfg.ACBr.Workers[i] != w {
			t.Errorf("Workers[%d] = %q, quero %q", i, cfg.ACBr.Workers[i], w)
		}
	}
}

// O limite por IP é promessa de segurança: se a variável existe e é lida, tem
// de valer. Zero desliga, e é opção legítima atrás de um gateway que já limita.
// Negativo não é nada: é engano de quem quis desligar e digitou -1.
func TestAPIRatePerMin(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIRatePerMin <= 0 {
		t.Errorf("APIRatePerMin = %d, o default precisa limitar alguma coisa", cfg.APIRatePerMin)
	}

	t.Setenv("API_RATE_PER_MIN", "0")
	if cfg, err := Load(); err != nil || cfg.APIRatePerMin != 0 {
		t.Errorf("zero deveria carregar e desligar: %d, %v", cfg.APIRatePerMin, err)
	}

	t.Setenv("API_RATE_PER_MIN", "-1")
	if _, err := Load(); err == nil {
		t.Error("negativo deveria falhar no boot em vez de virar limite desligado em silêncio")
	}
}

// X-Forwarded-For é escrito por quem chama. Dentro do Docker o peer da conexão
// é o gateway da bridge, que é privado: se o default confiasse no cabeçalho,
// todo chamador pareceria estar atrás de um proxy nosso e um valor forjado
// furaria o limite por endereço, sem sintoma nenhum. Confiar é ato explícito de
// quem pôs o proxy na frente.
func TestTrustProxyHeadersNaoConfiaPorPadrao(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrustProxyHeaders {
		t.Error("o default precisa ser não confiar: um cabeçalho forjado furaria o limite por endereço")
	}
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	if cfg, _ := Load(); !cfg.TrustProxyHeaders {
		t.Error("quem tem proxy de verdade precisa conseguir ligar")
	}
}

// A Config carrega o Bearer aceito pela API: vazado num log, ele permite a
// qualquer um transmitir documentos com o certificado que enviar. E como
// problema de configuração é o que mais se depura, ela é candidata natural a
// ser logada inteira.
func TestConfig_NaoVazaOToken(t *testing.T) {
	const segredo = "token-super-secreto-do-bearer"
	t.Setenv("API_TOKEN", segredo)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{"%v", "%+v", "%s"} {
		if strings.Contains(fmt.Sprintf(f, cfg), segredo) {
			t.Errorf("fmt %s vazou o token", f)
		}
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("subindo", "cfg", cfg)
	log.Info("subindo", slog.Any("cfg", cfg))
	if strings.Contains(buf.String(), segredo) {
		t.Errorf("o slog vazou o token:\n%s", buf.String())
	}

	// Redigir não pode significar ficar sem informação: config ilegível empurra
	// quem depura a logar os campos na mão, que é o caminho de volta ao
	// vazamento. O que interessa é se HÁ token, não qual é.
	for _, quero := range []string{"com_token", "modo", "workers"} {
		if !strings.Contains(buf.String(), quero) {
			t.Errorf("o log da config perdeu %q e virou inútil:\n%s", quero, buf.String())
		}
	}
	if !strings.Contains(buf.String(), `"com_token":true`) {
		t.Errorf("o log precisa dizer que existe token:\n%s", buf.String())
	}
}
