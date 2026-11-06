package fiscal

import (
	"reflect"
	"testing"
)

func TestParseEnvio(t *testing.T) {
	casos := []struct {
		nome, resp, status string
		quero              Emissao
	}{
		{"vazia", "  ", "erro", Emissao{}},
		{"texto solto vira erro", "falha na lib", "rejeitado",
			Emissao{Erros: []Mensagem{{Descricao: "falha na lib"}}}},
		{"autorizado", "[Retorno]\ncStat=100\nxMotivo=Autorizado\nnProt=P1\nnRec=R1\nchCTe=C1\n", "autorizado",
			Emissao{Sucesso: true, Chave: "C1", Protocolo: "P1", CStat: "100", XMotivo: "Autorizado", Recibo: "R1"}},
		{"status de sucesso sem chave não é autorizado", "[Retorno]\ncStat=100\n", "rejeitado",
			Emissao{CStat: "100"}},
		{"rejeição com erros", "[Retorno]\ncStat=204\n[Erro1]\nCodigo=9\nDescricao=d\n[Erro2]\nCodigo=10\n", "rejeitado",
			Emissao{CStat: "204", Erros: []Mensagem{{"9", "d"}, {"10", ""}}}},
		{"valor vazio não apaga o que já veio", "[A]\nchCTe=C1\nnProt=P1\n[B]\nchCTe=\nnProt=\n", "erro",
			Emissao{Chave: "C1", Protocolo: "P1"}},
		{"nomes alternativos", "[A]\nchave=C2\nprotocolo=P2\n", "erro",
			Emissao{Chave: "C2", Protocolo: "P2"}},
		{"linha sem igual e em branco são ignoradas", "[A]\nsolta\n\ncStat=1\n", "rejeitado",
			Emissao{CStat: "1"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := ParseEnvio(c.resp, "chCTe", "100")
			if !reflect.DeepEqual(got, c.quero) {
				t.Errorf("= %+v, quero %+v", got, c.quero)
			}
			if s := StatusEmissao(got); s != c.status {
				t.Errorf("status = %q, quero %q", s, c.status)
			}
		})
	}
}
