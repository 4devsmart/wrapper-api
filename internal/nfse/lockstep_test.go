package nfse

import (
	"sort"
	"strings"
	"testing"

	"github.com/4devsmart/wrapper-api/internal/platform/espelho"
	"github.com/4devsmart/wrapper-api/internal/platform/lockstep"
)

// Este arquivo é a rede contra a classe de bug que já apareceu três vezes
// (CodigoPais, CodigoNBS, CodigoServicoNacional/NumeroProcesso): um campo que a
// lib aceita, ou que um builder envia, e o outro caminho descarta em silêncio.
// O cliente manda, a API aceita, e o dado some antes do XML.
//
// São três comparações:
//
//  1. chaves que a LIB aceita e que NÃO enviamos (lacuna de cobertura);
//  2. chaves que ENVIAMOS e a LIB não lê (chave morta: pior que a primeira,
//     porque parece que funciona). Esta faltava aqui, e foi por isso que o
//     grupo [tribFed] passou meses sendo descartado: a biblioteca lê
//     [tribFederal], e nada comparava essa direção no NFS-e;
//  3. campos do contrato consumidos por UM builder e ignorados pelo OUTRO.
//
// As duas pontas são mecânicas, e nenhuma lê fonte com expressão regular: o que
// escrevemos sai do INI que o construtor gerou (espelho.SecoesEChaves), e o que
// a biblioteca lê sai do fonte Pascal (scripts/gerar-chaves-lerini.py).
// Ver internal/platform/lockstep.
//
// Falhar aqui não significa "corrija o código", significa "decida": passar a
// enviar o campo, ou declará-lo nas listas abaixo com o motivo.

// naoEnviadas complementa o baseline em testdata/nao_enviadas.tsv com os casos
// que merecem motivo explícito no código.
var naoEnviadas = map[string]string{
	// Preenchidos pela lib/provedor na resposta, não no pedido.
	"Servico/xMunicipio":                 "descritivo devolvido pelo provedor",
	"Servico/xItemListaServico":          "descritivo devolvido pelo provedor",
	"Servico/xCodigoTributacaoMunicipio": "descritivo devolvido pelo provedor",
	"Servico/xNBS":                       "descritivo devolvido pelo provedor",

	// Prefixo do logradouro ("Rua", "Avenida"). O contrato: espelhando a Nuvem
	// Fiscal: carrega o logradouro inteiro num campo só (xLgr), então não há o
	// que separar. Prestador/ e Tomador/ já estavam no baseline; Servico/ apareceu
	// no bump para r47859, quando o endereço do local da prestação ganhou seção
	// própria no LerIni.
	"Servico/TipoLogradouro": "o contrato usa um logradouro único (xLgr), sem prefixo separado",
}

