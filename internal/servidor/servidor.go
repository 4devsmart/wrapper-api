// Package servidor é a raiz de composição do HTTP: monta o mux, aplica os
// middlewares e pendura os módulos sob /v1/<nome>.
//
// Ele não conhece documento fiscal nenhum: só o contrato de internal/modulo.
package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/platform/config"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
)

// Base é o prefixo da API versionada. Módulos são montados sob Base+"/"+Nome.
const Base = "/v1"

// Servidor é o handler HTTP completo.
type Servidor struct {
	cfg      config.Config
	svc      *acbr.Servicos
	mux      *http.ServeMux
	modulos  []modulo.Modulo
	capsJSON map[string][]string
	taxa     *limitador
}

// Novo monta o servidor com os módulos informados. Passar nenhum módulo é
// legítimo: a API sobe, responde /healthz e /readyz, e anuncia zero capacidades.
func Novo(cfg config.Config, svc *acbr.Servicos, modulos ...modulo.Modulo) *Servidor {
	s := &Servidor{
		cfg:      cfg,
		svc:      svc,
		mux:      http.NewServeMux(),
		modulos:  modulos,
		capsJSON: map[string][]string{},
		taxa:     novoLimitador(cfg.APIRatePerMin),
	}
	s.rotas()
	return s
}

func (s *Servidor) rotas() {
	// Liveness e readiness ficam FORA de /v1 e fora da autenticação: quem os
	// consulta é o orquestrador, que não tem token, e um healthcheck que exige
	// credencial é um healthcheck que falha por motivo errado.
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	api := http.NewServeMux()
	api.HandleFunc("GET /ping", s.handlePing)
	api.HandleFunc("GET /capacidades", s.handleCapacidades)

	for _, m := range s.modulos {
		nome := strings.Trim(m.Nome(), "/")
		if nome == "" {
			panic("modulo sem nome")
		}
		if _, dup := s.capsJSON[nome]; dup {
			panic("modulo duplicado: " + nome)
		}
		caps := m.Capacidades()
		sort.Strings(caps)
		s.capsJSON[nome] = caps
		m.Registrar(prefixo{mux: api, base: "/" + nome})
	}

	// StripPrefix para que os módulos registrem caminhos relativos e o /v1 seja
	// trocável num lugar só.
	s.mux.Handle(Base+"/", http.StripPrefix(Base, s.exigirAuth(api)))

	// Depois dos módulos: o índice de /docs lista as capacidades deles.
	s.rotasDocs()
}

// Handler devolve o handler com a cadeia de middlewares aplicada.
func (s *Servidor) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.limitarCorpo(h)
	// O limite vem ANTES de ler o corpo e DEPOIS do registro: recusar cedo é o
	// ponto, e o 429 precisa aparecer no log como qualquer outro desfecho.
	h = s.limitarTaxa(h)
	h = s.registrar(h)
	h = s.recuperar(h)
	return h
}

// --- middlewares ------------------------------------------------------------

// recuperar transforma um panic em 500 e mantém o servidor vivo. Não cobre
// SIGSEGV da lib nativa: esse não é recuperável e é justamente por isso que a
// lib roda em outro processo (ver cmd/fiscal-worker).
func (s *Servidor) recuperar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic no handler", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
				escreverErro(w, http.StatusInternalServerError, "erro_interno", "erro interno")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// limitarCorpo põe teto no corpo da requisição. Importa mais aqui do que num
// serviço comum: toda transmissão carrega certificado e XML, e um corpo sem
// teto é exaustão de memória barata.
func (s *Servidor) limitarCorpo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func (s *Servidor) registrar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		// NUNCA logar corpo nem cabeçalho de autorização: o corpo carrega o
		// certificado A1 e a senha dele. O que se registra é rota e desfecho.
		slog.Info("http",
			"metodo", r.Method, "rota", r.URL.Path,
			"status", rec.status, "ms", time.Since(inicio).Milliseconds())
	})
}

