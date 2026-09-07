package mdfe

import (
	"sort"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
	"github.com/4devsmart/wrapper-api/internal/platform/lockstep"
)

// Rede contra a classe de bug que já apareceu cinco vezes: um campo aceito pelo
// contrato, a emissão "funciona", e o dado some antes do XML.
//
// São duas comparações, em direções opostas:
//
//  1. a lib aceita e NÃO enviamos: lacuna de cobertura (baseline em testdata);
//  2. enviamos e a lib NÃO lê: chave morta, pior que a primeira porque parece
//     que funciona. Foi assim que as retenções federais da NFS-e sumiram, com o
//     builder escrevendo [tribFed] e a lib lendo [tribFederal].
//
// Nenhuma das duas pontas é palpite: o que escrevemos sai do INI que o
// construtor gerou, e o que a lib lê sai do fonte Pascal pinado
// (scripts/gerar-chaves-lerini.py). Ver internal/platform/lockstep.
//
// Falhar aqui não é "conserte o código", é "decida": passar a enviar, ou
// declarar com o motivo.

var chavesMortas = map[string]string{
	// infRespTec.idCSRT/hashCSRT existem na classe do MDF-e e o gravador de XML
	// os emite, mas só chegam lá por leitura de XML: o leitor de INI não os lê.
	// Mesma situação do CT-e (ver internal/cte/lockstep_test.go).
	"infRespTec/idCSRT":   "a lib não lê idCSRT do INI (só de XML)",
	"infRespTec/hashCSRT": "a lib não lê hashCSRT do INI (só de XML)",

	// O ambiente é configuração da sessão (ConfigGravarValor), não conteúdo do
	// documento: a lib o resolve antes de ler o INI. Escrevê-lo é inofensivo e
	// mantém o INI legível quando alguém o inspeciona no preview.
	"ide/tpAmb": "ambiente vem da config da sessão, não do INI",
}

// naoEnviadas complementa o baseline com casos que merecem motivo no código.
var naoEnviadas = map[string]string{}

// gruposNaoSuportados são seções inteiras fora do contrato.
var gruposNaoSuportados = map[string]string{}

// escritas é o que o construtor de fato escreve, lido do INI que ele gerou.
// Não há leitura de fonte Go: era ela que não via seção montada por
// concatenação e atribuía chave à seção errada.
func escritas(t *testing.T) lockstep.Escritas {
	t.Helper()
	// Os construtores de EVENTO entram junto: eles escrevem [EVENTO] e
	// [DETEVENTO], seções que a lib lê e que ficariam fora da conferência se
	// só o documento fosse considerado.
	const (
		ch   = "35260999999999000191570010000001001311178271"
		cnpj = "99999999000191"
		prot = "135260000000001"
		dh   = "2026-09-07T12:00:00-03:00"
	)
	casos := []espelho.Caso{
		{Novo: func() any { return &PedidoEmissao{} },
			Gerar: func(a any) string { return ToINI(*a.(*PedidoEmissao)) }, Grupos: gruposMDFe},
		{Novo: func() any { return &PedidoEncerramento{} },
			Gerar: func(a any) string { return ToINIEncerramento(ch, cnpj, prot, dh, *a.(*PedidoEncerramento)) }},
		{Novo: func() any { return &PedidoCancelamento{} },
			Gerar: func(a any) string { return ToINICancelamento(ch, cnpj, prot, dh, *a.(*PedidoCancelamento)) }},
		{Novo: func() any { return &PedidoInclusaoCondutor{} },
			Gerar: func(a any) string { return ToINIInclusaoCondutor(ch, cnpj, dh, *a.(*PedidoInclusaoCondutor)) }},
		{Novo: func() any { return &PedidoInclusaoDFe{} },
			Gerar: func(a any) string { return ToINIInclusaoDFe(ch, cnpj, dh, *a.(*PedidoInclusaoDFe)) }},
		{Novo: func() any { return &PedidoPagamentoOperacao{} },
			Gerar: func(a any) string { return ToINIPagamentoOperacao(ch, cnpj, dh, *a.(*PedidoPagamentoOperacao)) }},
	}
	var todas []lockstep.Escritas
	for _, c := range casos {
		todas = append(todas, espelho.SecoesEChaves(c))
	}
	return lockstep.Uniao(todas...)
}