// gruposNaoSuportados são seções inteiras que o contrato não cobre. Aparecem no
// snapshot da lib mas não são lacuna: são escopo que decidimos não atender.
var gruposNaoSuportados = map[string]string{
	// As quatro abaixo foram conferidas no fonte: são lidas SÓ pelo LerIniNfse,
	// o caminho que carrega uma NFS-e pronta. O LerIniRps, que é o que o nosso
	// INI alimenta, não passa por nenhuma delas, então não há como enviá-las
	// num RPS. Conferir com o mapa de chamadas do ACBrNFSeX.LerIni.pas.
	//
	// A justificativa antiga de [Emitente] dizia que o emitente ia por
	// ConfigGravarValor. É falsa: não existe seção de emitente na config da lib
	// (nem ACBrLibNFSeConfig.pas nem o DataModule têm uma), e o nosso
	// ConfigGravarValor só escreve NFSe, DANFSe e DFe. O que a seção faz é
	// copiar CNPJ, IM, razão social, endereço e contato para NFSe.Prestador,
	// que já mandamos: escrevê-la seria um segundo caminho para o mesmo dado,
	// com a chance de sobrescrever o prestador se divergirem.
	"Emitente":         "lida só pelo LerIniNfse, e copia para Prestador, que já enviamos",
	"ValoresNFSe":      "lida só pelo LerIniNfse: os valores calculados da nota emitida",
	"NFSeCancelamento": "lida só pelo LerIniNfse: o desfecho do cancelamento, que lemos da resposta",
	"OrgaoGerador":     "lida só pelo LerIniNfse: órgão gerador, preenchido pelo provedor",

	// Esta o LerIniRps LÊ, e é por isso que escrevemos TipoXML=RPS nela. As 16
	// chaves que faltam são da nota já emitida (Id, cStat, dhProc, nNFSe,
	// ambGer, procEmi, tpEmis) ou descritivos que o provedor devolve (xLocEmi,
	// xTribNac, xNBS): quem preenche é o fisco, não quem emite.
	"IdentificacaoNFSe": "alcançável pelo RPS, mas são os campos que o fisco atribui",

	// Seções do leitor de PEDIDOS (ACBrNFSeXWebserviceBase.pas), que entrou no
	// snapshot junto com [CancelarNFSe] e [Evento].
	//
	// A justificativa anterior dizia que consulta não vai por INI. É falsa: a
	// lib exporta NFSE_ConsultarNFSeGenerico e NFSE_ConsultarLinkNFSe, e as
	// duas recebem exatamente estas seções (TInfConsultaNFSe.LerFromIni e
	// TInfConsultaLinkNFSe.LerFromIni, ACBrLibNFSeBase.pas:977 e :1024). O que
	// é verdade é outra coisa: não ligamos esses dois símbolos. O acbrlib.h não
	// os declara, e as nossas consultas usam as entradas POSICIONAIS (PorChave,
	// PorNumero, PorFaixa, PorRps), que não leem INI nenhum.
	//
	// Enquanto o binding não existir, as 32 chaves não têm para onde ir. No dia
	// em que existir, estas duas linhas saem daqui e viram contrato.
	"ConsultarNFSe":     "INI da consulta genérica; falta ligar NFSE_ConsultarNFSeGenerico",
	"ConsultarLinkNFSe": "INI da consulta de link; falta ligar NFSE_ConsultarLinkNFSe",
}

// escritas é o que os construtores de fato escrevem, lido do INI que eles
// geraram. Não há leitura de fonte Go aqui: era ela que não via seção montada
// por concatenação e atribuía chave à seção errada.
//
// São cinco: os dois da nota (Padrão Nacional e ABRASF) e os três dos pedidos
// (cancelamento, evento e a DPS substituta do Padrão Nacional).
func escritas(t *testing.T) lockstep.Escritas {
	t.Helper()
	caso := func(g func(DPSPedido) string) espelho.Caso {
		return espelho.Caso{
			Novo:   func() any { return &DPSPedido{} },
			Gerar:  func(a any) string { return g(*a.(*DPSPedido)) },
			Grupos: gruposNFSe,
		}
	}
	return lockstep.Uniao(
		espelho.SecoesEChaves(caso(ToINI)),
		espelho.SecoesEChaves(caso(ToINIAbrasf)),
		// Os INIs que NÃO são a nota. O de cancelamento já era enviado e nunca
		// passou por aqui: o leitor de pedidos não estava no snapshot.
		espelho.SecoesEChaves(espelho.Caso{
			Novo: func() any { return &CancelamentoPedido{} },
			Gerar: func(a any) string {
				return ToINICancelamento("chave", "3550308", *a.(*CancelamentoPedido))
			},
		}),
		espelho.SecoesEChaves(espelho.Caso{
			Novo:  func() any { return &EventoPN{} },
			Gerar: func(a any) string { return ToINIEvento(*a.(*EventoPN)) },
		}),
		espelho.SecoesEChaves(espelho.Caso{
			Novo:   func() any { return &SubstituicaoPedido{} },
			Gerar:  func(a any) string { return ToINISubstituicaoPN(*a.(*SubstituicaoPedido)) },
			Grupos: gruposNFSe,
		}),
	)
}

