package fiscal

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// svcFake implementa só os dois métodos que a geração usa. O embed satisfaz o
// resto da interface; chamar qualquer outro método aqui é erro de teste, e o
// panic de ponteiro nil avisa na hora.
type svcFake struct {
	acbr.Servico

	xml       string
	errMontar error

	respValidar string
	errValidar  error
}

func (f svcFake) MontarXML(acbr.TenantConfig, string) (acbr.Result, error) {
	return acbr.Result{XML: f.xml}, f.errMontar
}

func (f svcFake) ValidarRegras(acbr.TenantConfig, string) (acbr.Result, error) {
	return acbr.Result{Resposta: f.respValidar}, f.errValidar
}

// --- a validação não pode engolir falha que não seja falta de suporte --------

// TestMontar_FalhaNaValidacaoNaoViraOK cobre o defeito que existia aqui: a
// geração são DUAS chamadas ao worker, e a segunda tratava qualquer erro como
// "esta lib não valida". Worker reciclado, timeout ou vaga esgotada viravam
// validacao{ok:true, suportada:false}, e o cliente transmitia um documento que
// ninguém conferiu, com resposta 200.
func TestMontar_FalhaNaValidacaoNaoViraOK(t *testing.T) {
	falha := errors.New("worker fiscal caiu: " + acbr.ErrUnavailable.Error())
	svc := svcFake{xml: "<CTe/>", errValidar: falha}

	xml, val, _, err := Montar(svc, acbr.TenantConfig{}, "[ide]")
	if err == nil {
		t.Fatalf("falha na validação precisa subir; veio xml=%q val=%+v", xml, val)
	}
	if val.OK {
		t.Error("validacao.ok=true numa validação que não rodou")
	}
	if xml != "" {
		t.Error("XML devolvido junto com o erro: o chamador pode achar que serve")
	}
}

// TestMontar_NaoSuportadoContinuaSendoRespostaValida garante que o conserto não
// virou rigor demais: NFS-e não expõe validação de regras, e a sentinela é o
// jeito de dizer isso sem mentir que passou.
func TestMontar_NaoSuportadoContinuaSendoRespostaValida(t *testing.T) {
	svc := svcFake{xml: "<DPS/>", errValidar: acbr.ErrNaoSuportado}

	xml, val, _, err := Montar(svc, acbr.TenantConfig{}, "[DPS]")
	if err != nil {
		t.Fatalf("ErrNaoSuportado não é falha da geração: %v", err)
	}
	if xml != "<DPS/>" {
		t.Errorf("XML perdido: %q", xml)
	}
	if val.Suportada {
		t.Error("validacao.suportada deveria ser false onde a lib não valida")
	}
	if !val.OK {
		t.Error("sem validação disponível, ok=true é o contrato declarado")
	}
}

func TestMontar_RejeicaoDeRegraDeNegocio(t *testing.T) {
	svc := svcFake{xml: "<CTe/>", respValidar: "252 - Ambiente informado diverge\n"}

	_, val, _, err := Montar(svc, acbr.TenantConfig{}, "[ide]")
	if err != nil {
		t.Fatalf("rejeição de regra não é erro de infraestrutura: %v", err)
	}
	if val.OK || !val.Suportada {
		t.Errorf("esperava reprovada e suportada, veio %+v", val)
	}
	if len(val.Mensagens) != 1 {
		t.Errorf("a rejeição precisa chegar ao cliente: %+v", val.Mensagens)
	}
}

// TestResponderErroDaLib_StatusPorSentinela pina o mapeamento que o conserto
// acima passou a exercitar de verdade. A diferença entre 503 e 502 é a diferença
// entre "pode repetir" e "consulte antes de repetir".
func TestResponderErroDaLib_StatusPorSentinela(t *testing.T) {
	casos := map[string]struct {
		err   error
		quero int
	}{
		"indisponível":  {acbr.ErrUnavailable, http.StatusServiceUnavailable},
		"não suportado": {acbr.ErrNaoSuportado, http.StatusUnprocessableEntity},
		"indeterminado": {acbr.ErrIndeterminado, http.StatusBadGateway},
		"erro da lib":   {errors.New("falhou"), http.StatusBadGateway},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			ResponderErroDaLib(w, "CT-e", acbr.Result{}, c.err)
			if w.Code != c.quero {
				t.Errorf("status %d, esperava %d", w.Code, c.quero)
			}
		})
	}
}
