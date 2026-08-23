// Package api embute a especificação OpenAPI e a serve como bytes.
//
// A spec mora aqui, e não em internal/, porque é artefato PÚBLICO: quem gera
// SDK consome o arquivo, e um caminho estável importa. O go:embed exige que o
// arquivo esteja no diretório do pacote: daí este pacote de uma linha.
package api

import _ "embed"

//go:embed openapi.yaml
var openAPI []byte

// OpenAPI devolve a especificação em YAML.
func OpenAPI() []byte { return openAPI }
