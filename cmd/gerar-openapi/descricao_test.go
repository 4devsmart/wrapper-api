package main

import "testing"

// O comentário Go serve a quem mantém o modelo; a descrição da spec serve a quem
// integra. O gerador tira um do outro por recorte, e recorte deixa rastro:
// parêntese sem fecho, " .", ",.", artigo repetido, e o identificador Go
// abrindo a frase. Como o texto publicado é o produto, cada um desses é defeito.
//
// Os casos abaixo saíram do que estava REALMENTE publicado na spec antes deste
// conserto.

func TestSemDetalheDeImplementacao_ApartesInternos(t *testing.T) {
	casos := map[string]string{
		// O aparte era recortado por dentro e sobrava "do." com parêntese aberto.
		"CancelamentoPedido é o corpo do pedido de cancelamento (espelha o NfsePedidoCancelamento do ACBr.API). Códigos variam por prefeitura.": "CancelamentoPedido é o corpo do pedido de cancelamento. Códigos variam por prefeitura.",

		// Aparte com seção do arquivo intermediário.
		"DocReeRep é um documento de reembolso (gReeRepRes, seção [DocumentosNNNN]). Identifica o doc por chave DFe.": "DocReeRep é um documento de reembolso. Identifica o doc por chave DFe.",

		// Aparte legítimo continua inteiro: não é sobre implementação.
		"Peso da carga (em quilogramas).": "Peso da carga (em quilogramas).",

		// Citação de arquivo-fonte da biblioteca.
		"O grupo saiu do layout na 4.00 (pcteCTeW.pas:3309).": "O grupo saiu do layout na 4.00.",
	}
	for entrada, quero := range casos {
		if got := semDetalheDeImplementacao(entrada); got != quero {
			t.Errorf("entrada:\n  %s\ngot:\n  %s\nquero:\n  %s", entrada, got, quero)
		}
	}
}

func TestSemDetalheDeImplementacao_NomeDoProdutoDeOrigem(t *testing.T) {
	casos := map[string]string{
		"PedidoEmissao é o CT-e a ser gerado, no contrato da Nuvem Fiscal / ACBr.API.": "PedidoEmissao é o CT-e a ser gerado.",
		"DPSPedido é o corpo de emissão (estilo Nuvem Fiscal: POST /v1/nfse/xml).":     "DPSPedido é o corpo de emissão.",
	}
	for entrada, quero := range casos {
		if got := semDetalheDeImplementacao(entrada); got != quero {
			t.Errorf("entrada:\n  %s\ngot:\n  %s\nquero:\n  %s", entrada, got, quero)
		}
	}
}

// A troca do nome da biblioteca não pode duplicar o artigo que já estava na
// frase: "a ACBrLib resolve" virava "a a biblioteca fiscal resolve".
func TestSemDetalheDeImplementacao_NaoDuplicaArtigo(t *testing.T) {
	got := semDetalheDeImplementacao("a ACBrLib resolve o provedor pela tabela.")
	if got != "A biblioteca fiscal resolve o provedor pela tabela." && got != "a biblioteca fiscal resolve o provedor pela tabela." {
		t.Errorf("got %q", got)
	}
}

func TestNormalizarPontuacao(t *testing.T) {
	casos := map[string]string{
		"Identificador do processo de emissão. .": "Identificador do processo de emissão.",
		"Endereço (Padrão Nacional:":              "Endereço.",
		"algo ,. mais":                            "algo. mais.",
		"sem pontuação no fim":                    "sem pontuação no fim.",
		"Já termina certo!":                       "Já termina certo!",
		"Pergunta?":                               "Pergunta?",
		"Lista incompleta...":                     "Lista incompleta...",
		"":                                        "",
	}
	for entrada, quero := range casos {
		if got := normalizarPontuacao(entrada); got != quero {
			t.Errorf("normalizarPontuacao(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

// Parênteses aninhados quebram expressão regular; a varredura tem de fechar no
// par certo, senão come texto legítimo depois do aparte.
func TestRemoverApartesInternos_Aninhado(t *testing.T) {
	entrada := "Campo do layout (ver ACBr (versão MT)) e nada mais."
	quero := "Campo do layout e nada mais."
	if got := removerApartesInternos(entrada); got != quero {
		t.Errorf("got %q, quero %q", got, quero)
	}
}

func TestRemoverApartesInternos_ParenteseSemFecho(t *testing.T) {
	// Não é para explodir nem comer o resto: deixa como está e segue.
	entrada := "Texto com (aparte aberto e sem fim"
	if got := removerApartesInternos(entrada); got != entrada {
		t.Errorf("got %q, quero a entrada intacta", got)
	}
}

// O schema publicado se chama CteDocumento; a descrição não pode se apresentar
// como PedidoEmissao, que é nome que só existe no nosso código.
func TestSemNomeDoTipoGo(t *testing.T) {
	casos := []struct {
		doc, tipoGo, publicado, quero string
	}{
		{"PedidoEmissao é o CT-e a ser gerado.", "PedidoEmissao", "CteDocumento", "O CT-e a ser gerado."},
		{"Ide é a identificação do documento.", "Ide", "CteIde", "A identificação do documento."},
		// Quando o nome bate com o publicado, a frase fica: o leitor reconhece.
		{"Erro é o envelope de erro.", "Erro", "Erro", "Erro é o envelope de erro."},
		// Verbo fora da lista de definição: o nome é conteúdo, não convenção.
		{"Emit informa o emitente do documento.", "Emit", "CteEmit", "Emit informa o emitente do documento."},
		// Campo: qualquer identificador na abertura sai (tipoGo vazio).
		{"ExigibilidadeISS é campo do padrão ABRASF.", "", "exigibilidadeISS", "Campo do padrão ABRASF."},
		{"", "X", "Y", ""},
	}
	for _, c := range casos {
		if got := semNomeDoTipoGo(c.doc, c.tipoGo, c.publicado); got != c.quero {
			t.Errorf("semNomeDoTipoGo(%q, %q, %q) = %q, quero %q", c.doc, c.tipoGo, c.publicado, got, c.quero)
		}
	}
}

// Só a frase de abertura sai. Um comentário que é UMA frase e nada mais fica
// como está: sem ele a descrição some.
func TestSemNomeDoTipoGo_NaoEsvaziaADescricao(t *testing.T) {
	if got := semNomeDoTipoGo("Sacado é o pagador.", "Sacado", "BoletoSacado"); got != "O pagador." {
		t.Errorf("got %q", got)
	}
	if got := semNomeDoTipoGo("Sacado é", "Sacado", "BoletoSacado"); got != "Sacado é" {
		t.Errorf("frase sem resto deveria ficar intacta, got %q", got)
	}
}
