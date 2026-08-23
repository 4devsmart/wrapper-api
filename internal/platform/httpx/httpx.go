// Package httpx são os utilitários HTTP compartilhados: leitura de JSON,
// escrita de resposta e o envelope de erro.
//
// Não conhece domínio fiscal, o que é comum aos documentos mas específico do
// fisco (certificado, ambiente, tradução de erro da lib) mora em internal/fiscal.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Erro é o envelope de erro da API. Um formato só, em toda rota: o cliente
// escreve UM tratamento.
type Erro struct {
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
	Detalhes any    `json:"detalhes,omitempty"`
}

type envelopeErro struct {
	Erro Erro `json:"erro"`
}

// JSON escreve v como JSON com o status informado.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErroJSON escreve o envelope de erro.
func ErroJSON(w http.ResponseWriter, status int, codigo, mensagem string) {
	JSON(w, status, envelopeErro{Erro{Codigo: codigo, Mensagem: mensagem}})
}

// ErroDetalhado escreve o envelope de erro com um corpo auxiliar (lista de
// rejeições, XML montado, etc.).
func ErroDetalhado(w http.ResponseWriter, status int, codigo, mensagem string, detalhes any) {
	JSON(w, status, envelopeErro{Erro{Codigo: codigo, Mensagem: mensagem, Detalhes: detalhes}})
}

// LerJSON decodifica o corpo em dst. Devolve false (e já respondeu) em erro.
//
// Rejeita CAMPO DESCONHECIDO de propósito. Num contrato fiscal, um campo com
// nome errado que o servidor ignora em silêncio vira documento transmitido com
// dado faltando: o pior desfecho possível, porque só aparece depois de a SEFAZ
// autorizar. Melhor um 400 dizendo qual campo o cliente errou. Cliente e
// servidor deste wrapper versionam juntos, então não há compatibilidade
// prospectiva a preservar.
func LerJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		ErroJSON(w, statusDeErroJSON(err), "json_invalido", mensagemDeErroJSON(err))
		return false
	}
	// Um segundo valor no corpo quase sempre é cliente montando o pedido errado.
	if dec.More() {
		ErroJSON(w, http.StatusBadRequest, "json_invalido", "o corpo deve conter um único objeto JSON")
		return false
	}
	return true
}

func statusDeErroJSON(err error) int {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func mensagemDeErroJSON(err error) string {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return "corpo maior que o limite do servidor (MAX_BODY_BYTES)"
	}
	var tipo *json.UnmarshalTypeError
	if errors.As(err, &tipo) {
		return "campo " + tipo.Field + ": esperava " + tipo.Type.String() + ", veio " + tipo.Value
	}
	msg := err.Error()
	// "json: unknown field \"x\"" é a mensagem mais útil da stdlib; traduz.
	if campo, ok := strings.CutPrefix(msg, "json: unknown field "); ok {
		return "campo desconhecido " + campo + ": confira o nome no contrato"
	}
	return "corpo JSON inválido: " + msg
}
