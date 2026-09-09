package nfse

import (
	"strings"
	"testing"
)

// O INI do evento é o contrato com TInfEvento.LerFromIni. Estas chaves estão
// sob o lockstep (o leitor de pedidos entrou no snapshot), mas o gate confere
// que a lib LÊ a chave, não que escrevemos o VALOR certo nela.
func TestToINIEventoEscreveOPedidoDeRegistro(t *testing.T) {
	ini := ToINIEvento(EventoPN{
		Chave:     "3550308...44",
		TpAmb:     "2",
		VerAplic:  "wrapper-teste",
		DhEvento:  "2026-09-08 10:30:00",
		CodMotivo: "9",
		Motivo:    "duplicidade com a nota anterior",
	})
	for _, quero := range []string{
		"[Evento]", "tpAmb=2", "verAplic=wrapper-teste",
		"dhEvento=08/09/2026 10:30:00", "chNFSe=3550308...44",
		"tpEvento=e101101", "cMotivo=9", "xMotivo=duplicidade com a nota anterior",
	} {
		if !strings.Contains(ini, quero) {
			t.Errorf("INI não tem %q:\n%s", quero, ini)
		}
	}
	// chSubstituta só existe nos eventos de substituição: escrever em branco
	// faria a lib mandar a tag vazia num evento que não a tem.
	if strings.Contains(ini, "chSubstituta") {
		t.Errorf("chSubstituta num cancelamento simples:\n%s", ini)
	}
}

func TestValidarCancelamentoPN(t *testing.T) {
	casos := map[string]struct {
		p    CancelamentoPedido
		erro bool
	}{
		"motivo com 15 caracteres":  {CancelamentoPedido{Motivo: "servico errado."}, false},
		"código default é 1":        {CancelamentoPedido{Motivo: "servico nao prestado"}, false},
		"motivo com 14 caracteres":  {CancelamentoPedido{Motivo: "servico errado"}, true},
		"código do outro conjunto":  {CancelamentoPedido{Codigo: "05", Motivo: "servico nao prestado"}, true},
		"motivo só de espaços":      {CancelamentoPedido{Motivo: "                    "}, true},
		"código 9 exige motivo bom": {CancelamentoPedido{Codigo: "9", Motivo: "outro motivo qualquer"}, false},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			if msg := ValidarCancelamentoPN(c.p); (msg != "") != c.erro {
				t.Errorf("erro = %q, queria erro = %v", msg, c.erro)
			}
		})
	}
}

// A lib devolve as listas separadas por "|", com separador sobrando no fim.
func TestParseCapacidades(t *testing.T) {
	c := ParseCapacidades("[ObterInformacoesProvedor]\n" +
		"IdentificacaoProvedor=Nome:WebISS|Versao:2.02|Layout:ABRASF\n" +
		"AutenticacoesRequeridas=RequerCertificado|RequerLoginSenha|\n" +
		"ServicosDisponibilizados=EnviarLoteSincrono|CancelarNfse|\n")
	if c.Provedor != "WebISS" {
		t.Errorf("provedor = %q", c.Provedor)
	}
	if len(c.Servicos) != 2 || len(c.Autenticac) != 2 {
		t.Errorf("listas = %+v / %+v", c.Servicos, c.Autenticac)
	}
	if !c.Oferece("cancelarnfse") {
		t.Error("Oferece deveria ignorar a caixa: a lista vem da lib, com a grafia dela")
	}
	if c.Oferece(ServicoSubstituir) {
		t.Error("ofereceu um serviço que não está na lista")
	}
}
