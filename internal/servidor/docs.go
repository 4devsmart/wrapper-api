package servidor

import (
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/4devsmart/wrapper-api/api"
)

// rotasDocs registra a documentação. Fora da autenticação de propósito: a spec
// de uma API self-hosted não é segredo, e ter a referência à mão é o que torna
// o serviço consumível.
func (s *Servidor) rotasDocs() {
	s.mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(api.OpenAPI())
	})
	s.mux.HandleFunc("GET /docs", s.handleDocs)
}

// handleDocs serve um índice navegável das rotas.
//
// Não há Swagger UI: ele são ~1,9 MB de assets para vendorizar, e carregá-lo de
// CDN quebraria a política de não depender de rede externa. O índice abaixo é
// autocontido, responde à pergunta "que rotas existem e o que cada uma faz", e
// aponta para a spec completa em /openapi.yaml — que é o que ferramenta consome.
func (s *Servidor) handleDocs(w http.ResponseWriter, _ *http.Request) {
	mods := make([]moduloDoc, 0, len(s.capsJSON))
	for nome, caps := range s.capsJSON {
		mods = append(mods, moduloDoc{Nome: nome, Capacidades: strings.Join(caps, " · ")})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Nome < mods[j].Nome })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	_ = tmplDocs.Execute(w, struct {
		Base    string
		Modulos []moduloDoc
	}{Base: Base, Modulos: mods})
}

type moduloDoc struct {
	Nome        string
	Capacidades string
}

var tmplDocs = template.Must(template.New("docs").Parse(`<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>wrapper-api</title>
<style>
 :root{color-scheme:light dark}
 body{font:16px/1.6 system-ui,sans-serif;max-width:46rem;margin:3rem auto;padding:0 1.2rem}
 code{font-family:ui-monospace,monospace;font-size:.9em;background:#8881;padding:.1em .35em;border-radius:.25em}
 h1{font-size:1.5rem;margin-bottom:.2rem} h2{font-size:1.05rem;margin-top:2rem}
 .sub{opacity:.7;margin-top:0} table{border-collapse:collapse;width:100%}
 td{padding:.4rem .6rem .4rem 0;vertical-align:top;border-bottom:1px solid #8883}
 td:first-child{white-space:nowrap;font-family:ui-monospace,monospace}
 .aviso{border-left:3px solid #c93;padding:.2rem 0 .2rem .9rem;margin:1.5rem 0}
</style>
<h1>wrapper-api</h1>
<p class=sub>Documentos fiscais sem estado. Especificação completa em
<a href="/openapi.yaml"><code>/openapi.yaml</code></a>.</p>

<h2>Duas fases</h2>
<p>Emitir não é uma chamada só. A fase&nbsp;1 monta o documento e devolve o XML e
a identidade dele — <strong>sem certificado e sem rede</strong>. A fase&nbsp;2
assina e transmite.</p>
<pre><code>POST {{.Base}}/cte/xml          → { chave, xml_b64, validacao }
POST {{.Base}}/cte/transmissao  → { chave, protocolo, status, xml_proc_b64 }</code></pre>
<p>A fase&nbsp;1 põe o documento nas suas mãos <em>antes</em> de qualquer byte
sair. É isso que torna recuperável uma transmissão cuja resposta se perdeu.</p>

<div class=aviso>
<strong>Timeout na transmissão não autoriza repetir.</strong> A resposta
<code>502 desfecho_indeterminado</code> significa que o documento pode ter sido
autorizado. Consulte pela chave — repetir é como se duplica documento fiscal.
</div>

<h2>Módulos</h2>
<table>
{{range .Modulos}}<tr><td>{{$.Base}}/{{.Nome}}</td><td>{{.Capacidades}}</td></tr>
{{end}}</table>
<p>Também disponíveis sem autenticação: <code>/healthz</code>,
<code>/readyz</code>, <code>/openapi.yaml</code>.
Com token: <code>{{.Base}}/ping</code>, <code>{{.Base}}/capacidades</code>.</p>

<h2>Antes de produção</h2>
<p>Leia as limitações. As que mais surpreendem: perder o XML do CT-e ou do MDF-e
é perder o PDF (não há segunda via); não há proteção contra duplicidade; e a
NFS-e é multi-provedor, com capacidade descoberta em tempo de execução.</p>
`))
