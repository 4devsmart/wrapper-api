package mdfe

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Lockstep do MDF-e: compara as chaves de INI que a LIB aceita com as que os
// nossos builders escrevem. É o mesmo mecanismo de internal/nfse e internal/cte,
// que nasceu de uma classe de bug que apareceu quatro vezes no NFS-e: o campo é
// aceito pelo contrato, a emissão "funciona", e o dado some antes do XML.
//
// Duas comparações, em direções opostas:
//
//  1. a lib aceita e NÃO enviamos → lacuna de cobertura (baseline em testdata);
//  2. enviamos e a lib NÃO lê → chave morta: escrevemos no INI e a lib ignora
//     em silêncio. Pior que a primeira, porque parece que funciona.
//
// O snapshot sai do fonte do ACBr, na revisão fixada em docs/ACBRLIB.md, e
// precisa ser refeito a cada bump do .so. Não há script neste repositório:
// refazer exige o fonte, que não vem no clone.
// Falhar aqui não é "conserte o código", é "decida": passar a enviar, ou
// declarar com o motivo.

// chavesMortas são chaves que NÓS escrevemos e o leitor de INI da lib não lê.
// Cada uma é uma decisão registrada, não um bug tolerado.
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

// par normaliza (seção, chave) para a forma comparável: o INI do ACBr é
// case-insensitive (TIniFile), e tanto a lib quanto nós indexamos seções e
// chaves repetidas com sufixo numérico (det001, cInfManu002). Sem normalizar,
// a comparação acusaria "Exped" × "exped" e "EVENTO" × "EVENTO001" como
// divergência: ruído puro.
func par(secao, chave string) string {
	return strings.ToLower(semIndice(secao)) + "/" + strings.ToLower(semIndice(chave))
}

func semIndice(s string) string { return reIndice.ReplaceAllString(s, "") }

var (
	reIndice   = regexp.MustCompile(`[0-9]+$`)
	reSecaoGo  = regexp.MustCompile(`b\.section\("([A-Za-z0-9_]+)"`)
	reChaveGo  = regexp.MustCompile(`b\.kv[A-Za-z]*\("([A-Za-z0-9_]+)"`)
	reHelperGo = regexp.MustCompile(`\bb\.([a-z][A-Za-z0-9_]*)\(`)
	reFuncGo   = regexp.MustCompile(`func \(b \*iniBuilder\) ([A-Za-z0-9_]+)\(`)
)

func TestLockstep_MDFe_ChavesDaLibQueNaoEnviamos(t *testing.T) {
	lib := carregarSnapshot(t, "testdata/lerini_chaves.tsv")
	nosso := chavesDoBuilder(t, ".")
	baseline := carregarBaselineDFe(t, "testdata/nao_enviadas.tsv")

	var novas []string
	for p := range lib {
		if nosso[p] || baseline[p] {
			continue
		}
		novas = append(novas, p)
	}
	if len(novas) == 0 {
		return
	}
	sort.Strings(novas)
	t.Errorf("%d chave(s) NOVAS que a lib aceita e não enviamos.\n\n"+
		"Não são todas as lacunas: só as que apareceram depois do último baseline\n"+
		"(normalmente após um bump do .so). Decida cada uma: emitir no builder, ou\n"+
		"acrescentar a testdata/nao_enviadas.tsv, com o motivo.\n\n  %s",
		len(novas), strings.Join(novas, "\n  "))
}

func TestLockstep_MDFe_ChavesQueEnviamosEALibIgnora(t *testing.T) {
	lib := carregarSnapshot(t, "testdata/lerini_chaves.tsv")
	nosso := chavesDoBuilder(t, ".")

	declaradas := map[string]bool{}
	for k := range chavesMortas {
		sc := strings.SplitN(k, "/", 2)
		declaradas[par(sc[0], sc[1])] = true
	}
	var mortas, ressuscitadas []string
	for p := range nosso {
		if lib[p] {
			if declaradas[p] {
				ressuscitadas = append(ressuscitadas, p)
			}
			continue
		}
		if !declaradas[p] {
			mortas = append(mortas, p)
		}
	}
	if len(mortas) > 0 {
		sort.Strings(mortas)
		t.Errorf("%d chave(s) que ESCREVEMOS e a lib não lê: o dado é aceito e descartado em silêncio.\n"+
			"Corrija o builder (nome/seção errados?) ou declare em chavesMortas com o motivo:\n  %s",
			len(mortas), strings.Join(mortas, "\n  "))
	}
	if len(ressuscitadas) > 0 {
		sort.Strings(ressuscitadas)
		t.Errorf("estas chaves estão em chavesMortas mas a lib passou a lê-las: "+
			"remova-as da lista para travar o ganho:\n  %s", strings.Join(ressuscitadas, "\n  "))
	}
}

// --- helpers ---------------------------------------------------------------

// carregarSnapshot lê o TSV extraído do fonte da biblioteca.
func carregarSnapshot(t *testing.T, arquivo string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatalf("snapshot ausente: %v", err)
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if c := strings.SplitN(l, "\t", 2); len(c) == 2 {
			out[par(c[0], c[1])] = true
		}
	}
	if len(out) == 0 {
		t.Fatalf("snapshot %s vazio: o layout do fonte mudou?", arquivo)
	}
	return out
}

