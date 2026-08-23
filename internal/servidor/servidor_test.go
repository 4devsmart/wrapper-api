package servidor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// moduloFake registra rotas relativas, como um módulo real faria.
type moduloFake struct {
	nome  string
	caps  []string
	rotas []string
}

func (m *moduloFake) Nome() string          { return m.nome }
func (m *moduloFake) Capacidades() []string { return m.caps }
func (m *moduloFake) Registrar(r modulo.Router) {
	for _, padrao := range m.rotas {
		p := padrao
		r.HandleFunc(p, func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte(m.nome + ":" + p))
		})
	}
}

func cfgTeste() config.Config {
	return config.Config{AuthToken: "s3gr3d0", MaxBodyBytes: 1 << 20, Modo: "homologacao"}
}

func servir(t *testing.T, cfg config.Config, mods ...modulo.Modulo) http.Handler {
	t.Helper()
	return Novo(cfg, &acbr.Servicos{}, mods...).Handler()
}

func TestHealthzNaoExigeToken(t *testing.T) {
	// O orquestrador não tem token. Um healthcheck que pede credencial falha
	// pelo motivo errado e derruba o container que estava são.
	rec := httptest.NewRecorder()
	servir(t, cfgTeste()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, quero 200", rec.Code)
	}
}

func TestAPIExigeBearer(t *testing.T) {
	h := servir(t, cfgTeste())
	casos := map[string]struct {
		cab  string
		quer int
	}{
		"sem cabeçalho":   {"", http.StatusUnauthorized},
		"token errado":    {"Bearer outro", http.StatusUnauthorized},
		"esquema errado":  {"Basic s3gr3d0", http.StatusUnauthorized},
		"prefixo parcial": {"Bearer s3gr3d", http.StatusUnauthorized},
		"correto":         {"Bearer s3gr3d0", http.StatusOK},
		"caixa do bearer": {"bearer s3gr3d0", http.StatusOK},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/capacidades", nil)
			if c.cab != "" {
				req.Header.Set("Authorization", c.cab)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.quer {
				t.Errorf("status = %d, quero %d", rec.Code, c.quer)
			}
		})
	}
}

func TestModuloMontaSobSeuPrefixo(t *testing.T) {
	m := &moduloFake{
		nome:  "cte",
		caps:  []string{"transmissao", "xml"},
		rotas: []string{"POST /xml", "POST /transmissao", "POST /eventos/{tipo}"},
	}
	h := servir(t, cfgTeste(), m)

	casos := map[string]int{
		"/v1/cte/xml":                    http.StatusOK,
		"/v1/cte/transmissao":            http.StatusOK,
		"/v1/cte/eventos/carta-correcao": http.StatusOK,
		"/v1/cte/inexistente":            http.StatusNotFound,
		"/v1/mdfe/xml":                   http.StatusNotFound,
	}
	for caminho, quer := range casos {
		req := httptest.NewRequest(http.MethodPost, caminho, nil)
		req.Header.Set("Authorization", "Bearer s3gr3d0")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != quer {
			t.Errorf("POST %s = %d, quero %d", caminho, rec.Code, quer)
		}
	}
}

