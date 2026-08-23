package nfse

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Este arquivo é a rede contra a classe de bug que já apareceu três vezes
// (CodigoPais, CodigoNBS, CodigoServicoNacional/NumeroProcesso): um campo que a
// lib aceita, ou que um builder envia, e o outro caminho descarta em silêncio.
// O cliente manda, a API aceita, e o dado some antes do XML.
//
// São duas comparações:
//
//  1. chaves que a LIB aceita × chaves que ENVIAMOS (snapshot em testdata,
//     refeito a cada bump do .so, a partir do fonte do ACBr);
//  2. campos do contrato consumidos por UM builder e ignorados pelo OUTRO.
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
	"Evento":                     "atividade de evento, sem demanda",
	"Rodoviaria":                 "pedágio/vale, sem demanda",
	"LocacaoSubLocacao":          "locação de postes/dutos, sem demanda",
	"IdentificacaoNFSe":          "campos de RESPOSTA (cStat, dhProc, verAplic…)",
	"NFSeSubstituicao":           "substituição usa endpoint próprio",
	"RpsSubstituido":             "idem",
	"NFSeCancelamento":           "cancelamento tem INI próprio",
	"ValoresNFSe":                "valores de RESPOSTA",
	"Emitente":                   "config de emitente vai por ConfigGravarValor",
	"DeclaracaoPrestacaoServico": "agregador do layout, sem chaves nossas",
}

// secoesQueEnviamos são as seções que nossos builders realmente escrevem.
var secoesQueEnviamos = []string{"IdentificacaoRps", "Prestador", "Tomador", "Intermediario", "Servico", "Valores"}

func TestLockstep_ChavesDaLibQueNaoEnviamos(t *testing.T) {
	lib := carregarSnapshot(t)
	nossas := chavesDosBuilders(t)

	baseline := carregarBaseline(t)
	var novas []string
	for _, secao := range secoesQueEnviamos {
		for chave := range lib[secao] {
			id := secao + "/" + chave
			if nossas[secao][chave] || naoEnviadas[id] != "" || baseline[id] {
				continue
			}
			novas = append(novas, id)
		}
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
		"IdentificacaoRps/verAplic":            "PN: versão do aplicativo emissor",
		"Servico/xMunicipioIncidencia":         "descritivo; o ABRASF recebe só o código",
	}

	pn := chavesDoArquivo(t, "ini.go")
	abrasf := chavesDoArquivo(t, "ini_abrasf.go")
	// pessoaCommon vive no ini.go e serve aos dois builders.
	for _, s := range []string{"Prestador", "Tomador", "Intermediario"} {
		if abrasf[s] == nil {
			continue
		}
		for k := range pn[s] {
			abrasf[s][k] = true
		}
	}

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

// carregarBaseline lê as lacunas JÁ CONHECIDAS. O teste não cobra o passado:
// cobra o que aparecer depois, que é onde mora o campo silenciosamente
// descartado. Regenerar exige revisão humana: cada linha nova é uma decisão.
func carregarBaseline(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile("testdata/nao_enviadas.tsv")
	if err != nil {
		t.Fatalf("baseline ausente: %v", err)
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out[strings.SplitN(l, "\t", 2)[0]] = true
	}
	return out
}

func carregarSnapshot(t *testing.T) map[string]map[string]bool {
	t.Helper()
	f, err := os.Open("testdata/lerini_chaves.tsv")
	if err != nil {
		t.Fatalf("snapshot ausente: %v", err)
	}
	defer func() { _ = f.Close() }()

	out := map[string]map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linha := sc.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}
		partes := strings.SplitN(linha, "\t", 2)
		if len(partes) != 2 {
			continue
		}
		secao := regexp.MustCompile(`\d+$`).ReplaceAllString(partes[0], "") // Itens001 → Itens
		if gruposNaoSuportados[secao] != "" {
			continue
		}
		if out[secao] == nil {
			out[secao] = map[string]bool{}
		}
		out[secao][partes[1]] = true
	}
	return out
}

var (
	reSecao  = regexp.MustCompile(`b\.section\("([^"]+)"\)`)
	reChave  = regexp.MustCompile(`b\.kv\w*\("([^"]+)"`)
	rePessoa = regexp.MustCompile(`b\.pessoaCommon\(`)
)

// chavesDoArquivo extrai, do fonte de um builder, as chaves escritas por seção.
func chavesDoArquivo(t *testing.T, arquivo string) map[string]map[string]bool {
	t.Helper()
	b, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatalf("lendo %s: %v", arquivo, err)
	}
	txt := string(b)

	// chaves do helper compartilhado de pessoa
	pessoa := map[string]bool{}
	if i := strings.Index(txt, "func (b *iniBuilder) pessoaCommon"); i >= 0 {
		for _, m := range reChave.FindAllStringSubmatch(txt[i:], -1) {
			pessoa[m[1]] = true
		}
	}

	out := map[string]map[string]bool{}
	secao := ""
	for _, linha := range strings.Split(txt, "\n") {
		if m := reSecao.FindStringSubmatch(linha); m != nil {
			secao = m[1]
			if out[secao] == nil {
				out[secao] = map[string]bool{}
			}
			continue
		}
		if secao == "" {
			continue
		}
		if rePessoa.MatchString(linha) {
			for k := range pessoa {
				out[secao][k] = true
			}
		}
		if m := reChave.FindStringSubmatch(linha); m != nil {
			out[secao][m[1]] = true
		}
	}
	return out
}

// chavesDosBuilders é a união do que os dois builders enviam.
func chavesDosBuilders(t *testing.T) map[string]map[string]bool {
	t.Helper()
	uniao := map[string]map[string]bool{}
	for _, arq := range []string{"ini.go", "ini_abrasf.go"} {
		for secao, chaves := range chavesDoArquivo(t, arq) {
			if uniao[secao] == nil {
				uniao[secao] = map[string]bool{}
			}
			for k := range chaves {
				uniao[secao][k] = true
			}
		}
	}
	// pessoaCommon (ini.go) vale para as três seções de pessoa nos dois builders.
	pessoa := chavesDoArquivo(t, "ini.go")
	for _, s := range []string{"Prestador", "Tomador", "Intermediario"} {
		if uniao[s] == nil {
			uniao[s] = map[string]bool{}
		}
		for k := range pessoa[s] {
			uniao[s][k] = true
		}
	}
	return uniao
}
