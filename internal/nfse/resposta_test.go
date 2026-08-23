package nfse

import "testing"

// Fixtures anonimizadas espelhando a estrutura INI das respostas do ACBr
// (NFSE_Emitir / NFSE_Cancelar).

// TestParseEnvio_ABRASF: provedor ABRASF identifica a nota por Número +
// CodigoVerificacao (não há chave de 50 dígitos / Link).
func TestParseEnvio_ABRASF(t *testing.T) {
	resp := "[Envio]\nSucesso=1\nNumeroNota=4521\nCodigoVerificacao=A1B2-C3D4\nDescSituacao=Normal\n"
	e := ParseEnvio(resp)
	if !e.Sucesso || e.Numero != "4521" || e.CodigoVerificacao != "A1B2-C3D4" {
		t.Errorf("retorno ABRASF mal parseado: %+v", e)
	}
	if e.Chave != "" {
		t.Errorf("ABRASF não tem chave; veio %q", e.Chave)
	}
}

func TestOperacaoNaoSuportada(t *testing.T) {
	naoSup := []string{
		"Serviço Cancelar não implementado para este provedor.",
		"Serviço não implementado pelo Provedor.",
	}
	for _, s := range naoSup {
		if !OperacaoNaoSuportada(s) {
			t.Errorf("deveria detectar não-suportado: %q", s)
		}
	}
	sup := []string{
		"[Envio]\nSucesso=1\nNumeroNota=10\n",
		"[Envio]\nSucesso=0\n[Erro1]\nCodigo=E0014\nDescricao=DPS duplicada\n",
		"",
	}
	for _, s := range sup {
		if OperacaoNaoSuportada(s) {
			t.Errorf("falso positivo de não-suportado: %q", s)
		}
	}
}

func TestParseEnvio_Sucesso(t *testing.T) {
	resp := "[Envio]\nSucesso=1\nNumeroNota=12\nLink=43149022203780307000140000000000001226051163474655\nCodigoVerificacao=ABC123\nDescSituacao=Autorizada\n"
	e := ParseEnvio(resp)
	if !e.Sucesso {
		t.Fatal("esperava Sucesso=true")
	}
	if e.Numero != "12" {
		t.Errorf("Numero = %q, esperado 12", e.Numero)
	}
	if e.Chave == "" {
		t.Error("Chave (Link) não preenchida")
	}
	if e.CodigoVerificacao != "ABC123" {
		t.Errorf("CodigoVerificacao = %q", e.CodigoVerificacao)
	}
	if len(e.Erros) != 0 {
		t.Errorf("não deveria haver erros: %+v", e.Erros)
	}
	if StatusFromEmissao := statusHelper(e); StatusFromEmissao != "autorizado" {
		t.Errorf("status = %q, esperado autorizado", StatusFromEmissao)
	}
}

func TestParseEnvio_Erro(t *testing.T) {
	resp := "[Envio]\nSucesso=0\n[Erro1]\nCodigo=E0014\nDescricao=DPS duplicada\n"
	e := ParseEnvio(resp)
	if e.Sucesso {
		t.Error("esperava Sucesso=false")
	}
	if len(e.Erros) != 1 || e.Erros[0].Codigo != "E0014" {
		t.Fatalf("erro não parseado: %+v", e.Erros)
	}
}

func TestParseEnvio_NaoINI(t *testing.T) {
	e := ParseEnvio("Exceção: algo deu errado")
	if e.Sucesso || len(e.Erros) != 1 {
		t.Errorf("texto não-INI deveria virar um erro único: %+v", e)
	}
}

func TestParseCancelamento(t *testing.T) {
	ok := ParseCancelamento("[Cancelamento]\nSucesso=1\nDataHora=2026-05-30T18:00:00\n")
	if !ok.Sucesso {
		t.Error("esperava Sucesso=true")
	}
	if StatusCancelamento(ok) != "concluido" {
		t.Errorf("status = %q, esperado concluido", StatusCancelamento(ok))
	}

	rej := ParseCancelamento("[Cancelamento]\nSucesso=0\n[Erro1]\nCodigo=X\nDescricao=Falhou\n")
	if StatusCancelamento(rej) != "rejeitado" {
		t.Errorf("status = %q, esperado rejeitado", StatusCancelamento(rej))
	}
}

// statusHelper reusa a lógica de status da emissão (definida em handlers via
// statusEmissao; aqui replicamos a regra mínima para o teste do parser).
func statusHelper(e Emissao) string {
	switch {
	case e.Sucesso:
		return "autorizado"
	case len(e.Erros) > 0:
		return "rejeitado"
	default:
		return "erro"
	}
}
