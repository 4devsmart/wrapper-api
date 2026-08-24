# Contribuindo

Obrigado pelo interesse. Este documento é curto de propósito, o que importa
está em dois lugares: as armadilhas abaixo e as
[limitações](docs/LIMITACOES.md).

## Rodando

```bash
make test        # unidade, sem lib nativa e sem certificado
make acbr-libs-baixar && make up   # o stack de verdade
```

As libs nativas não estão no repositório: vêm de anexos de release, e
[docs/ACBRLIB.md](docs/ACBRLIB.md) explica o ciclo inteiro, de como consumir a
como publicar uma revisão nova.

O domínio inteiro é testável **sem a lib nativa e sem certificado**. Isso não é
sorte: a geração não assina, e é justamente o que torna a camada de tradução
JSON para INI, o ativo real do projeto, verificável por quem não tem um A1 na
mão.

## Armadilhas que já custaram tempo

**O build da lib nativa é TUDO no Docker.** Não compile o `.so` no host. Para
validar o binding cgo, use `make test-cgo` (type-check + link contra as `.so`) ou
rebuilde a imagem: nunca `go build -tags acbrlib` avulso.

**Método novo na interface `Servico` = 6 lugares.** Interface, `acbrlib.h`, o
binding cgo, o stub `indisponivel`, o cliente `remoto` e o despacho em
`rpcserver.go`. O teste `TestDespachoCobreTodasAsInterfaces` pega o esquecimento
no despacho, mas não os outros: confira os seis.

**Confira a assinatura C em DUAS fontes oficiais.** O
`ACBrLib*StaticImportMT.pas` do próprio ACBr já esteve desatualizado em relação
ao `ACBrLib*MT.pas` que ele espelha: foi assim que `CTE_Enviar` ficou com um
argumento a menos no projeto de origem, o que desloca os registradores no SysV
AMD64 e vira corrupção de memória. Cruze com os imports de PHP (`.h`), C# ou VB6
nos `Demos/`.

**O `.so` é GTK2 e exige Xvfb.** Ele chama `gtk_init` no carregamento, mesmo para
operações que não imprimem. A variante `nogui` não serve.

**Nunca commite** `.env`, `.so`, certificados, XMLs ou logs reais. O CI tem um
gate para isso, mas ele é a última linha, não a primeira. O log da ACBr em nível
3+ grava XML **e certificado**, se ligar para depurar, apague depois.

**Fixtures são sintéticas.** Nunca use XML, CNPJ ou certificado reais em teste.

**Tipo que carrega segredo redige a si mesmo**, com `String()` **e**
`LogValue()`. Os dois: o `fmt` usa o primeiro, e o `slog` usa o segundo. O
manipulador JSON do `slog`, que é o do serviço, não consulta `Stringer` e
serializa a struct campo a campo, então só com `String()` a senha sai inteira no
log. Um teste varre o repositório e recusa o tipo novo sem redação.

## O que o código já decidiu

Antes de propor uma mudança estrutural, saiba o que está fechado e por quê:

- **Nada é persistido.** O certificado vem no payload e morre com a requisição.
- **Gerar e transmitir são chamadas separadas.** A geração põe o documento nas
  mãos do cliente antes de qualquer
  byte sair; é isso que torna recuperável uma transmissão perdida. Não há atalho
  de uma chamada só.
- **Um módulo não importa outro módulo**, não conhece o servidor, e `platform/`
  não conhece domínio fiscal. Verificado por teste sobre o grafo de imports.
- **Campo desconhecido no JSON é 400.** Campo ignorado em silêncio vira documento
  transmitido com dado faltando.
- **Nada é reenviado automaticamente.** Retry cego duplica documento fiscal.

Discordar é legítimo: abra uma issue com o caso concreto. O que não ajuda é um
PR que contorna a regra sem tocar no raciocínio dela.

## Onde o código compartilhado mora

Os quatro documentos têm a mesma forma e conteúdos diferentes, e a tentação é
copiar. Já custou caro: a escolha entre CNPJ e CPF existia em três versões, e
uma delas aceitava um CNPJ em branco que as outras descartavam.

| Camada | O que mora lá |
|---|---|
| `internal/platform/inifmt` | **como** escrever o arquivo intermediário: seção, par chave=valor, moeda, largura de índice, data no fuso do emitente |
| `internal/fiscal` | o que é comum ao FISCO e não a um documento: certificado, ambiente, chave, erro da biblioteca virando HTTP |
| `internal/platform/httpx` | envelope de erro, leitura e escrita de JSON |
| `internal/{cte,mdfe,nfse,boleto}` | **o que** escrever: é aqui que o domínio de cada documento vive |

Antes de escrever um ajudante num módulo de documento, procure nas três
primeiras. Se o mesmo corpo já existe em outro módulo, ele pertence a uma
delas.

