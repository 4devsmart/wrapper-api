// Command gerar-openapi extrai os schemas do corpo dos documentos fiscais a
// partir dos MODELOS Go e os injeta em api/openapi.yaml.
//
// Por que gerar em vez de escrever à mão: são ~270 structs. Uma spec escrita à
// mão vira uma segunda fonte da verdade, e a segunda fonte diverge da primeira
// no primeiro campo que alguém adicionar. Gerando, o contrato publicado é o
// contrato compilado, e `make openapi-check` no CI reprova a divergência.
//
// Usa go/ast (stdlib) em vez de reflexão porque só o código-fonte tem os
// COMENTÁRIOS, e é neles que está o conhecimento caro: "money com vírgula",
// "CodigoPais=1058 é obrigatório", "default 1 = erro na emissão".
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// raiz é um tipo exposto no contrato HTTP e o nome que ele terá na spec.
type raiz struct {
	Pacote, Tipo, Nome string
}

// raizes são os tipos que aparecem no corpo de alguma rota. Ao expor um pedido
// novo, acrescente-o aqui: senão ele fica fora da documentação.
var raizes = []raiz{
	{"cte", "PedidoEmissao", "CteDocumento"},
	{"cte", "PedidoSimp", "CteDocumentoSimplificado"},
	{"cte", "PedidoCancelamento", "CteEventoCancelamento"},
	{"cte", "PedidoCartaCorrecao", "CteEventoCartaCorrecao"},
	{"cte", "PedidoEPEC", "CteEventoEPEC"},
	{"cte", "PedidoComprovanteEntrega", "CteEventoComprovanteEntrega"},
	{"cte", "PedidoInsucessoEntrega", "CteEventoInsucessoEntrega"},
	{"cte", "PedidoDesacordo", "CteEventoPrestacaoDesacordo"},

	{"mdfe", "PedidoEmissao", "MdfeDocumento"},
	{"mdfe", "PedidoEncerramento", "MdfeEventoEncerramento"},
	{"mdfe", "PedidoCancelamento", "MdfeEventoCancelamento"},
	{"mdfe", "PedidoInclusaoCondutor", "MdfeEventoInclusaoCondutor"},
	{"mdfe", "PedidoInclusaoDFe", "MdfeEventoInclusaoDFe"},
	{"mdfe", "PedidoPagamentoOperacao", "MdfeEventoPagamentoOperacao"},

	{"nfse", "DPSPedido", "NfseDocumento"},
	{"nfse", "CancelamentoPedido", "NfseEventoCancelamento"},
	{"nfse", "SubstituicaoPedido", "NfseEventoSubstituicao"},

	{"boleto", "Pedido", "BoletoPedido"},
}

// prefixo do pacote no nome do schema: cte.Ide e mdfe.Ide são estruturas
// DIFERENTES, e achatá-las num nome só produziria uma spec errada.
var prefixo = map[string]string{
	"cte": "Cte", "mdfe": "Mdfe", "nfse": "Nfse", "boleto": "Boleto",
}

const (
	inicioBloco = "    # >>> INÍCIO DOS SCHEMAS GERADOS, não edite à mão (make openapi)"
	fimBloco    = "    # <<< FIM DOS SCHEMAS GERADOS"
)