// carregarBaselineDFe lê as lacunas JÁ CONHECIDAS. O teste não cobra o passado:
// cobra o que aparecer depois, que é onde mora o campo esquecido.
func carregarBaselineDFe(t *testing.T, arquivo string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatalf("baseline ausente: %v", err)
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		campo := strings.SplitN(l, "\t", 2)[0]
		if sc := strings.SplitN(campo, "/", 2); len(sc) == 2 {
			out[par(sc[0], sc[1])] = true
		}
	}
	return out
}

// chavesDoBuilder extrai do fonte do pacote as chaves de INI escritas, por seção.
//
// Lê TODOS os arquivos não-teste do diretório, não só o ini.go: no MDF-e os
// eventos (cancelamento, CC-e, EPEC, comprovante, insucesso) montam INI em
// arquivos próprios, e no MDF-e o mesmo vale para eventos e pagamento. Ler só um
// arquivo faria as chaves dos outros parecerem lacuna.
//
// Vários helpers (pessoa, endereco, entrega…) escrevem na seção do CHAMADOR:
// para eles, as chaves são expandidas no ponto da chamada, transitivamente:
// pessoa() chama endereco(), e as duas contribuem para a seção de quem chamou.
func chavesDoBuilder(t *testing.T, dir string) map[string]bool {
	t.Helper()
	arquivos, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("listando %s: %v", dir, err)
	}
	var linhas []string
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		b, err := os.ReadFile(a)
		if err != nil {
			t.Fatalf("lendo %s: %v", a, err)
		}
		// Separador entre arquivos: a seção corrente não vaza de um para o outro.
		linhas = append(linhas, "// ---- "+a)
		linhas = append(linhas, strings.Split(string(b), "\n")...)
	}

	// 1) corpo de cada helper: chaves próprias, helpers chamados, e se abre seção.
	type helper struct {
		chaves    []string
		chamados  []string
		abreSecao bool
	}
	helpers := map[string]*helper{}
	atual := ""
	for _, l := range linhas {
		if m := reFuncGo.FindStringSubmatch(l); m != nil {
			atual = m[1]
			helpers[atual] = &helper{}
			continue
		}
		if atual == "" {
			continue
		}
		h := helpers[atual]
		if reSecaoGo.MatchString(l) {
			h.abreSecao = true
		}
		if m := reChaveGo.FindStringSubmatch(l); m != nil {
			h.chaves = append(h.chaves, m[1])
		}
		for _, m := range reHelperGo.FindAllStringSubmatch(l, -1) {
			h.chamados = append(h.chamados, m[1])
		}
	}

	// 2) expansão transitiva das chaves de um helper sem seção própria.
	var expandir func(nome string, visto map[string]bool) []string
	expandir = func(nome string, visto map[string]bool) []string {
		h, ok := helpers[nome]
		if !ok || visto[nome] || h.abreSecao {
			return nil
		}
		visto[nome] = true
		out := append([]string{}, h.chaves...)
		for _, c := range h.chamados {
			out = append(out, expandir(c, visto)...)
		}
		return out
	}

	// 3) varredura linear, mantendo a seção corrente.
	out := map[string]bool{}
	secao := ""
	for _, l := range linhas {
		// A seção corrente vale DENTRO de uma função: o arquivo tem dezenas
		// delas, e sem zerar aqui as chaves de uma função sem seção própria
		// seriam atribuídas à seção aberta pela função anterior no arquivo:
		// que é ordem de texto, não ordem de execução.
		if strings.HasPrefix(l, "func ") || strings.HasPrefix(l, "// ---- ") {
			secao = ""
		}
		if strings.HasPrefix(l, "// ---- ") {
			continue
		}
		if m := reSecaoGo.FindStringSubmatch(l); m != nil {
			secao = m[1]
			continue
		}
		if secao == "" {
			continue
		}
		if m := reChaveGo.FindStringSubmatch(l); m != nil {
			out[par(secao, m[1])] = true
		}
		for _, m := range reHelperGo.FindAllStringSubmatch(l, -1) {
			for _, k := range expandir(m[1], map[string]bool{}) {
				out[par(secao, k)] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("nenhuma chave extraída de %s: os helpers do builder mudaram de forma?", dir)
	}
	return out
}

// Baseline que acumula linha morta para de significar alguma coisa: o teste
// deixa de cobrar sem ninguém perceber. Foi assim que a lista sobreviveu ao
// conserto das lacunas que ela mesma registrava.
func TestLockstep_MDFE_BaselineSemLinhaMorta(t *testing.T) {
	lib := carregarSnapshot(t, "testdata/lerini_chaves.tsv")
	nosso := chavesDoBuilder(t, ".")
	baseline := carregarBaselineDFe(t, "testdata/nao_enviadas.tsv")

	var mortas []string
	for p := range baseline {
		switch {
		case nosso[p]:
			mortas = append(mortas, p+"\t(passamos a enviar)")
		case !lib[p]:
			mortas = append(mortas, p+"\t(a lib não aceita mais esta chave)")
		}
	}
	if len(mortas) == 0 {
		return
	}
	sort.Strings(mortas)
	t.Errorf("%d linha(s) de testdata/nao_enviadas.tsv que não valem mais.\n"+
		"Remova para travar o ganho, senão a lista vira depósito:\n  %s",
		len(mortas), strings.Join(mortas, "\n  "))
}