Duas exceções deliberadas: `ChaveDoXML` e `StatusEmissao` têm corpos idênticos
em CT-e e MDF-e, mas o que muda entre elas é o dado (a expressão da chave, o
tipo da resposta), e compartilhá-las custaria mais do que a cópia de cinco
linhas.

## O despacho RPC é escrito à mão, duas vezes

A API não abre a biblioteca nativa: ela fala com o `fiscal-worker` por RPC. Do
lado do cliente, cada método monta `Pedido{Metodo, Args}`; do lado do worker,
um `switch` lê aquele `Args` de volta para chamar o método. **Nada no
compilador liga as duas pontas**: um nome de método com typo, ou um campo de
`Args` trocado, compila e vira dado errado atravessando a fronteira em silêncio.

Por isso o teste percorre todos os métodos das cinco interfaces, chama cada um
pelo cliente com uma sentinela distinta por posição, e cobra do outro lado o
mesmo nome e os mesmos valores. O espião que recebe as chamadas **não embute
nada**: método novo numa interface quebra a compilação, e quem o acrescentar é
obrigado a passar por ali.

## Testes de lockstep

`internal/{cte,mdfe,nfse}/testdata/*.tsv` guardam as chaves de INI que a lib
aceita. Dois testes comparam em direções opostas:

1. a lib aceita e **não enviamos** → lacuna de cobertura;
2. **enviamos** e a lib não lê → chave morta, o dado é descartado em silêncio.

A comparação normaliza o índice das seções repetidas (`Comp001` e `Comp` são a
mesma coisa). Cuidado: **nem todo dígito no fim é índice.** `ICMS60` e `toma4`
são nomes do layout, e achatá-los fundia as sete variantes de ICMS numa só,
deixando o teste cego. Seção numerada nova exige uma decisão explícita, e há um
teste que a cobra.

Falhar ali não é "conserte o código", é **decida**: passar a enviar, ou declarar
com o motivo. Foi um desses baselines que registrou por meses que o CT-e não
enviava `tpAmb`.

Só que nenhuma das duas pontas é o **contrato**. Um campo que existe no modelo
Go e que nenhuma linha escreve não aparece em conjunto nenhum, e as duas
comparações passam enquanto o dado morre na tradução. Por isso existe a terceira
direção, o **espelho** (`internal/*/espelho_test.go`): ele preenche o modelo
inteiro com valores sentinela, gera o INI, e cobra cada folha do contrato na
saída. O que não sai vai para `testdata/nao_espelhadas*.tsv` **com o motivo**,
e uma linha que deixa de valer é acusada: lista que vira depósito para de
significar alguma coisa.

Ele achou, entre outros, o `versaoModal` que os dois documentos aceitavam sem
ter para onde mandar, e que por isso saiu do contrato, e o `autXML` que saía com
índice de três casas onde a biblioteca lê duas.

## A especificação é gerada

`api/openapi.yaml` tem as rotas escritas à mão e os **schemas gerados** dos
modelos Go, entre marcadores. Ao mexer num modelo:

```bash
make openapi        # regenera
make openapi-check  # é o que o CI roda
```

Expôs um pedido novo numa rota? Acrescente-o a `raizes` em
`cmd/gerar-openapi/main.go`, senão ele fica fora da documentação.

Escreva o comentário do campo pensando em quem vai ler no Swagger: ele **é** a
descrição publicada.

### Enriquecendo um campo

Três fontes alimentam a documentação de um campo, nesta ordem:

1. **comentário no modelo**, que sempre vence, por ser específico do contexto;
2. **`internal/fiscal/glossario.tsv`**, a descrição padrão de nomes que se
   repetem nos três documentos (`CNPJ`, `xNome`, `cMun`, `vBC`). Comentar um a
   um seria copiar a mesma frase em dezenas de lugares para desatualizar em
   alguns;
3. **tags no campo**:
   - `enum:"TipoServico"` publica os valores aceitos e a legenda. A tabela é
     `internal/<doc>/enums.tsv`, com os valores vindos das unidades de conversão
     da ACBrLib, que é o que a lib aceita de fato;
   - `fmt:"data"` ou `fmt:"data-hora"` publicam o formato e a semântica do
     offset.

```go
TpServ int    `json:"tpServ" enum:"TipoServico"`
DhEmi  string `json:"dhEmi"  fmt:"data-hora"`
```

Campo enumerado que aparece como `integer` puro no Swagger não diz nada a quem
integra. Se você encostar num, aproveite e enriqueça.

## Pull requests

- um assunto por PR;
- `make fmt-check vet test` verdes;
- teste que falharia sem a sua mudança;
- comentário explicando **por que**, não o que: o código já diz o que.

Domínio e comentários em **português**: o domínio é brasileiro e os termos não
têm tradução útil.
