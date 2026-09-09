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
	"Prestador/xMunicipio":               "descritivo devolvido pelo provedor",
	"Tomador/xMunicipio":                 "descritivo devolvido pelo provedor",
	"Intermediario/xMunicipio":           "descritivo devolvido pelo provedor",

	// Prefixo do logradouro ("Rua", "Avenida"). O contrato: espelhando a Nuvem
	// Fiscal: carrega o logradouro inteiro num campo só (xLgr), então não há o
	// que separar. Prestador/ e Tomador/ já estavam no baseline; Servico/ apareceu
	// no bump para r47859, quando o endereço do local da prestação ganhou seção
	// própria no LerIni.
	"Servico/TipoLogradouro": "o contrato usa um logradouro único (xLgr), sem prefixo separado",

	// Endereço do TOMADOR no exterior: coberto por CodigoPais/xPais.
	"Tomador/DocEstrangeiro":  "tomador no exterior: fora do escopo atual",
	"Tomador/NIF":             "tomador no exterior: fora do escopo atual",
	"Tomador/cNaoNIF":         "tomador no exterior: fora do escopo atual",
	"Tomador/TomadorExterior": "derivado de CodigoPais; default da lib já é 'não'",
	"Prestador/NIF":           "prestador é sempre nacional neste produto",
	"Prestador/cNaoNIF":       "prestador é sempre nacional neste produto",
	"Intermediario/NIF":       "intermediário no exterior: fora do escopo",
	"Intermediario/cNaoNIF":   "intermediário no exterior: fora do escopo",
}

// gruposNaoSuportados são seções inteiras que o contrato não cobre. Aparecem no
// snapshot da lib mas não são lacuna: são escopo que decidimos não atender.
var gruposNaoSuportados = map[string]string{
	"ConstrucaoCivil":            "obra/ART, sem demanda até agora",
	"Rodoviaria":                 "pedágio/vale, sem demanda",
	"LocacaoSubLocacao":          "locação de postes/dutos, sem demanda",
	"IdentificacaoNFSe":          "campos de RESPOSTA (cStat, dhProc, verAplic…)",
	"RpsSubstituido":             "RPS substituído do ABRASF; a substituição do PN é por chave",
	"NFSeCancelamento":           "campos de RESPOSTA do cancelamento",
	"ValoresNFSe":                "valores de RESPOSTA",
	"Emitente":                   "config de emitente vai por ConfigGravarValor",
	"DeclaracaoPrestacaoServico": "agregador do layout, sem chaves nossas",

	// Apareceram quando o snapshot passou a ser GERADO do fonte: antes a
	// direção 1 só olhava seis seções, então estas nunca foram comparadas.
	// Todas são layout de provedor específico, fora do contrato.
	"CondicaoPagamento":        "parcelamento por provedor, fora do contrato",
	"Parcelas":                 "idem, as parcelas em si",
	"Deducoes":                 "dedução por documento referenciado, sem demanda",
	"DocumentosDeducaoReducao": "idem, os documentos da dedução",
	"Despesas":                 "despesas reembolsáveis, sem demanda",
	"Quartos":                  "hotelaria (diárias por quarto), sem demanda",
	"Genericos":                "campos livres de provedor, sem contrato possível",
	"Fornecedor":               "fornecedor do item, layout de provedor",
	"Transportadora":           "transportadora, layout de provedor",
	"Impostos":                 "quebra de imposto por item, layout de provedor",
	"OrgaoGerador":             "órgão gerador, preenchido pelo provedor",
	"EnderecoServico":          "endereço do local da prestação por ITEM",
	"Email":                    "lista de e-mails do provedor",
	"Itens":                    "detalhe por item do RPS: o contrato tem um serviço só",
	"IBSCBSNFSE":               "grupo IBS/CBS de RESPOSTA",
	"IBSCBSValoresNFSE":        "idem, os valores",
	"TotCIBS":                  "totais IBS/CBS de RESPOSTA",
	"TotgCBS":                  "idem",
	"TotgIBS":                  "idem",
	"gRefNFSe":                 "referência a NFS-e anterior, sem demanda",
	"gTribCompraGov":           "compra governamental, sem demanda",
	"gTribRegularNFSe":         "tributação regular de RESPOSTA",

	// Seções do leitor de PEDIDOS (ACBrNFSeXWebserviceBase.pas), que entrou no
	// snapshot junto com [CancelarNFSe] e [Evento].
	"ConsultarNFSe":     "as consultas não vão por INI: cada uma tem função própria na lib, com parâmetros posicionais",
	"ConsultarLinkNFSe": "idem; o link da NFS-e não é exposto por este produto",

	// [Evento] existe nos DOIS leitores com o mesmo nome: a atividade de evento
	// da nota (show, feira), que o contrato não cobre, e o pedido de registro de
	// evento, que ESCREVEMOS (ToINIEvento). Como o snapshot funde as duas, esta
	// direção não consegue separá-las e fica de fora; a direção de chave morta
	// continua cobrindo o que escrevemos.
	"Evento": "atividade de evento (sem demanda) colide com o pedido de registro de evento",
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

		// Retenções federais: o PN as envia na seção própria [tribFed]; no ABRASF
		// elas ficam dentro de [Valores]. Mesmo dado, seções diferentes.
		"Valores/ValorPis":    "PN envia em [tribFed]",
		"Valores/ValorCofins": "PN envia em [tribFed]",
		"Valores/ValorInss":   "PN envia em [tribFed]",
		"Valores/ValorIr":     "PN envia em [tribFed]",
		"Valores/ValorCsll":   "PN envia em [tribFed]",

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