func snapshot(t *testing.T) *lockstep.Snapshot {
	t.Helper()
	s, err := lockstep.Carregar("testdata/lerini_chaves.tsv")
	if err != nil {
		t.Fatalf("%v (rode 'make acbr-chaves')", err)
	}
	return s
}

func TestLockstep_ChavesDaLibQueNaoEnviamos(t *testing.T) {
	snap, nossas := snapshot(t), escritas(t)
	baseline := carregarBaseline(t)

	var novas []string
	for _, e := range snap.Entradas() {
		id := e.ID()
		if nossas.Enviamos(e) || naoEnviadas[id] != "" || baseline[e.IDLower()] ||
			gruposNaoSuportados[e.Secao] != "" {
			continue
		}
		novas = append(novas, id)
	}
	sort.Strings(novas)
	if len(novas) > 0 {
		t.Errorf(`%d chave(s) NOVAS que a lib aceita e nós não enviamos.

Não são todas as lacunas: só as que apareceram depois do último baseline
(normalmente após um bump do .so). Decida cada uma: emitir no builder, ou
acrescentar a testdata/nao_enviadas.tsv, com o motivo.

%s`, len(novas), "  "+strings.Join(novas, "\n  "))
	}
}

// chavesMortas são chaves que NÓS escrevemos e o leitor de INI da lib não lê.
// Cada uma é uma decisão registrada, não um bug tolerado.
var chavesMortas = map[string]string{
	// O leitor lê CNPJCPF para os dois, e só o PRESTADOR tem o CNPJ como
	// segunda opção (ReadString(s,'CNPJCPF', ReadString(s,'CNPJ',''))). No
	// tomador e no intermediário o CNPJ sozinho é ignorado. Continuamos
	// escrevendo os dois porque o valor viaja no CNPJCPF, que é lido, e alguns
	// provedores esperam o par.
	"Tomador/CNPJ":       "a lib só lê CNPJCPF no tomador; o valor vai por lá",
	"Intermediario/CNPJ": "idem no intermediário",
}

func TestLockstep_NFSe_ChavesQueEnviamosEALibIgnora(t *testing.T) {
	snap, nossas := snapshot(t), escritas(t)

	var mortas, ressuscitadas []string
	for _, id := range nossas.Mortas(snap) {
		if chavesMortas[id] == "" {
			mortas = append(mortas, id)
		}
	}
	for id := range chavesMortas {
		sc := strings.SplitN(id, "/", 2)
		if len(sc) == 2 && snap.Aceita(sc[0], sc[1]) {
			ressuscitadas = append(ressuscitadas, id)
		}
	}
	if len(mortas) > 0 {
		sort.Strings(mortas)
		t.Errorf(`%d chave(s) que ESCREVEMOS e a lib não lê: o dado é aceito e
descartado em silêncio, e a emissão parece ter funcionado.

Foi assim que o grupo inteiro de retenções federais sumiu: escrevíamos a seção
[tribFed] e a lib lê [tribFederal]. Corrija o builder (nome ou seção errados?)
ou declare em chavesMortas com o motivo:

%s`, len(mortas), "  "+strings.Join(mortas, "\n  "))
	}
	if len(ressuscitadas) > 0 {
		sort.Strings(ressuscitadas)
		t.Errorf("estas estão em chavesMortas mas a lib passou a lê-las: "+
			"remova-as da lista para travar o ganho:\n  %s", strings.Join(ressuscitadas, "\n  "))
	}
}

