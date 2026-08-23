package servidor

import (
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strings"

	"github.com/4devsmart/wrapper-api/api"
	"github.com/4devsmart/wrapper-api/web"
)

// rotasDocs registra a documentação. Fora da autenticação de propósito: a spec
// de uma API self-hosted não é segredo, e ter a referência à mão — inclusive
// para experimentar — é o que torna o serviço consumível.
func (s *Servidor) rotasDocs() {
	s.mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(api.OpenAPI())
	})
	s.mux.HandleFunc("GET /docs", s.handleDocs)
	s.mux.HandleFunc("GET /docs/{$}", s.handleDocs)

	// Assets do Swagger UI, servidos do binário. NÃO vêm de CDN: um wrapper
	// fiscal self-hosted não deve depender de host de terceiros para a própria
	// documentação abrir, e a CSP daqui é estrita.
	sub, err := fs.Sub(web.Swagger(), "swagger")
	if err != nil {
		panic("assets do swagger ausentes: " + err.Error())
	}
	s.mux.Handle("GET /docs/", http.StripPrefix("/docs/", cacheLongo(http.FileServerFS(sub))))
}

// cacheLongo marca os assets como imutáveis: eles só mudam quando o binário
// muda, então revalidar a cada carga de página é desperdício.
func cacheLongo(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		h.ServeHTTP(w, r)
	})
}

// handleDocs serve o Swagger UI com um preâmbulo curto.
//
// O preâmbulo não é enfeite: o Swagger lista rotas, mas não explica por que
// emitir são DUAS chamadas nem o que fazer quando a transmissão dá timeout — e
// essas duas coisas são o que separa usar a API corretamente de duplicar
// documento fiscal.
func (s *Servidor) handleDocs(w http.ResponseWriter, _ *http.Request) {
	mods := make([]moduloDoc, 0, len(s.capsJSON))
	for nome, caps := range s.capsJSON {
		mods = append(mods, moduloDoc{Nome: nome, Capacidades: strings.Join(caps, " · ")})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Nome < mods[j].Nome })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// O Swagger UI injeta estilo e script inline; 'unsafe-inline' é o mínimo que
	// ele exige. Nada vem de fora: sem CDN em nenhuma diretiva.
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; "+
			"style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
			"font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
	_ = tmplDocs.Execute(w, struct {
		Base    string
		Modulos []moduloDoc
		Versao  string
	}{Base: Base, Modulos: mods, Versao: web.VersaoSwagger()})
}

type moduloDoc struct {
	Nome        string
	Capacidades string
}

var tmplDocs = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="pt-BR">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>wrapper-api · API</title>
<link rel="stylesheet" href="/docs/swagger-ui.css">
<style>
 /* TUDO daqui é escopado em .preambulo. Seletor de elemento solto (table, td,
    pre, code) atropela o CSS do Swagger UI — que é construído justamente
    desses elementos — e quebra a renderização abaixo. */
 body{margin:0;background:#fff;color:#1a1a1a;
      font:15px/1.55 ui-sans-serif,system-ui,-apple-system,sans-serif}
 .preambulo{max-width:64rem;margin:0 auto;padding:2rem 1.5rem .5rem}
 .preambulo h1{font-size:1.35rem;margin:0 0 .15rem;letter-spacing:-.01em}
 .preambulo .sub{margin:0 0 1.5rem;color:#555;font-size:.95rem}
 .preambulo .sub a{color:#3b5bdb}
 .preambulo pre{margin:0;padding:1rem 1.1rem;overflow-x:auto;background:#1e1e28;
                color:#e6e6ef;border-radius:.5rem;font-size:.82rem;line-height:1.5}
 .preambulo pre .c{color:#8b8ba7}
 .preambulo pre .k{color:#7fd3a0}
 .preambulo code{font-family:ui-monospace,SFMono-Regular,monospace;font-size:.85em}
 .preambulo .passos{display:grid;gap:.75rem;grid-template-columns:1fr 1fr;margin-bottom:1rem}
 .preambulo .nota{margin:1rem 0 1.6rem;padding:.75rem 1rem;font-size:.9rem;
                  background:#fff6e5;border:1px solid #f0c674;border-radius:.5rem;color:#5c4300}
 .preambulo .nota code{background:#0000000d;padding:.05em .3em;border-radius:.2em}
 @media (max-width:52rem){.preambulo .passos{grid-template-columns:1fr}}
 #swagger-ui{max-width:64rem;margin:0 auto}
 .swagger-ui .topbar{display:none}
 .swagger-ui .info{margin:1.5rem 0}
</style>

<div class=preambulo>
  <h1>wrapper-api</h1>
  <p class=sub>Documentos fiscais em duas fases, sem estado.
     Spec: <a href="/openapi.yaml">/openapi.yaml</a> ·
     Saúde: <a href="/healthz">/healthz</a>, <a href="/readyz">/readyz</a></p>

  <div class=passos>
    <pre><span class=c># 1. montar — devolve o XML e a chave. Sem certificado.</span>
curl -X POST {{.Base}}/cte/xml \
  -H <span class=k>&#39;Authorization: Bearer $TOKEN&#39;</span> \
  -H <span class=k>&#39;Content-Type: application/json&#39;</span> \
  -d @cte.json
<span class=c># → { "chave": "3526…", "xml_b64": "…" }</span></pre>
    <pre><span class=c># 2. transmitir — o certificado entra aqui, e só aqui.</span>
curl -X POST {{.Base}}/cte/transmissao \
  -H <span class=k>&#39;Authorization: Bearer $TOKEN&#39;</span> \
  -H <span class=k>&#39;Content-Type: application/json&#39;</span> \
  -d <span class=k>&#39;{"xml_b64":"…","certificado":{"pfx_b64":"…","senha":"…"}}&#39;</span>
<span class=c># → { "protocolo": "135…", "xml_proc_b64": "…" }</span></pre>
  </div>

  <p class=sub style="margin-bottom:0">Guarde o <code>xml_proc_b64</code>: não há
  segunda via. A fase 1 existe para você ter a chave <strong>antes</strong> de
  transmitir — é ela que salva quando a resposta se perde.</p>

  <div class=nota>
    <strong>Timeout na transmissão não autoriza repetir.</strong>
    <code>502 desfecho_indeterminado</code> significa que o documento pode ter
    sido autorizado. Consulte pela chave (<code>{{.Base}}/cte/consulta</code>,
    <code>{{.Base}}/nfse/consulta-dps</code>) — repetir duplica documento fiscal.
  </div>

  <p class=sub>Para experimentar abaixo: <strong>Authorize</strong> → seu
  <code>API_TOKEN</code>.</p>
</div>

<div id="swagger-ui"></div>

<script src="/docs/swagger-ui-bundle.js"></script>
<script>
 window.ui = SwaggerUIBundle({
   url: "/openapi.yaml",
   dom_id: "#swagger-ui",
   deepLinking: true,
   persistAuthorization: true,
   docExpansion: "none",
   defaultModelsExpandDepth: 0,
   defaultModelExpandDepth: 3,
   tryItOutEnabled: true,
   supportedSubmitMethods: ["get", "post"],
 });
</script>
</html>
`))