func main() {
	if err := executar(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar() error {
	pacotes := []string{"cte", "mdfe", "nfse", "boleto"}
	tipos, err := carregarTipos(pacotes)
	if err != nil {
		return err
	}
	enums, err := carregarEnums(pacotes)
	if err != nil {
		return err
	}
	glossario, err := carregarGlossario("internal/fiscal/glossario.tsv")
	if err != nil {
		return err
	}
	grupos, err := carregarGlossario("internal/fiscal/glossario-grupos.tsv")
	if err != nil {
		return err
	}

	g := &gerador{tipos: tipos, enums: enums, glossario: glossario, grupos: grupos, feitos: map[string]bool{}, saida: map[string]string{}, apelidos: map[string]string{}}
	for _, r := range raizes {
		if _, ok := tipos[chave(r.Pacote, r.Tipo)]; !ok {
			return fmt.Errorf("raiz %s.%s não encontrada", r.Pacote, r.Tipo)
		}
		g.apelidos[chave(r.Pacote, r.Tipo)] = r.Nome
	}
	for _, r := range raizes {
		g.emitir(r.Pacote, r.Tipo)
	}

	nomes := make([]string, 0, len(g.saida))
	for n := range g.saida {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)

	var b strings.Builder
	b.WriteString(inicioBloco + "\n")
	for _, n := range nomes {
		b.WriteString(g.saida[n])
	}
	b.WriteString(fimBloco + "\n")

	return substituirBloco("api/openapi.yaml", b.String())
}

// --- carga do fonte ---------------------------------------------------------

type tipoInfo struct {
	pacote string
	spec   *ast.TypeSpec
	doc    string
}

func chave(pacote, tipo string) string { return pacote + "." + tipo }

func carregarTipos(pacotes []string) (map[string]tipoInfo, error) {
	out := map[string]tipoInfo{}
	fset := token.NewFileSet()
	for _, p := range pacotes {
		dir := "internal/" + p
		pkgs, err := parser.ParseDir(fset, dir, semTestes, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		for _, pkg := range pkgs {
			for _, arq := range pkg.Files {
				for _, d := range arq.Decls {
					gd, ok := d.(*ast.GenDecl)
					if !ok || gd.Tok != token.TYPE {
						continue
					}
					for _, s := range gd.Specs {
						ts, ok := s.(*ast.TypeSpec)
						if !ok {
							continue
						}
						doc := semReferenciaInterna(textoDoc(ts.Doc))
						if doc == "" {
							doc = semReferenciaInterna(textoDoc(gd.Doc))
						}
						out[chave(p, ts.Name.Name)] = tipoInfo{pacote: p, spec: ts, doc: doc}
					}
				}
			}
		}
	}
	return out, nil
}

// reEspelha casa "Ide espelha CteSefazIde.", que é referência ao tipo do
// contrato de origem.
var reEspelha = regexp.MustCompile(`(?i)\b\w+ espelha \w+\.?\s*`)

// semReferenciaInterna tira a nota de correspondência com o contrato de origem.
// Ela ajuda quem mantém o modelo e não diz nada a quem integra: saber que Toma3
// espelha CteSefazToma3 não ensina o que preencher.
func semReferenciaInterna(doc string) string {
	return semDetalheDeImplementacao(reEspelha.ReplaceAllString(doc, ""))
}

// Detalhes de COMO montamos o documento, que não pertencem ao contrato público:
// nomes de seção do arquivo intermediário ([Cedente], [TituloN]), o nome da
// biblioteca e as funções nativas. O comentário no código continua com eles,
// porque ali são úteis; o que é publicado, não.
var (
	reSecaoINI  = regexp.MustCompile(`\s*\((?:seção|grupo|chaves)?\s*\[[^\]]+\](?:[^)]*)\)`)
	reSecaoSolt = regexp.MustCompile(`\s*(?:seção|grupo)\s+\[[^\]]+\][^,.;]*`)
	reColchete  = regexp.MustCompile(`\[[A-Za-z][A-Za-z0-9_]*\]`)
	reNativa    = regexp.MustCompile(`\b(?:NFSE|CTE|MDFE|NFE|Boleto)_[A-Za-z]+\b`)
	reACBrAPI   = regexp.MustCompile(`(?i)\s*\(?\b(?:espelha o |contrato )?ACBr\.API\b[^),.;]*\)?`)
	reACBrLib   = regexp.MustCompile(`(?i)\bACBrLib[A-Za-z]*\b|\bACBrNFSeX\b`)
	reACBr      = regexp.MustCompile(`(?i)\bo ACBr\b|\ba ACBr\b|\bACBr\b`)
	reINI       = regexp.MustCompile(`(?i)\bdo INI\b|\bno INI\b|\bo INI\b|\bINI\b`)
	reEspacos   = regexp.MustCompile(`\s{2,}`)
)

func semDetalheDeImplementacao(doc string) string {
	doc = reSecaoINI.ReplaceAllString(doc, "")
	doc = reSecaoSolt.ReplaceAllString(doc, "")
	doc = reNativa.ReplaceAllString(doc, "a biblioteca fiscal")
	doc = reACBrAPI.ReplaceAllString(doc, "")
	doc = reACBrLib.ReplaceAllString(doc, "a biblioteca fiscal")
	doc = reACBr.ReplaceAllString(doc, "a biblioteca fiscal")
	doc = reINI.ReplaceAllString(doc, "o documento")
	doc = reColchete.ReplaceAllString(doc, "")
	doc = strings.NewReplacer(" ,", ",", " .", ".", " :", ":", " ;", ";", "()", "", "( )", "").Replace(doc)
	doc = reEspacos.ReplaceAllString(doc, " ")
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(doc), ",;:-"))
}

func semTestes(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }

// opcao é um valor aceito por um campo, com o rótulo do layout fiscal.
type opcao struct{ Valor, Rotulo string }

// carregarEnums lê os internal/<pacote>/enums.tsv. Os valores vêm das tabelas de
// conversão da ACBrLib (é o que a lib aceita de fato) e os rótulos, do layout do
// documento. Ficam em TSV, e não em Go, porque são DADO: assim o gerador os lê
// direto e não há uma cópia em código para divergir da tabela.
func carregarEnums(pacotes []string) (map[string][]opcao, error) {
	out := map[string][]opcao{}
	for _, p := range pacotes {
		b, err := os.ReadFile("internal/" + p + "/enums.tsv")
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimRight(l, "\r")
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			c := strings.Split(l, "\t")
			if len(c) < 3 {
				return nil, fmt.Errorf("internal/%s/enums.tsv: linha inválida: %q", p, l)
			}
			k := chave(p, c[0])
			out[k] = append(out[k], opcao{Valor: c[1], Rotulo: c[2]})
		}
	}
	return out, nil
}

