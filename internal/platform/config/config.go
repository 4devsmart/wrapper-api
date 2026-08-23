// Package config carrega a configuração do serviço a partir de variáveis de
// ambiente — a única forma de configurar este wrapper.
//
// Não há banco, arquivo de configuração nem página de administração: o serviço
// é sem estado, e o que antes seria "cadastro" (empresa, certificado, ambiente)
// viaja no payload de cada requisição. O que sobra aqui é infraestrutura:
// endereço, autenticação, limites e onde encontrar os workers.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config agrega toda a configuração do serviço.
type Config struct {
	HTTPAddr string
	// AuthToken é o Bearer aceito pela API. Obrigatório em produção — sem ele
	// qualquer um transmite documentos com o certificado que enviar.
	AuthToken string
	// Modo distingue a implantação: "producao" endurece as validações do boot.
	// NÃO é o tpAmb do documento — esse vai no payload, por requisição, e a
	// mesma instância atende homologação e produção.
	Modo string
	// APIRatePerMin limita requisições por minuto por IP (0 = desligado).
	APIRatePerMin int
	// MaxBodyBytes limita o corpo aceito. Importa mais aqui do que num serviço
	// comum: cada transmissão carrega o certificado e o XML, e um corpo sem teto
	// vira vetor de exaustão de memória barato.
	MaxBodyBytes int64
	// TrustProxyHeaders confia em X-Forwarded-For quando o peer é proxy interno.
	TrustProxyHeaders bool
	ACBr              ACBr
}

// ACBr configura o binding da ACBrLib.
type ACBr struct {
	// Workers são os endereços dos processos fiscal-worker (caminho de socket
	// unix, "unix:///path" ou "http://host:porta"). Com pelo menos um, a API NÃO
	// carrega a lib nativa: delega por RPC, e um SIGSEGV na lib derruba o worker,
	// não a API. Vazio = binding no próprio processo (só faz sentido no worker).
	Workers []string
	// WorkerSlots é quantas chamadas simultâneas cada worker aceita. O default 1
	// dá o isolamento máximo — um crash da lib mata UMA requisição. Aumentar
	// eleva a vazão e, na mesma medida, quantas requisições um crash leva junto.
	WorkerSlots int
	// WorkerTimeout é o teto de uma chamada ao worker. Cobre o caso de a lib
	// TRAVAR (em vez de crashar): sem isso o slot ficaria preso para sempre.
	WorkerTimeout time.Duration
	// WorkerListen é o socket unix que o fiscal-worker abre (só o worker usa).
	WorkerListen string
	// WorkerMaxCalls recicla o processo do worker após N chamadas nativas. O
	// risco que isso limita não é o crash (contido pelo isolamento) e sim a
	// CORRUPÇÃO SILENCIOSA: memória estragada numa chamada sobrevivendo para a
	// seguinte, virando dado errado sem sintoma. 0 = nunca recicla. Reciclar com
	// UM worker abre janela de indisponibilidade no restart; use com 2+.
	WorkerMaxCalls int

	// IniBasePath é o diretório de RASCUNHO da lib nativa: cada sessão grava ali
	// o seu arquivo de config antes de operar. Não guarda dado de cliente e pode
	// ser efêmero (tmpfs serve) — o serviço não persiste nada.
	IniBasePath     string
	SchemasPath     string // XSD da NFS-e (por provedor)
	SchemasPathCTe  string // XSD do CT-e
	SchemasPathMDFe string // XSD do MDF-e
	SchemasPathNFe  string // XSD da NF-e (etapa futura: distribuição/manifestação)
	// LogNivel/LogPath ligam o log de depuração da ACBr. PERIGOSO: a partir do
	// nível 3 ela grava XML e CERTIFICADO em disco. Ligue só para depurar, e
	// apague depois — num serviço que recebe o certificado por payload, isso é a
	// diferença entre não persistir nada e persistir o pior.
	LogNivel string
	LogPath  string
	// DANFSeLocal gera o PDF do DANFSE localmente (exige widgetset GUI + Xvfb).
	DANFSeLocal bool
}

// EmProducao indica implantação de produção (endurece o boot).
func (c Config) EmProducao() bool { return strings.EqualFold(c.Modo, "producao") }

// Load lê a configuração do ambiente, com defaults sensatos para dev.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		AuthToken:         env("API_TOKEN", ""),
		Modo:              env("MODO", "homologacao"),
		APIRatePerMin:     envInt("API_RATE_PER_MIN", 240),
		MaxBodyBytes:      int64(envInt("MAX_BODY_BYTES", 8<<20)), // 8 MiB
		TrustProxyHeaders: envBool("TRUST_PROXY_HEADERS", true),
		ACBr: ACBr{
			Workers:         envList("ACBR_WORKERS"),
			WorkerSlots:     envInt("ACBR_WORKER_SLOTS", 1),
			WorkerTimeout:   time.Duration(envInt("ACBR_WORKER_TIMEOUT_SECONDS", 90)) * time.Second,
			WorkerListen:    env("ACBR_WORKER_LISTEN", "/run/wrapper/fiscal.sock"),
			WorkerMaxCalls:  envInt("ACBR_WORKER_MAX_CALLS", 0),
			IniBasePath:     env("ACBR_WORK_DIR", filepath.Join(os.TempDir(), "wrapper-acbr")),
			SchemasPath:     env("ACBR_SCHEMAS_PATH", "/opt/acbr/schemas"),
			SchemasPathCTe:  env("ACBR_SCHEMAS_PATH_CTE", "/opt/acbr/schemas-cte"),
			SchemasPathMDFe: env("ACBR_SCHEMAS_PATH_MDFE", "/opt/acbr/schemas-mdfe"),
			SchemasPathNFe:  env("ACBR_SCHEMAS_PATH_NFE", "/opt/acbr/schemas-nfe"),
			LogNivel:        env("ACBR_LOG_NIVEL", ""),
			LogPath:         env("ACBR_LOG_PATH", ""),
			DANFSeLocal:     envBool("ACBR_DANFSE_LOCAL", false),
		},
	}

	if cfg.EmProducao() {
		if cfg.AuthToken == "" {
			return Config{}, fmt.Errorf("produção exige API_TOKEN: sem ele a API aceita qualquer chamador")
		}
		// O log de nível 3+ da ACBr grava XML e certificado em disco. Num serviço
		// que recebe o certificado por payload, deixar isso ligado em produção
		// transforma "não persistimos nada" em mentira — e sem aviso.
		if n, _ := strconv.Atoi(cfg.ACBr.LogNivel); n >= 3 && cfg.ACBr.LogPath != "" {
			return Config{}, fmt.Errorf("ACBR_LOG_NIVEL=%s grava XML e certificado em disco: proibido em produção", cfg.ACBr.LogNivel)
		}
	}
	if cfg.MaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("MAX_BODY_BYTES deve ser positivo")
	}
	return cfg, nil
}

func env(chave, padrao string) string {
	if v := strings.TrimSpace(os.Getenv(chave)); v != "" {
		return v
	}
	return padrao
}

func envInt(chave string, padrao int) int {
	if v, err := strconv.Atoi(env(chave, "")); err == nil {
		return v
	}
	return padrao
}

func envBool(chave string, padrao bool) bool {
	switch strings.ToLower(env(chave, "")) {
	case "1", "true", "sim", "yes", "on":
		return true
	case "0", "false", "nao", "não", "no", "off":
		return false
	}
	return padrao
}

// envList lê uma lista separada por vírgula, ignorando entradas vazias.
func envList(chave string) []string {
	bruto := env(chave, "")
	if bruto == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(bruto, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
