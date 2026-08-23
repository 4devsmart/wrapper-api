module github.com/4devsmart/wrapper-api

go 1.25.0

// Sem dependências externas: o serviço é stdlib + a lib nativa por cgo. Ao
// adicionar alguma, rode `go mod tidy` e justifique no PR — cada dependência
// aqui é superfície num serviço que manipula certificado alheio em memória.