// carregarGlossario lê internal/fiscal/glossario.tsv: a descrição padrão dos
// nomes de campo do layout fiscal.
//
// Existe porque os mesmos nomes se repetem nos três documentos (CNPJ, xNome,
// cMun, vBC aparecem dezenas de vezes) e comentá-los um a um seria copiar a
// mesma frase em dezenas de lugares, para desatualizar em alguns deles. Um
// comentário no modelo sempre vence o glossário: ele é específico do contexto.
func carregarGlossario(arquivo string) (map[string]string, error) {
	out := map[string]string{}
	b, err := os.ReadFile(arquivo)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimRight(l, "\r")
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		c := strings.SplitN(l, "\t", 2)
		if len(c) != 2 {
			return nil, fmt.Errorf("%s: linha inválida: %q", arquivo, l)
		}
		out[c[0]] = c[1]
	}
	return out, nil
}

// formatos descreve o que a API espera em campos de data e hora. É o detalhe que
// mais custa tempo numa integração, e não está no tipo: "string" não diz nada.
var formatos = map[string]struct{ Formato, Descricao string }{
	"data": {"date", "Data no formato AAAA-MM-DD. Também aceita DD/MM/AAAA e " +
		"RFC 3339, caso em que a hora é descartada."},
	"data-hora": {"date-time", "Data e hora. COM offset (2026-08-23T14:05:00-03:00) " +
		"a entrada é um instante e é convertida para o fuso do documento; SEM offset " +
		"(2026-08-23T14:05:00) é lida como relógio de parede do emitente e não sofre " +
		"conversão. Vazio usa o momento da chamada."},
}

// --- geração ----------------------------------------------------------------

type gerador struct {
	tipos     map[string]tipoInfo
	enums     map[string][]opcao
	glossario map[string]string
	grupos    map[string]string
	feitos    map[string]bool
	saida     map[string]string
	apelidos  map[string]string
}

func (g *gerador) nome(pacote, tipo string) string {
	if a, ok := g.apelidos[chave(pacote, tipo)]; ok {
		return a
	}
	return prefixo[pacote] + tipo
}