// exigirAuth valida o Bearer. Token vazio na config só é tolerado fora de
// produção: config.Load recusa produção sem API_TOKEN.
func (s *Servidor) exigirAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		const prefixo = "Bearer "
		cab := r.Header.Get("Authorization")
		if len(cab) <= len(prefixo) || !strings.EqualFold(cab[:len(prefixo)], prefixo) ||
			!igual(cab[len(prefixo):], s.cfg.AuthToken) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="wrapper-api"`)
			escreverErro(w, http.StatusUnauthorized, "nao_autorizado", "token ausente ou inválido")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- handlers ---------------------------------------------------------------

func (s *Servidor) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz responde 503 quando o motor fiscal não está atendendo. É a
// diferença entre "o processo subiu" (healthz) e "dá para transmitir" (readyz):
// sem isso o orquestrador manda tráfego para uma instância sem worker.
func (s *Servidor) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.motorPronto(r.Context()); err != nil {
		escreverJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "indisponivel", "motivo": err.Error(),
		})
		return
	}
	escreverJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// motorPronto pergunta ao pool de workers se há alguém atendendo. Usa o /healthz
// do worker, não uma chamada nativa: responde "dá para transmitir?" sem abrir
// sessão na lib e sem depender de certificado.
//
// O type assertion é deliberado: só a implementação remota tem essa noção. Com
// o binding local (o próprio worker) não há o que checar, se o processo está
// de pé, o motor está.
func (s *Servidor) motorPronto(ctx context.Context) error {
	if s.svc == nil || s.svc.CTe == nil {
		return errSemMotor
	}
	// O stub compila e responde a tudo com "lib indisponível". Ele existe para a
	// API subir sem os .so, mas subir não é estar pronta: sem worker de verdade
	// nada é transmitido, e devolver 200 aqui faria o orquestrador mandar
	// tráfego para uma instância que só sabe errar.
	if s.svc.CTe.Backend() == "stub" {
		return errSemMotor
	}
	if sv, ok := s.svc.CTe.(interface{ Saudavel(context.Context) error }); ok {
		return sv.Saudavel(ctx)
	}
	// Binding local com a lib carregada (é o caso do próprio worker): se o
	// processo está de pé, o motor está.
	return nil
}

func (s *Servidor) handlePing(w http.ResponseWriter, _ *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"versao": versao.Atual(),
	})
}

// handleCapacidades diz o que esta build sabe fazer, por módulo. É como o
// cliente descobre o contrato sem tentar a rota e tomar 404.
func (s *Servidor) handleCapacidades(w http.ResponseWriter, _ *http.Request) {
	mods := make(map[string][]string, len(s.capsJSON))
	for k, v := range s.capsJSON {
		mods[k] = v
	}
	escreverJSON(w, http.StatusOK, map[string]any{
		"base":    Base,
		"modulos": mods,
		"versao":  versao.Atual(),
	})
}

// --- apoio ------------------------------------------------------------------

// prefixo adapta um *http.ServeMux ao modulo.Router, injetando o caminho base do
// módulo no padrão. Aceita tanto "POST /xml" quanto "/xml".
type prefixo struct {
	mux  *http.ServeMux
	base string
}

func (p prefixo) Handle(padrao string, h http.Handler) {
	p.mux.Handle(p.expandir(padrao), h)
}

func (p prefixo) HandleFunc(padrao string, h http.HandlerFunc) {
	p.mux.HandleFunc(p.expandir(padrao), h)
}

func (p prefixo) expandir(padrao string) string {
	metodo, caminho := "", strings.TrimSpace(padrao)
	if i := strings.IndexByte(caminho, ' '); i >= 0 {
		metodo, caminho = caminho[:i+1], strings.TrimSpace(caminho[i+1:])
	}
	if caminho == "/" || caminho == "" {
		// "POST /" no módulo cte vira "POST /cte/{$}": casa exatamente a raiz do
		// módulo, sem virar curinga de tudo que começa com /cte/.
		return metodo + p.base + "/{$}"
	}
	if !strings.HasPrefix(caminho, "/") {
		caminho = "/" + caminho
	}
	return metodo + p.base + caminho
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// igual compara em tempo constante para não vazar o token por timing.
func igual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := 0; i < len(a); i++ {
		d |= a[i] ^ b[i]
	}
	return d == 0
}

func escreverJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func escreverErro(w http.ResponseWriter, status int, codigo, msg string) {
	escreverJSON(w, status, map[string]any{"erro": map[string]string{"codigo": codigo, "mensagem": msg}})
}

var errSemMotor = errors.New("nenhum worker fiscal configurado (ACBR_WORKERS)")
