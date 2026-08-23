package config

import (
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
// promessa em mentira — e sem sintoma. Por isso é erro de boot, não warning.
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