func (g *gerador) emitir(pacote, tipo string) {
	k := chave(pacote, tipo)
	if g.feitos[k] {
		return
	}
	g.feitos[k] = true

	info, ok := g.tipos[k]
	if !ok {
		return
	}
	st, ok := info.spec.Type.(*ast.StructType)
	if !ok {
		return // alias/enum: tratado inline por tipoDe
	}

	nome := g.nome(pacote, tipo)
	doc := info.doc
	if doc == "" {
		// O glossário de grupos é indexado pelo nome do TIPO, sem o prefixo do
		// documento: "Ide" descreve CteIde e MdfeIde, que são o mesmo conceito.
		doc = g.grupos[tipo]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "    %s:\n      type: object\n", nome)
	if doc != "" {
		fmt.Fprintf(&b, "      description: %s\n", yamlStr(doc))
	}
	fmt.Fprintf(&b, "      properties:\n")

	vazio := true
	for _, f := range st.Fields.List {
		jsonNome, omitir := nomeJSON(f)
		if omitir {
			continue
		}
		if len(f.Names) == 0 { // struct embutida: achata os campos dela
			if id, ok := f.Type.(*ast.Ident); ok {
				g.achatar(&b, pacote, id.Name)
				vazio = false
			}
			continue
		}
		vazio = false
		fmt.Fprintf(&b, "        %s:\n", jsonNome)
		g.escreverCampo(&b, pacote, jsonNome, f, "          ")
	}
	if vazio {
		// properties: {} vazio é YAML inválido no contexto; usa objeto livre.
		b.Reset()
		fmt.Fprintf(&b, "    %s:\n      type: object\n      additionalProperties: true\n", nome)
	}
	g.saida[nome] = b.String()
}

// achatar copia as propriedades de uma struct embutida (o JSON as promove ao
// nível de cima, e a spec precisa refletir isso).
func (g *gerador) achatar(b *strings.Builder, pacote, tipo string) {
	info, ok := g.tipos[chave(pacote, tipo)]
	if !ok {
		return
	}
	st, ok := info.spec.Type.(*ast.StructType)
	if !ok {
		return
	}
	for _, f := range st.Fields.List {
		jsonNome, omitir := nomeJSON(f)
		if omitir || len(f.Names) == 0 {
			continue
		}
		fmt.Fprintf(b, "        %s:\n", jsonNome)
		g.escreverCampo(b, pacote, jsonNome, f, "          ")
	}
}

// escreverCampo emite o tipo do campo, os valores aceitos e a descrição.
//
// Quando o tipo é uma referência, embrulha em allOf: $ref com irmãos é legal em
// OpenAPI 3.1, mas parte das ferramentas (inclusive versões do Swagger UI)
// descarta o irmão, e a descrição sumiria justamente nos campos que apontam
// para outra estrutura.
func (g *gerador) escreverCampo(b *strings.Builder, pacote, jsonNome string, f *ast.Field, ind string) {
	desc := semDetalheDeImplementacao(descricaoCampo(f))
	if desc == "" {
		desc = g.glossario[jsonNome]
	}
	opcoes := g.enums[chave(pacote, valorTagCampo(f, "enum"))]
	if len(opcoes) > 0 {
		desc = strings.TrimSpace(semListaDeValores(desc) + " " + descricaoDasOpcoes(opcoes))
	}
	if fo, ok := formatos[valorTagCampo(f, "fmt")]; ok {
		desc = strings.TrimSpace(semDicaDeFormato(desc) + " " + fo.Descricao)
	}

	var tipo strings.Builder
	g.escreverTipo(&tipo, pacote, f.Type, ind+"    ")
	if ref := strings.TrimSpace(tipo.String()); strings.HasPrefix(ref, "$ref:") {
		if desc != "" {
			fmt.Fprintf(b, "%sdescription: %s\n", ind, yamlStr(desc))
		}
		fmt.Fprintf(b, "%sallOf:\n%s  - %s\n", ind, ind, ref)
		return
	}

	g.escreverTipo(b, pacote, f.Type, ind)
	if fo, ok := formatos[valorTagCampo(f, "fmt")]; ok {
		fmt.Fprintf(b, "%sformat: %s\n", ind, fo.Formato)
	}
	if len(opcoes) > 0 {
		// enum na spec faz o Swagger mostrar um seletor em vez de campo livre, e
		// permite que gerador de SDK crie o tipo. O rótulo vai na descrição
		// porque o enum do OpenAPI carrega só o valor.
		//
		// Em campo de lista quem tem os valores é o item, não o array: o enum
		// desce um nível, para dentro de items.
		indEnum, alvo := ind, semPonteiro(f.Type)
		if arr, ok := alvo.(*ast.ArrayType); ok && !ehListaDeBytes(arr) {
			indEnum, alvo = ind+"  ", arr.Elt
		}
		fmt.Fprintf(b, "%senum: [%s]\n", indEnum, valoresYAML(opcoes, ehNumerico(alvo)))
	}
	if desc != "" {
		fmt.Fprintf(b, "%sdescription: %s\n", ind, yamlStr(desc))
	}
}

