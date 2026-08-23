package servidor

import (
	"html/template"
	"io/fs"
	"net/http"

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

// handleDocs serve a página do Swagger UI.
//
// A página é só o container: a introdução (o que é, como gerar e transmitir, os
// códigos de erro) mora no `info.description` da própria spec. Duplicá-la aqui
// renderizaria tudo duas vezes — e o texto que vive na spec serve também quem
// abre o contrato no Postman, no Redoc ou num gerador de SDK.
func (s *Servidor) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// O Swagger UI injeta estilo e script inline; 'unsafe-inline' é o mínimo que
	// ele exige. Nada vem de fora: sem CDN em nenhuma diretiva.
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; "+
			"style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
			"font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
	_ = tmplDocs.Execute(w, struct{ Base string }{Base: Base})
}

var tmplDocs = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="pt-BR">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>wrapper-api · API</title>
<link rel="stylesheet" href="/docs/swagger-ui.css">
<style>
 /* TUDO escopado. Seletor de elemento solto (table, td, pre, code) atropela o
    CSS do Swagger UI — que é construído desses elementos — e quebra a
    renderização. Há teste que reprova isso. */
 body{margin:0}
 .swagger-ui .topbar{display:none}
 .swagger-ui .info{margin:2.5rem 0 1.5rem}
 .swagger-ui .info hgroup.main a{display:none}
 #swagger-ui{max-width:66rem;margin:0 auto;padding:0 1rem}

 /* O padrão do Swagger para markdown é ruim de ler: código em roxo #9012fe
    negrito, <pre> sem fundo nem padding, e word-break:break-all — que quebra
    palavra no meio e embaralha exemplo de linha de comando. */
 .swagger-ui .info .renderedMarkdown pre,
 .swagger-ui .info .markdown pre{
   background:#1e1f2b;border-radius:.5rem;padding:.85rem 1rem;margin:.9rem 0;
   overflow-x:auto;white-space:pre;word-break:normal;line-height:1.55}
 .swagger-ui .info .renderedMarkdown pre>code,
 .swagger-ui .info .markdown pre>code{
   background:none;color:#e8e8f0;font-weight:400;font-size:13px;padding:0}
 .swagger-ui .info .renderedMarkdown code,
 .swagger-ui .info .markdown code{
   background:#11182714;color:#1f3d2b;font-weight:500;
   padding:.1em .38em;border-radius:.25em}
 .swagger-ui .info .renderedMarkdown p,
 .swagger-ui .info .markdown p{word-break:normal;line-height:1.6}

 /* Tabela de erros: sem borda o olho não acompanha a linha até a última coluna,
    que é justamente a que importa ("pode repetir?"). */
 .swagger-ui .info table{border-collapse:collapse;width:100%;margin:1rem 0}
 .swagger-ui .info table th,
 .swagger-ui .info table td{
   padding:.45rem .7rem;text-align:left;vertical-align:top;
   border-bottom:1px solid #0000001f;font-size:13.5px}
 .swagger-ui .info table th{font-weight:600;border-bottom:2px solid #00000033}
 .swagger-ui .info table tr:hover td{background:#00000006}

 .swagger-ui .info h2{margin:2rem 0 .6rem;font-size:1.2rem}
 .swagger-ui .info h3{margin:1.6rem 0 .5rem;font-size:1rem}
 .swagger-ui .info blockquote{margin:1rem 0;padding:.1rem 1rem;
   border-left:3px solid #f0c674;background:#fff8e6}
</style>
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
