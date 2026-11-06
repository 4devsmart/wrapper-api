package nfse

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/4devsmart/wrapper-api/internal/fiscal"
)

// maxRejeicoesNoLog limita a linha: o que importa é o motivo, não a lista toda.
const maxRejeicoesNoLog = 5

var (
	// O teto de 10 é o que mantém CNPJ e CPF fora: código de rejeição vem do
	// provedor, e um campo fora do padrão não pode virar dado pessoal no log.
	reCodigoRejeicao = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,10}$`)
	// O {tipo} da rota vem do cliente, e ainda não foi validado quando é lido.
	reTipoOperacao = regexp.MustCompile(`^[a-z-]{1,20}$`)
)

// registro é a resposta de uma operação, que vai juntando o que a linha de log
// precisa. Viaja no ResponseWriter porque ele já chega a todo ponto que decide a
// rota ou responde, sem mudar assinatura nenhuma.
type registro struct {
	http.ResponseWriter
	status    int
	rota      Rota
	ambiente  string
	desfecho  string
	rejeicoes []string
}

func (o *registro) WriteHeader(status int) {
	o.status = status
	o.ResponseWriter.WriteHeader(status)
}

// operacao registra uma linha por chamada: por onde a nota saiu, por que e como
// terminou. É o que responde "essa nota foi para onde?" sem reproduzir o pedido.
//
// NUNCA acrescente aqui CNPJ, CPF, chave, número da nota, XML ou a descrição da
// rejeição: a chave carrega o documento do emitente, e a descrição pode repetir
// dados do pedido. Rejeição é só o código.
func operacao(nome string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		o := &registro{ResponseWriter: w, status: http.StatusOK}
		h(o, r)
		attrs := []any{"operacao", nome, "http", o.status, "ms", time.Since(inicio).Milliseconds()}
		adicionar := func(chave, valor string) {
			if valor != "" {
				attrs = append(attrs, chave, valor)
			}
		}
		if tipo := r.PathValue("tipo"); reTipoOperacao.MatchString(tipo) {
			adicionar("tipo", tipo)
		}
		// Sem sessão, a rota nem chegou a ser decidida.
		if o.rota.Sessao != "" {
			// A distribuição chega aqui com o município do pedido sem validar.
			if len(o.rota.Municipio) == 7 {
				adicionar("municipio", o.rota.Municipio)
			}
			adicionar("sessao", o.rota.Sessao)
			adicionar("provedor", o.rota.Provedor())
			adicionar("layout", string(o.rota.Layout))
			adicionar("motivo", string(o.rota.Motivo))
		}
		adicionar("ambiente", o.ambiente)
		if o.rota.Sessao != "" {
			adicionar("nota_tecnica", NotaTecnicaDe(o.rota.Provedor(), o.ambiente))
		}
		adicionar("desfecho", o.desfecho)
		if len(o.rejeicoes) > 0 {
			attrs = append(attrs, "rejeicoes", o.rejeicoes)
		}
		slog.Info("nfse", attrs...)
	}
}

// registrarRota anota a rota mais recente: a última chamada é a que valeu.
func registrarRota(w http.ResponseWriter, rota Rota, ambiente string) {
	if o, ok := w.(*registro); ok {
		o.rota, o.ambiente = rota, fiscal.Primeiro(ambiente, o.ambiente)
	}
}

// registrarDesfecho anota como a operação terminou. Só os códigos das mensagens
// vão adiante, e só os que têm cara de código.
func registrarDesfecho(w http.ResponseWriter, desfecho string, erros []Mensagem) {
	o, ok := w.(*registro)
	if !ok {
		return
	}
	o.desfecho = desfecho
	for _, e := range erros {
		if len(o.rejeicoes) < maxRejeicoesNoLog && reCodigoRejeicao.MatchString(e.Codigo) {
			o.rejeicoes = append(o.rejeicoes, e.Codigo)
		}
	}
}