// reListaDeValores casa a enumeração informal que muitos comentários trazem
// ("1=não optante, 2=MEI, 3=ME/EPP"), inclusive quando ela é o comentário todo.
var reListaDeValores = regexp.MustCompile(`\(?"?\b\d+"?\s*=\s*[^;.]*(?:,\s*"?\d+"?\s*=\s*[^;.]*)*\)?`)

// semListaDeValores tira do comentário a lista de valores escrita à mão. Com o
// enum na spec ela vira repetição, e o campo passaria a exibir a mesma
// informação duas vezes, uma delas desatualizável.
func semListaDeValores(desc string) string {
	desc = reListaDeValores.ReplaceAllString(desc, "")
	desc = strings.NewReplacer(" ,", ",", " .", ".", "..", ".", "  ", " ").Replace(desc)
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(desc), ",;:-"))
}

// reDicaDeFormato casa a dica de formato escrita à mão no comentário, como
// "(YYYY-MM-DD)" ou "; ISO".
var reDicaDeFormato = regexp.MustCompile(`\(?\b(?i:[AY]{4}-MM-DD|DD/MM/[AY]{4}|ISO ?8601|ISO|RFC ?3339)\b[^);.]*\)?`)

// semDicaDeFormato tira do comentário a dica de formato: com o campo `format` e
// a descrição canônica na spec, ela vira repetição, e repetição desatualiza.
func semDicaDeFormato(desc string) string {
	desc = reDicaDeFormato.ReplaceAllString(desc, "")
	desc = strings.NewReplacer(" ,", ",", " ;", ";", "  ", " ", "()", "").Replace(desc)
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(desc), ",;:-"))
}

// descricaoDasOpcoes rende os valores legíveis: "1 = Produção; 2 = Homologação".
func descricaoDasOpcoes(opcoes []opcao) string {
	partes := make([]string, 0, len(opcoes))
	for _, o := range opcoes {
		partes = append(partes, o.Valor+" = "+o.Rotulo)
	}
	txt := "Valores: " + strings.Join(partes, "; ")
	if strings.HasSuffix(txt, ".") {
		return txt
	}
	return txt + "."
}

func valoresYAML(opcoes []opcao, numerico bool) string {
	partes := make([]string, 0, len(opcoes))
	for _, o := range opcoes {
		if numerico {
			partes = append(partes, strings.TrimLeft(o.Valor, "0")+"")
			if strings.TrimLeft(o.Valor, "0") == "" {
				partes[len(partes)-1] = "0"
			}
			continue
		}
		partes = append(partes, `"`+o.Valor+`"`)
	}
	return strings.Join(partes, ", ")
}

// semPonteiro desembrulha *T, que na spec é o mesmo tipo (a diferença é só se
// o campo pode ser omitido).
func semPonteiro(t ast.Expr) ast.Expr {
	if p, ok := t.(*ast.StarExpr); ok {
		return semPonteiro(p.X)
	}
	return t
}

// ehListaDeBytes reconhece o []byte, que na spec vira string base64 e não array.
func ehListaDeBytes(a *ast.ArrayType) bool {
	id, ok := a.Elt.(*ast.Ident)
	return ok && id.Name == "byte"
}

// ehNumerico decide se o enum sai como número ou string na spec: precisa casar
// com o tipo do campo, senão a validação do cliente rejeita o próprio exemplo.
func ehNumerico(t ast.Expr) bool {
	if p, ok := t.(*ast.StarExpr); ok {
		return ehNumerico(p.X)
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return false
	}
	switch id.Name {
	case "int", "int8", "int16", "int32", "int64", "float32", "float64":
		return true
	}
	return false
}

// valorTagCampo lê uma tag arbitrária do campo (enum, fmt).
func valorTagCampo(f *ast.Field, chave string) string {
	if f.Tag == nil {
		return ""
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return ""
	}
	return valorTag(tag, chave)
}

