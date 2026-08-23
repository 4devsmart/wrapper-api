// Package modulo define a fronteira entre o servidor HTTP e cada documento
// fiscal (CT-e, MDF-e, NFS-e, boletos).
//
// A regra que este pacote existe para tornar verificável: o servidor não conhece
// documento fiscal, e um módulo não conhece outro módulo. Tudo que o CT-e
// precisa saber sobre HTTP mora em internal/cte/http.go; tudo que o servidor
// sabe sobre CT-e é o que está aqui — um nome e um conjunto de rotas.
//
// Sem isso, adicionar um campo de CT-e volta a tocar dois pacotes (foi assim no
// projeto de origem, onde um único handlers.go de 45 KB atendia todos os
// documentos).
package modulo

import "net/http"

// Router registra rotas JÁ SOB o prefixo do módulo. Um módulo chamado "cte"
// que registra "POST /xml" atende POST /v1/cte/xml — ele nunca escreve o
// prefixo, então renomear a versão da API ou o caminho base não toca módulo
// nenhum.
//
// O padrão é o do http.ServeMux (Go 1.22+): "MÉTODO /caminho/{param}".
type Router interface {
	Handle(padrao string, h http.Handler)
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Modulo é um documento fiscal exposto pela API.
type Modulo interface {
	// Nome é o segmento de caminho do módulo ("cte", "mdfe", "nfse", "boletos").
	// Precisa ser estável: é contrato público.
	Nome() string
	// Registrar declara as rotas do módulo no Router recebido.
	Registrar(r Router)
	// Capacidades lista o que este módulo sabe fazer nesta build, em termos do
	// contrato ("xml", "transmissao", "eventos", "pdf", …). Alimenta
	// GET /v1/capacidades, que é como o cliente descobre o que existe sem
	// tentar e tomar 404.
	Capacidades() []string
}