func TestLockstep_UmBuilderEnviaEOOutroNao(t *testing.T) {
	// Campos legítimos de um layout só (o outro não tem destino para eles).
	exclusivos := map[string]string{
		"IdentificacaoRps/TipoXML":          "PN: marcador do tipo de XML",
		"Servico/CodigoServicoNacional":     "ABRASF: PN deriva do ItemListaServico",
		"Valores/Aliquota":                  "ABRASF: no PN a alíquota vai em [tribMun]",
		"Prestador/OptanteSN":               "ABRASF: no PN é regTrib.opSimpNac",
		"Prestador/IncentivadorCultural":    "ABRASF: não existe no PN",
		"Servico/ResponsavelRetencao":       "ABRASF: não existe no PN",
		"IdentificacaoRps/NaturezaOperacao": "ABRASF: não existe no PN",
		"IdentificacaoRps/Status":           "ABRASF: não existe no PN",
		"IdentificacaoRps/Tipo":             "ABRASF: não existe no PN",
		"Valores/IssRetido":                 "ABRASF: no PN é tribMun.tpRetISSQN",

		// Retenções federais: os dois builders enviam o grupo [tribFederal], e o
		// ABRASF as repete dentro de [Valores], que é onde os layouts ABRASF leem.
		"Valores/ValorPis":    "ABRASF: repete em [Valores] o que vai em [tribFederal]",
		"Valores/ValorCofins": "ABRASF: repete em [Valores] o que vai em [tribFederal]",
		"Valores/ValorInss":   "ABRASF: repete em [Valores] o que vai em [tribFederal]",
		"Valores/ValorIr":     "ABRASF: repete em [Valores] o que vai em [tribFederal]",
		"Valores/ValorCsll":   "ABRASF: repete em [Valores] o que vai em [tribFederal]",

		"Prestador/DataOptanteSimplesNacional": "ABRASF: não existe no PN",
		"Prestador/RegimeEspTrib":              "ABRASF: no PN é regTrib.regEspTrib",
		"Prestador/Regime":                     "PN: o regime especial, que no ABRASF é RegimeEspTrib",
		"Prestador/opSimpNac":                  "PN: optante do Simples, que no ABRASF é OptanteSN",
		"IdentificacaoRps/verAplic":            "PN: versão do aplicativo emissor",
		"Servico/xMunicipioIncidencia":         "descritivo; o ABRASF recebe só o código",
	}

	pn := espelho.SecoesEChaves(espelho.Caso{
		Novo:   func() any { return &DPSPedido{} },
		Gerar:  func(a any) string { return ToINI(*a.(*DPSPedido)) },
		Grupos: gruposNFSe,
	})
	abrasf := espelho.SecoesEChaves(espelho.Caso{
		Novo:   func() any { return &DPSPedido{} },
		Gerar:  func(a any) string { return ToINIAbrasf(*a.(*DPSPedido)) },
		Grupos: gruposNFSe,
	})

	var divergentes []string
	for secao := range pn {
		if abrasf[secao] == nil {
			continue // seção que só um layout tem
		}
		for chave := range pn[secao] {
			id := secao + "/" + chave
			if !abrasf[secao][chave] && exclusivos[id] == "" {
				divergentes = append(divergentes, "PN envia, ABRASF não: "+id)
			}
		}
		for chave := range abrasf[secao] {
			id := secao + "/" + chave
			if !pn[secao][chave] && exclusivos[id] == "" {
				divergentes = append(divergentes, "ABRASF envia, PN não: "+id)
			}
		}
	}
	sort.Strings(divergentes)
	if len(divergentes) > 0 {
		t.Errorf(`%d campo(s) tratados por um builder e ignorados pelo outro.

Foi assim que cNBS, cServ e numeroProcesso sumiram silenciosamente. Decida:
emitir nos dois, ou registrar em exclusivos com o motivo.

%s`, len(divergentes), "  "+strings.Join(divergentes, "\n  "))
	}
}

// --- helpers ---------------------------------------------------------------

func carregarBaseline(t *testing.T) map[string]bool {
	t.Helper()
	b, err := lockstep.CarregarBaseline("testdata/nao_enviadas.tsv")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
