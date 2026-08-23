// Command gerar-openapi extrai os schemas do corpo dos documentos fiscais a
// partir dos MODELOS Go e os injeta em api/openapi.yaml.
//
// Por que gerar em vez de escrever à mão: são ~270 structs. Uma spec escrita à
// mão vira uma segunda fonte da verdade, e a segunda fonte diverge da primeira
// no primeiro campo que alguém adicionar. Gerando, o contrato publicado é o
// contrato compilado — e `make openapi-check` no CI reprova a divergência.
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
	"sort"
	"strconv"
	"strings"
)

// raiz é um tipo exposto no contrato HTTP e o nome que ele terá na spec.
type raiz struct {
	Pacote, Tipo, Nome string
}

// raizes são os tipos que aparecem no corpo de alguma rota. Ao expor um pedido
// novo, acrescente-o aqui — senão ele fica fora da documentação.
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
	inicioBloco = "    # >>> INÍCIO DOS SCHEMAS GERADOS — não edite à mão (make openapi)"
	fimBloco    = "    # <<< FIM DOS SCHEMAS GERADOS"
)

func main() {
	if err := executar(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar() error {
	tipos, err := carregarTipos([]string{"cte", "mdfe", "nfse", "boleto"})
	if err != nil {
		return err
	}

	g := &gerador{tipos: tipos, feitos: map[string]bool{}, saida: map[string]string{}, apelidos: map[string]string{}}
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
						doc := textoDoc(ts.Doc)
						if doc == "" {
							doc = textoDoc(gd.Doc)
						}
						out[chave(p, ts.Name.Name)] = tipoInfo{pacote: p, spec: ts, doc: doc}
					}
				}
			}
		}
	}
	return out, nil
}

func semTestes(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }

// --- geração ----------------------------------------------------------------

type gerador struct {
	tipos    map[string]tipoInfo
	feitos   map[string]bool
	saida    map[string]string
	apelidos map[string]string
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
	var b strings.Builder
	fmt.Fprintf(&b, "    %s:\n      type: object\n", nome)
	if info.doc != "" {
		fmt.Fprintf(&b, "      description: %s\n", yamlStr(info.doc))
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
		g.escreverCampo(&b, pacote, f.Type, descricaoCampo(f), "          ")
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
		g.escreverCampo(b, pacote, f.Type, descricaoCampo(f), "          ")
	}
}

// escreverCampo emite o tipo do campo mais a descrição.
//
// Quando o tipo é uma referência, embrulha em allOf: $ref com irmãos é legal em
// OpenAPI 3.1, mas parte das ferramentas (inclusive versões do Swagger UI)
// simplesmente descarta o irmão — e a descrição, que é o valor deste gerador,
// sumiria justamente nos campos que apontam para outra estrutura.
func (g *gerador) escreverCampo(b *strings.Builder, pacote string, t ast.Expr, desc, ind string) {
	if desc == "" {
		g.escreverTipo(b, pacote, t, ind)
		return
	}
	var tipo strings.Builder
	g.escreverTipo(&tipo, pacote, t, ind+"    ")
	if ref := strings.TrimSpace(tipo.String()); strings.HasPrefix(ref, "$ref:") {
		fmt.Fprintf(b, "%sdescription: %s\n%sallOf:\n%s  - %s\n", ind, yamlStr(desc), ind, ind, ref)
		return
	}
	g.escreverTipo(b, pacote, t, ind)
	fmt.Fprintf(b, "%sdescription: %s\n", ind, yamlStr(desc))
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
