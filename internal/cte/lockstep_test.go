package cte

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
	// O IEST (inscrição estadual do substituto tributário) existe na classe do
	// CT-e e o gravador de XML o emite, mas o leitor de INI nunca o lê, então
	// não há caminho para enviá-lo por este fluxo. Mantido no contrato porque
	// removê-lo quebraria clientes que já o mandam; documentado como limitação.
	"emit/IEST": "a lib não lê IEST do INI (só de XML), sem caminho por este fluxo",

	// idem: infRespTec.idCSRT/hashCSRT existem na classe e o gravador de XML os
	// emite, mas só chegam lá por leitura de XML. Pelo INI não há caminho.
	"infRespTec/idCSRT":   "a lib não lê idCSRT do INI (só de XML)",
	"infRespTec/hashCSRT": "a lib não lê hashCSRT do INI (só de XML)",

	// Na ICMSSN (Simples Nacional) do CT-e o leitor só consome indSN: o CST não
	// entra no XML deste grupo. Escrevê-lo é inofensivo e mantém simetria com os
	// outros grupos de ICMS do contrato.
	"ICMSSN/CST": "no CT-e o grupo ICMSSN só tem indSN; a lib não lê CST daqui",
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
			Gerar: func(a any) string { return ToINI(*a.(*PedidoEmissao)) }, Grupos: gruposCTe},
		{Novo: func() any { return &PedidoSimp{} },
			Gerar: func(a any) string { return ToINISimp(*a.(*PedidoSimp)) }},
		{Novo: func() any { return &PedidoCancelamento{} },
			Gerar: func(a any) string { return ToINICancelamento(ch, cnpj, prot, dh, *a.(*PedidoCancelamento)) }},
		{Novo: func() any { return &PedidoCartaCorrecao{} },
			Gerar: func(a any) string { return ToINICartaCorrecao(ch, cnpj, dh, *a.(*PedidoCartaCorrecao)) }},
		{Novo: func() any { return &PedidoComprovanteEntrega{} },
			Gerar: func(a any) string { return ToINIComprovanteEntrega(ch, cnpj, dh, *a.(*PedidoComprovanteEntrega)) }},
		{Novo: func() any { return &PedidoDesacordo{} },
			Gerar: func(a any) string { return ToINIDesacordo(ch, cnpj, dh, *a.(*PedidoDesacordo)) }},
		{Novo: func() any { return &PedidoEPEC{} },
			Gerar: func(a any) string { return ToINIEPEC(ch, cnpj, dh, *a.(*PedidoEPEC)) }},
		{Novo: func() any { return &PedidoInsucessoEntrega{} },
			Gerar: func(a any) string { return ToINIInsucessoEntrega(ch, cnpj, dh, *a.(*PedidoInsucessoEntrega)) }},
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

func TestLockstep_CTe_ChavesDaLibQueNaoEnviamos(t *testing.T) {
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

func TestLockstep_CTe_ChavesQueEnviamosEALibIgnora(t *testing.T) {
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

// TestLockstep_CTe_BaselineSemLinhaMorta cobra a higiene da lista: linha que não
// corresponde mais a nada (a lib parou de ler a chave, ou passamos a enviá-la)
// vira ruído que esconde a próxima lacuna de verdade.
func TestLockstep_CTe_BaselineSemLinhaMorta(t *testing.T) {
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