func (g *gerador) escreverTipo(b *strings.Builder, pacote string, t ast.Expr, ind string) {
	switch v := t.(type) {
	case *ast.StarExpr:
		g.escreverTipo(b, pacote, v.X, ind)
	case *ast.ArrayType:
		if id, ok := v.Elt.(*ast.Ident); ok && id.Name == "byte" {
			fmt.Fprintf(b, "%stype: string\n%sformat: byte\n", ind, ind)
			return
		}
		fmt.Fprintf(b, "%stype: array\n%sitems:\n", ind, ind)
		g.escreverTipo(b, pacote, v.Elt, ind+"  ")
	case *ast.MapType:
		fmt.Fprintf(b, "%stype: object\n%sadditionalProperties: true\n", ind, ind)
	case *ast.SelectorExpr: // json.RawMessage e afins
		fmt.Fprintf(b, "%stype: object\n%sadditionalProperties: true\n", ind, ind)
	case *ast.Ident:
		switch v.Name {
		case "string":
			fmt.Fprintf(b, "%stype: string\n", ind)
		case "bool":
			fmt.Fprintf(b, "%stype: boolean\n", ind)
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
			fmt.Fprintf(b, "%stype: integer\n", ind)
		case "float32", "float64":
			fmt.Fprintf(b, "%stype: number\n", ind)
		default:
			if info, ok := g.tipos[chave(pacote, v.Name)]; ok {
				if _, ehStruct := info.spec.Type.(*ast.StructType); ehStruct {
					fmt.Fprintf(b, "%s$ref: \"#/components/schemas/%s\"\n", ind, g.nome(pacote, v.Name))
					g.emitir(pacote, v.Name)
					return
				}
				// tipo nomeado sobre primitivo (ex.: type Familia string)
				g.escreverTipo(b, pacote, info.spec.Type, ind)
				return
			}
			fmt.Fprintf(b, "%stype: object\n%sadditionalProperties: true\n", ind, ind)
		}
	default:
		fmt.Fprintf(b, "%stype: object\n%sadditionalProperties: true\n", ind, ind)
	}
}

// --- extração de nomes e comentários ----------------------------------------

func nomeJSON(f *ast.Field) (nome string, omitir bool) {
	if f.Tag != nil {
		tag, err := strconv.Unquote(f.Tag.Value)
		if err == nil {
			if j := valorTag(tag, "json"); j != "" {
				n, _, _ := strings.Cut(j, ",")
				if n == "-" {
					return "", true
				}
				if n != "" {
					return n, false
				}
			}
		}
	}
	if len(f.Names) == 0 {
		return "", false // embutida: tratada por achatar
	}
	if !f.Names[0].IsExported() {
		return "", true
	}
	return f.Names[0].Name, false
}

func valorTag(tag, chave string) string {
	for _, parte := range strings.Fields(tag) {
		if v, ok := strings.CutPrefix(parte, chave+":"); ok {
			s, err := strconv.Unquote(v)
			if err != nil {
				return strings.Trim(v, `"`)
			}
			return s
		}
	}
	return ""
}

// descricaoCampo prefere o comentário ACIMA do campo; na falta, o da direita.
// É onde mora o conhecimento que não está no tipo.
func descricaoCampo(f *ast.Field) string {
	if d := textoDoc(f.Doc); d != "" {
		return d
	}
	return textoDoc(f.Comment)
}

func textoDoc(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	var partes []string
	for _, c := range g.List {
		t := strings.TrimPrefix(c.Text, "//")
		t = strings.TrimPrefix(t, "/*")
		t = strings.TrimSuffix(t, "*/")
		if t = strings.TrimSpace(t); t != "" {
			partes = append(partes, t)
		}
	}
	return strings.Join(partes, " ")
}

// yamlStr emite um escalar YAML seguro. Aspas simples com ” duplicado cobre
// dois-pontos, aspas duplas e acentos sem escapar nada.
func yamlStr(s string) string {
	s = strings.Join(strings.Fields(s), " ") // achata quebras de linha
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// --- escrita ----------------------------------------------------------------

func substituirBloco(arquivo, bloco string) error {
	b, err := os.ReadFile(arquivo)
	if err != nil {
		return err
	}
	texto := string(b)
	i := strings.Index(texto, inicioBloco)
	j := strings.Index(texto, fimBloco)
	if i < 0 || j < 0 {
		return fmt.Errorf("marcadores do bloco gerado não encontrados em %s", arquivo)
	}
	novo := texto[:i] + bloco + texto[j+len(fimBloco)+1:]
	if novo == texto {
		fmt.Println("openapi.yaml já está atualizado")
		return nil
	}
	fmt.Printf("openapi.yaml atualizado (%d bytes de schemas)\n", len(bloco))
	return os.WriteFile(arquivo, []byte(novo), 0o644)
}