func TestCapacidadesAnunciaOsModulos(t *testing.T) {
	h := servir(t, cfgTeste(),
		&moduloFake{nome: "cte", caps: []string{"xml", "transmissao"}, rotas: []string{"POST /xml"}},
		&moduloFake{nome: "mdfe", caps: []string{"xml"}, rotas: []string{"POST /xml"}},
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/capacidades", nil)
	req.Header.Set("Authorization", "Bearer s3gr3d0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var corpo struct {
		Base    string              `json:"base"`
		Modulos map[string][]string `json:"modulos"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&corpo); err != nil {
		t.Fatalf("resposta ilegível: %v", err)
	}
	if corpo.Base != Base {
		t.Errorf("base = %q, quero %q", corpo.Base, Base)
	}
	if len(corpo.Modulos) != 2 {
		t.Fatalf("módulos = %v, quero 2", corpo.Modulos)
	}
	// Ordenado: capacidades são contrato público e não podem variar de ordem
	// entre chamadas só porque o mapa iterou diferente.
	if got := strings.Join(corpo.Modulos["cte"], ","); got != "transmissao,xml" {
		t.Errorf("capacidades do cte = %q, quero ordenadas", got)
	}
}

func TestReadyzSemWorkerEhIndisponivel(t *testing.T) {
	// Sem motor fiscal a API sobe (healthz ok) mas não deve receber tráfego.
	// O caso REAL não é um *Servicos zerado e sim o stub que acbr.New devolve
	// quando não há ACBR_WORKERS — ele implementa tudo e responde "indisponível"
	// a tudo. Testar com o zerado esconderia justamente esse caminho.
	casos := map[string]*acbr.Servicos{
		"stub (sem ACBR_WORKERS)": acbr.New(config.ACBr{}),
		"servicos não montados":   {},
	}
	for nome, svc := range casos {
		t.Run(nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Novo(cfgTeste(), svc).Handler().
				ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("readyz = %d, quero 503", rec.Code)
			}
			// healthz continua 200: o processo está vivo, é o motor que não está.
			rec = httptest.NewRecorder()
			Novo(cfgTeste(), svc).Handler().
				ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("healthz = %d, quero 200", rec.Code)
			}
		})
	}
}

func TestCorpoAcimaDoTetoEhRecusado(t *testing.T) {
	cfg := cfgTeste()
	cfg.MaxBodyBytes = 32
	m := &moduloFake{nome: "cte", rotas: []string{"POST /xml"}}
	m.rotas = []string{"POST /xml"}
	h := Novo(cfg, &acbr.Servicos{}, &moduloEco{moduloFake: *m}).Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/cte/xml", strings.NewReader(strings.Repeat("x", 1024)))
	req.Header.Set("Authorization", "Bearer s3gr3d0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("corpo de 1 KiB passou com teto de 32 B (status %d)", rec.Code)
	}
}

// moduloEco lê o corpo inteiro — é o que dispara o MaxBytesReader.
type moduloEco struct{ moduloFake }

func (m *moduloEco) Registrar(r modulo.Router) {
	r.HandleFunc("POST /xml", func(w http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 4096)
		for {
			if _, err := req.Body.Read(buf); err != nil {
				if err.Error() != "EOF" {
					http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
				}
				return
			}
		}
	})
}

// --- documentação -----------------------------------------------------------

// A spec e o índice ficam FORA da autenticação: a especificação de uma API
// self-hosted não é segredo, e ter a referência à mão é o que torna o serviço
// consumível. Exigir token aqui só atrapalha quem está integrando.
func TestDocsNaoExigemToken(t *testing.T) {
	h := servir(t, cfgTeste(), &moduloFake{nome: "cte", caps: []string{"xml"}, rotas: []string{"POST /xml"}})
	for _, caminho := range []string{"/openapi.yaml", "/docs"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, caminho, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, quero 200", caminho, rec.Code)
		}
	}
}

// A spec precisa ser YAML de verdade e cobrir as rotas — servir um arquivo
// truncado ou vazio seria pior que não servir.
func TestSpecEhServidaEIntegra(t *testing.T) {
	rec := httptest.NewRecorder()
	servir(t, cfgTeste()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))

	corpo := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Errorf("Content-Type = %q", ct)
	}
	for _, quero := range []string{
		"openapi: 3.1.0", "/v1/cte/xml:", "/v1/cte/transmissao:",
		"/v1/mdfe/xml:", "/v1/nfse/consulta-dps:", "/v1/boletos/pdf:",
		"desfecho_indeterminado",
	} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("spec não menciona %q", quero)
		}
	}
}

// O Swagger UI é servido do BINÁRIO, não de CDN. Se estes assets sumirem, a
// página abre quebrada — e num serviço self-hosted não há CDN para socorrer.
func TestAssetsDoSwaggerVemDoBinario(t *testing.T) {
	h := servir(t, cfgTeste())
	for caminho, tipo := range map[string]string{
		"/docs/swagger-ui-bundle.js": "javascript",
		"/docs/swagger-ui.css":       "css",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, caminho, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, quero 200", caminho, rec.Code)
			continue
		}
		if rec.Body.Len() < 10000 {
			t.Errorf("GET %s devolveu %d bytes — asset truncado?", caminho, rec.Body.Len())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tipo) {
			t.Errorf("GET %s Content-Type = %q, quero %s", caminho, ct, tipo)
		}
	}
}

// A página não pode buscar NADA de fora: é a política do projeto, e a CSP é o
// que a torna verificável em vez de intenção.
func TestPaginaDeDocsNaoBuscaNadaDeFora(t *testing.T) {
	rec := httptest.NewRecorder()
	servir(t, cfgTeste()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	corpo := rec.Body.String()
	if strings.Contains(corpo, "//unpkg.com") || strings.Contains(corpo, "//cdn.") ||
		strings.Contains(corpo, "https://cdn") || strings.Contains(corpo, "jsdelivr") {
		t.Error("a página de docs referencia CDN externa")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP não restringe a origem: %q", csp)
	}
	// O Swagger UI precisa de inline, mas nada além da própria origem.
	for _, proibido := range []string{"http://", "https://"} {
		if strings.Contains(csp, proibido) {
			t.Errorf("CSP libera host externo: %q", csp)
		}
	}
	if !strings.Contains(corpo, `SwaggerUIBundle`) || !strings.Contains(corpo, `url: "/openapi.yaml"`) {
		t.Error("a página não monta o Swagger UI sobre a spec local")
	}
}

// O índice lista os módulos montados — se ele não acompanhar as capacidades,
// vira documentação que mente.
func TestIndiceDeDocsListaOsModulos(t *testing.T) {
	h := servir(t, cfgTeste(),
		&moduloFake{nome: "cte", caps: []string{"xml", "transmissao"}, rotas: []string{"POST /xml"}},
		&moduloFake{nome: "boletos", caps: []string{"pdf"}, rotas: []string{"POST /pdf"}},
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	corpo := rec.Body.String()
	for _, quero := range []string{"/v1/cte/xml", "/v1/cte/transmissao", "desfecho_indeterminado"} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a página não menciona %q", quero)
		}
	}
}

// Seletor de elemento solto (table, td, pre, code) no CSS da página atropela o
// do Swagger UI, que é construído desses elementos — foi assim que a renderização
// quebrou uma vez. Todo estilo próprio precisa estar escopado.
func TestCSSDaPaginaNaoAtropelaOSwagger(t *testing.T) {
	rec := httptest.NewRecorder()
	servir(t, cfgTeste()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	corpo := rec.Body.String()
	i, j := strings.Index(corpo, "<style>"), strings.Index(corpo, "</style>")
	if i < 0 || j < 0 {
		t.Fatal("bloco <style> não encontrado")
	}
	for _, linha := range strings.Split(corpo[i:j], "\n") {
		linha = strings.TrimSpace(linha)
		sel, _, tem := strings.Cut(linha, "{")
		if !tem || strings.HasPrefix(linha, "/*") || strings.HasPrefix(linha, "*") {
			continue
		}
		sel = strings.TrimSpace(sel)
		if sel == "" || strings.HasPrefix(sel, "@") || strings.HasPrefix(sel, ".") ||
			strings.HasPrefix(sel, "#") || strings.HasPrefix(sel, ":") {
			continue
		}
		// Sobrou um seletor que começa por nome de elemento: só body é aceitável.
		if sel != "body" {
			t.Errorf("seletor de elemento sem escopo %q — vai atropelar o Swagger UI", sel)
		}
	}
}
