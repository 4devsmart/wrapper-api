// Package web embute os assets estáticos servidos pela API: hoje, o Swagger UI.
//
// Eles são VENDORIZADOS, não carregados de CDN. O motivo é o mesmo que vale para
// o resto do serviço: um wrapper fiscal self-hosted não deve depender de um host
// de terceiros para a própria documentação abrir, e a política de CSP daqui é
// estrita. O custo é ~1,7 MB no repositório e no binário.
//
// O go:embed exige que os arquivos estejam no diretório do pacote: daí este
// pacote de poucas linhas, como o api/.
package web

import "embed"

//go:embed swagger/swagger-ui-bundle.js swagger/swagger-ui.css swagger/VERSION
var swagger embed.FS

// Swagger devolve o sistema de arquivos com os assets do Swagger UI, já sem o
// prefixo "swagger/".
func Swagger() embed.FS { return swagger }
