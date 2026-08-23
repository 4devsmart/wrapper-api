package boleto

import (
	"strings"
	"testing"
)

func pedidoFixture() Pedido {
	return Pedido{
		Conta: Conta{
			Banco: "237", TipoCobranca: 5, CNAB: "1",
			Agencia: "1234", DigitoAgencia: "5", NumeroConta: "67890", DigitoConta: "1",
			Nome: "EMPRESA TESTE LTDA", CNPJCPF: "99999999000191", TipoInscricao: 1,
			Cidade: "Sao Paulo", UF: "SP", Convenio: "123456",
			TipoCarteira: 1,
		},
		Titulos: []Titulo{{
			NumeroDocumento: "000010", NossoNumero: "0000001", Carteira: "09",
			ValorDocumento: 150.50, Vencimento: "2026-06-10",
			Sacado: Sacado{Nome: "JOAO PAGADOR", CNPJCPF: "11122233344",
				Logradouro: "Rua A", Cidade: "Sao Paulo", UF: "SP", CEP: "01001000"},
			Instrucoes: []string{"Nao receber apos vencimento"},
		}},
	}
}

func TestToINI_SecoesEChaves(t *testing.T) {
	ini := ToINI(pedidoFixture())
	must := []string{
		"[Banco]", "Numero=237", "TipoCobranca=5", "CNAB=1",
		"[Conta]", "Agencia=1234", "Conta=67890",
		"[Cedente]", "Nome=EMPRESA TESTE LTDA", "CNPJCPF=99999999000191",
		"[Titulo1]", "NumeroDocumento=000010", "NossoNumero=0000001", "Carteira=09",
		"ValorDocumento=150,50", "Vencimento=10/06/2026",
		"Sacado.NomeSacado=JOAO PAGADOR", "Sacado.CNPJCPF=11122233344",
		"Instrucao1=Nao receber apos vencimento",
	}
	for _, frag := range must {
		if !strings.Contains(ini, frag) {
			t.Errorf("INI não contém %q\n---\n%s", frag, ini)
		}
	}
}

func TestToINI_ValorComVirgula(t *testing.T) {
	if strings.Contains(ToINI(pedidoFixture()), "ValorDocumento=150.50") {
		t.Error("valor saiu com ponto; o INI do ACBr exige vírgula")
	}
}