func snapshot(t *testing.T) *lockstep.Snapshot {
	t.Helper()
	s, err := lockstep.Carregar("testdata/lerini_chaves.tsv")
	if err != nil {
		t.Fatalf("%v (rode 'make acbr-chaves')", err)
	}
	return s
}

func baseline(t *testing.T) map[string]bool {
	t.Helper()
	b, err := lockstep.CarregarBaseline("testdata/nao_enviadas.tsv")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLockstep_MDFe_ChavesDaLibQueNaoEnviamos(t *testing.T) {
	snap, nossas, base := snapshot(t), escritas(t), baseline(t)

	var novas []string
	for _, e := range snap.Entradas() {
		id := e.ID()
		if nossas.Enviamos(e) || base[e.IDLower()] || naoEnviadas[id] != "" ||
			gruposNaoSuportados[e.Secao] != "" {
			continue
		}
		novas = append(novas, id)
	}
	sort.Strings(novas)
	if len(novas) > 0 {
		t.Errorf(`%d chave(s) NOVAS que a lib aceita e nós não enviamos.

Decida cada uma: emitir no builder, ou acrescentar a testdata/nao_enviadas.tsv
com o motivo.

%s`, len(novas), "  "+strings.Join(novas, "\n  "))
	}
}

func TestLockstep_MDFe_ChavesQueEnviamosEALibIgnora(t *testing.T) {
	snap, nossas := snapshot(t), escritas(t)

	var mortas, ressuscitadas []string
	for _, id := range nossas.Mortas(snap) {
		if chavesMortas[id] == "" {
			mortas = append(mortas, id)
		}
	}
	for id := range chavesMortas {
		if sc := strings.SplitN(id, "/", 2); len(sc) == 2 && snap.Aceita(sc[0], sc[1]) {
			ressuscitadas = append(ressuscitadas, id)
		}
	}
	if len(mortas) > 0 {
		sort.Strings(mortas)
		t.Errorf(`%d chave(s) que ESCREVEMOS e a lib não lê: o dado é aceito e
descartado em silêncio, e a emissão parece ter funcionado.

Corrija o builder (nome ou seção errados?) ou declare em chavesMortas com o
motivo:

%s`, len(mortas), "  "+strings.Join(mortas, "\n  "))
	}
	if len(ressuscitadas) > 0 {
		sort.Strings(ressuscitadas)
		t.Errorf("estas estão em chavesMortas mas a lib passou a lê-las: "+
			"remova-as da lista para travar o ganho:\n  %s", strings.Join(ressuscitadas, "\n  "))
	}
}

// TestLockstep_MDFe_BaselineSemLinhaMorta cobra a higiene da lista: linha que não
// corresponde mais a nada (a lib parou de ler a chave, ou passamos a enviá-la)
// vira ruído que esconde a próxima lacuna de verdade.
func TestLockstep_MDFe_BaselineSemLinhaMorta(t *testing.T) {
	snap, nossas, base := snapshot(t), escritas(t), baseline(t)

	aceitas := map[string]bool{}
	naoEnviada := map[string]bool{}
	for _, e := range snap.Entradas() {
		aceitas[e.IDLower()] = true
		if !nossas.Enviamos(e) {
			naoEnviada[e.IDLower()] = true
		}
	}
	var obsoletas, jaEnviadas []string
	for id := range base {
		switch {
		case !aceitas[id]:
			obsoletas = append(obsoletas, id)
		case !naoEnviada[id]:
			jaEnviadas = append(jaEnviadas, id)
		}
	}
	sort.Strings(obsoletas)
	sort.Strings(jaEnviadas)
	if len(obsoletas) > 0 {
		t.Errorf("%d linha(s) do baseline que a lib não aceita mais: remova\n  %s",
			len(obsoletas), strings.Join(obsoletas, "\n  "))
	}
	if len(jaEnviadas) > 0 {
		t.Errorf("%d linha(s) do baseline que JÁ enviamos: remova para travar o ganho\n  %s",
			len(jaEnviadas), strings.Join(jaEnviadas, "\n  "))
	}
}
