# wrapper-api

Gera o XML do documento fiscal a partir do seu JSON, assina, transmite à SEFAZ
ou à prefeitura, e devolve o documento **protocolado**. CT-e, MDF-e, NFS-e e
boletos.

**Nada é guardado no servidor.** Nem o XML, nem o certificado.

> **English.** Stateless JSON REST API (Go) wrapping the native ACBrLib for
> Brazilian fiscal documents. It generates, signs and transmits the XML, and
> returns the authorized document. It stores nothing: your application owns the
> data. Domain code and comments are in Portuguese, because the domain is
> Brazil-specific and the terms have no useful translation.

## O que fazemos, e o que não fazemos

| Fazemos | Não fazemos |
|---|---|
| traduzir o seu JSON em XML fiscal e assinar | **decidir o que preencher.** CFOP, CST, alíquota, natureza da operação e valores são seus |
| transmitir ao fisco e devolver o retorno protocolado, autorizado ou rejeitado com o motivo | **controlar numeração e série**, ou impedir duplicidade |
| enviar os eventos: cancelamento, encerramento, carta de correção, EPEC | **guardar o XML.** Nem o autorizado. Não há histórico, listagem nem segunda via |
| gerar DACTE, DAMDFE e DANFSE a partir do XML | **guardar o certificado.** Ele chega no corpo da requisição e morre com ela |
| gerar boletos, arquivo de remessa e leitura de retorno CNAB | fila, retry, agendamento, tela de cadastro ou relatório |

**A responsabilidade pelo conteúdo fiscal é de quem emite.** Validamos o que a
biblioteca fiscal valida, que são as regras de negócio e o schema, e
transmitimos o que você mandou. Se o CFOP está errado, o documento é autorizado
errado.

### Escopo: modal rodoviário

**CT-e e MDF-e cobrem o modal rodoviário, e só ele.** Aéreo, aquaviário,
ferroviário, dutoviário e multimodal estão fora: os grupos não existem no
contrato, e o pedido que os traga é recusado na entrada.

| Você manda | Recebe |
|---|---|
| `ide.modal` diferente de rodoviário | `422 modal_nao_suportado` |
| `infModal.aereo`, `.aquav`, `.ferrov`, `.duto`, `.multimodal` | `400 campo desconhecido` |

Recusar é a escolha deliberada. A alternativa, aceitar o campo e ignorá-lo,
gera um documento autorizado sem o grupo do modal, e o erro só aparece depois
de o fisco autorizar. Um 400 na hora custa menos.

NFS-e e boletos não têm modal: o recorte não os afeta.

## Como ele roda

**Como serviço Docker, dentro da sua stack.** São dois containers que você
acrescenta ao `docker-compose.yml` da sua aplicação: a API e o worker que
carrega a biblioteca fiscal.

```yaml
services:
  minha-app:            # a sua aplicação
    environment:
      FISCAL_URL: http://fiscal-api:8080     # chamada interna, sem sair da rede

  fiscal-api:
    image: ghcr.io/4devsmart/wrapper-api/api:v1
    environment:
      API_TOKEN: ${FISCAL_API_TOKEN}
      ACBR_WORKERS: /run/wrapper/fiscal.sock
    volumes: [fiscal-run:/run/wrapper]

  fiscal-worker:
    image: ghcr.io/4devsmart/wrapper-api/api:v1
    command: ["worker"]
    volumes: [fiscal-run:/run/wrapper]

volumes: { fiscal-run: }
```

Repare que **`fiscal-api` não publica porta**. A sua aplicação alcança o serviço
pelo nome do container, na rede interna do Docker. Publicar a porta é opcional e
serve só para você abrir o `/docs` durante o desenvolvimento.

Não há interface para pessoa: nem tela, nem login, nem cadastro. Quem chama é o
seu código.

### Quem é responsável pelo quê

```
  sua aplicação  ────▶  wrapper-api  ────▶  SEFAZ / prefeitura / banco
                HTTP                  HTTPS
                interno               com o certificado que você enviou

  sua aplicação guarda:          wrapper-api guarda:
    o XML protocolado              nada
    o protocolo e a chave
    a numeração e a série
    o certificado A1
```

O seu sistema decide o conteúdo do documento, controla a numeração, guarda o XML
protocolado e trata os erros. Este serviço recebe o JSON, devolve o XML, e
transmite quando você mandar.

## Por que foi desenhado assim

**O estado fiscal já vive no seu sistema.** Guardá-lo aqui também criaria duas
fontes da verdade, que divergem no primeiro erro de rede. A sua é a que vale.
Sem banco, não há o que sincronizar, migrar ou reconciliar.

**Certificado de terceiro em repouso é um risco que ninguém quer.** Sem
persistência, um comprometimento deste servidor não vaza certificado nenhum,
porque não há o que vazar. O A1 chega na chamada que transmite, é usado em
memória e some com ela.

**Gerar e transmitir são chamadas separadas** para que o documento e a chave
estejam nas suas mãos antes de qualquer byte sair. É isso que torna recuperável
uma transmissão cuja resposta se perdeu: você consulta pela chave em vez de
reenviar e duplicar. Sem essa separação, um timeout viraria documento fantasma,
autorizado no fisco e desconhecido para você.

### A biblioteca fiscal roda em outro processo

Quem fala com a SEFAZ é a **ACBrLib**, uma biblioteca nativa. Ela pode falhar de
um jeito que não dá para tratar em Go: um SIGSEGV, que é o sistema operacional
matando o processo. Não existe `recover` para isso. Se ela rodasse dentro da
API, uma falha dessas derrubaria o servidor inteiro e todas as requisições em
andamento junto.

Por isso ela roda num processo separado, o `fiscal-worker`. A API não carrega a
biblioteca: conversa com o worker por um socket unix. Quando a biblioteca falha:

1. o worker morre, e leva junto **apenas a requisição que estava atendendo**;
2. o Docker reinicia o worker em segundos;
3. a API continua de pé e responde `503` enquanto isso;
4. a requisição afetada recebe erro tipado, não uma conexão cortada.

Você configura quantas requisições um crash pode levar junto: é o número de
workers vezes `ACBR_WORKER_SLOTS`, que por padrão é 1 × 1. Mais slots dão mais
vazão e aumentam o estrago de uma falha, na mesma proporção.

O mesmo raciocínio vale para o PDF. A geração da representação gráfica é o que
mais falha na biblioteca, então ela fica **fora** do caminho da transmissão: o
documento é transmitido primeiro, e o PDF é gerado depois, numa chamada
separada. Assim o pior caso é você não conseguir o PDF, e não um documento
autorizado no fisco que você nunca soube que existiu.

### A biblioteca acompanha as notas técnicas

A ACBrLib é mantida em cima das **notas técnicas** publicadas pelo fisco, e
segue o ciclo de atualização de layout de CT-e, MDF-e e NFS-e. Quando uma nota
técnica muda um campo, uma regra de validação ou uma URL de webservice, a
mudança chega pela biblioteca.

O que isso exige de você: **manter a versão em dia**. Atualizar a biblioteca é
subir a revisão pinada, regenerar os artefatos e publicar uma imagem nova.
Prazos de nota técnica são obrigatórios, e uma versão atrasada é rejeição na
data de virada. Ver [docs/ACBRLIB.md](docs/ACBRLIB.md).

Na NFS-e há um detalhe a mais: cada município escolhe o seu provedor, e a tabela
que mapeia município para provedor muda com frequência, porque municípios vão
migrando para o Padrão Nacional. Essa tabela também vem da biblioteca, e
`GET /v1/nfse/municipios/{codigo}` diz o que a sua versão conhece.

## Subindo

```bash
git clone https://github.com/4devsmart/wrapper-api && cd wrapper-api
cp .env.example .env            # defina API_TOKEN
make acbr-libs-baixar           # libs nativas (~56 MB, fora do clone)
docker compose up -d
curl localhost:8080/healthz
```

Abra **`http://localhost:8080/docs`**. É o Swagger UI, servido do próprio
binário, com todos os campos de todos os documentos.

Para embutir na stack de outro projeto, veja [Como ele roda](#como-ele-roda).
A imagem é pública: não há `docker login`.

Configuração é **só por variável de ambiente**, e o `.env.example` lista todas:
um teste recusa a lista incompleta, dos dois lados. Na prática você define
`API_TOKEN` e `MODO`; o resto tem default de imagem.

---

## Gerar e transmitir

**1. Gerar.** Devolve o XML e a chave. Sem certificado, sem rede.

```bash
curl -X POST localhost:8080/v1/cte/xml \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d @cte.json
```
```json
{
  "chave": "35260899999999000191570010000001001311178271",
  "xml_b64": "PD94bWwgdmVyc2lvbj0iMS4wIj8+...",
  "assinado": false,
  "validacao": { "ok": true, "suportada": true }
}
```

**2. Transmitir.** O certificado entra aqui, e só aqui.

```bash
curl -X POST localhost:8080/v1/cte/transmissao \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"xml_b64":"PD94bWw...","certificado":{"pfx_b64":"MIIK...","senha":"..."}}'
```
```json
{
  "chave": "3526...", "protocolo": "135260000000001",
  "status": "autorizado", "cstat": "100",
  "xml_proc_b64": "PD94bWw..."
}
```

**Guarde o `xml_proc_b64`.** Não há segunda via: em CT-e e MDF-e, perder o XML é
perder também o PDF.

### Quando a resposta se perde

`502 desfecho_indeterminado` significa que o documento **pode** ter sido
autorizado. Não repita: consulte pela chave que você guardou ao gerar.

```bash
curl -X POST localhost:8080/v1/cte/consulta \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"chave":"3526...","certificado":{...}}'
```

Na NFS-e a identidade é o `id_dps` e a rota é `/v1/nfse/consulta-dps`.

---

## Rotas

`GET /v1/capacidades` lista o que a build atende. Resumo:

| Módulo | Rotas (sob `/v1`) |
|---|---|
| `cte` | `xml` · `simp/xml` · `transmissao` · `eventos/{6 tipos}` · `consulta` · `status-servico` · `cadastro` · `pdf` · `pdf/evento` (rodoviário) |
| `mdfe` | `xml` · `transmissao` · `eventos/{5 tipos}` · `consulta` · `status-servico` · `nao-encerrados` · `pdf` · `pdf/evento` (rodoviário) |
| `nfse` | `xml` · `transmissao` · `eventos/{2 tipos}` · `consulta` · `consulta-dps` · `consultas/{5 tipos}` · `distribuicao` · `pdf` · `municipios/{codigo}` |
| `boletos` | `pdf` · `remessa` · `retorno` · `registro` |

Eventos. CT-e: `cancelamento`, `carta-correcao`, `epec`, `comprovante-entrega`,
`insucesso-entrega`, `prestacao-desacordo`. MDF-e: `encerramento`,
`cancelamento`, `inclusao-condutor`, `inclusao-dfe`, `pagamento-operacao`.
NFS-e: `cancelamento`, `substituicao`.

**Quase tudo é POST**, inclusive consultas: toda operação que fala com o fisco
carrega o certificado no corpo, e certificado não vai em query string.

Sem autenticação: `/healthz`, `/readyz`, `/docs`, `/openapi.yaml`.

### Limite por endereço

`API_RATE_PER_MIN` (default 240) limita as requisições por minuto **por
endereço do chamador**, e vale **antes** do Bearer: sem isso, adivinhar o token
sairia ao ritmo da rede. Estourar devolve `429` com `Retry-After`. As sondas
`/healthz` e `/readyz` não gastam ficha. `0` desliga, o que é razoável atrás de
um gateway que já limita.

Por padrão o endereço é o da **conexão**. `TRUST_PROXY_HEADERS=true` manda ler
`X-Forwarded-For`, e só ligue isso se houver mesmo um proxy na frente
reescrevendo o cabeçalho: dentro do Docker o peer é o gateway da bridge, então
sem proxy de verdade todo chamador pareceria interno e um valor forjado furaria
o limite. Da cadeia vale a entrada mais à direita que seja externa, não a
primeira. Desligado, quem vem por um proxy divide um balde só.

---

## Erros

Formato único em toda rota:

```json
{ "erro": { "codigo": "...", "mensagem": "...", "detalhes": {} } }
```

Trate por `codigo`, nunca pela mensagem:

| Código | HTTP | Significado | Pode repetir? |
|---|---|---|---|
| `json_invalido` | 400 | corpo malformado, ou **campo com nome desconhecido** | corrija |
| `campo_obrigatorio` | 400 | falta um campo do envelope | corrija |
| `certificado_invalido` | 400 | `pfx_b64`/`senha` ausente ou ilegível | corrija |
| `regras_de_negocio` | 422 | a lib reprovou; nada foi transmitido | corrija |
| `provedor_nao_suportado` | 422 | município sem provedor de NFS-e conhecido | não |
| `operacao_nao_suportada` | 422 | o provedor não implementa esta operação | não |
| `grupo_incompativel` | 400 | grupo que não existe no tipo de documento pedido (ex.: `infCteComp` fora do CT-e Complementar) | corrija |
| `limite_de_requisicoes` | 429 | passou de `API_RATE_PER_MIN`; veja `Retry-After` | sim, depois |
| `desfecho_indeterminado` | 502 | **pode ter sido transmitido** | **não.** consulte |
| `lib_indisponivel` | 503 | a chamada **não** saiu | sim |
| `falha_na_lib` | 502 | erro conhecido da lib | avalie |

Rejeição do fisco não é erro de protocolo: volta `422` com o corpo completo
(`status`, `cstat`, `motivo`, `erros[]`).

**Campo com nome errado é `400`, não é ignorado.** É deliberado: campo ignorado
em silêncio vira documento transmitido com dado faltando, e isso só aparece
depois de a SEFAZ autorizar.

---

## Antes de produção

Leia **[docs/LIMITACOES.md](docs/LIMITACOES.md)**. O essencial:

- **não há proteção contra duplicidade.** Número e série são sua disciplina;
- **CT-e e MDF-e não recuperam o PDF pela chave**; só a NFS-e recupera;
- **NFS-e é multi-provedor** e a capacidade é descoberta em tempo de execução.
  Consulte
  `/v1/nfse/municipios/{codigo}` antes de montar;
- **a validação da geração não é XSD** (o schema exige assinatura); o XSD roda na
  transmissão, ainda antes de o documento sair;
- **não paralelize a distribuição DF-e do mesmo CNPJ.** O cursor é seu.

---

## Por dentro

```
Cliente ──HTTP/JSON──▶ api (Go, sem cgo)
  servidor: mux, auth, teto de corpo      internal/servidor
  um módulo por documento                 internal/{cte,mdfe,nfse,boleto}
  vocabulário comum aos documentos        internal/fiscal
  binding da lib nativa                   internal/acbr
         │
         └── RPC (JSON sobre socket unix) ──▶ fiscal-worker (cgo)
                                              carrega a lib; é quem pode crashar
```

**Nós não escrevemos o XML.** Traduzimos o seu JSON para o INI que a
biblioteca consome, e é ela que monta, assina e transmite. Essa tradução, do
contrato JSON até o layout que a biblioteca entende, é o grosso do trabalho de
domínio deste repositório.

Um módulo não importa outro módulo, não conhece o servidor, e `platform/` não
conhece domínio fiscal. Isso é verificado por teste sobre o grafo de imports,
não por convenção.

---

## Desenvolvendo

```bash
make test        # unidade, sem lib nativa e sem certificado
make test-cgo    # type-check + link do binding contra as .so reais
make openapi     # regenera os schemas da spec a partir dos modelos Go
make help
```

O domínio inteiro é testável **sem a biblioteca e sem certificado**, que é
consequência direta de a geração não assinar.

Os schemas do `openapi.yaml` são **gerados dos modelos Go**, com as descrições
vindas dos comentários do fonte: o contrato publicado é o contrato compilado, e
o CI reprova se a spec envelhecer.

Não há `required`: no domínio fiscal a obrigatoriedade é condicional, porque
depende do modal, do provedor e do tipo de emitente. Um `required` estático
mentiria em metade dos casos.

[CONTRIBUTING.md](CONTRIBUTING.md) tem as armadilhas que já custaram tempo.

---

## Licença

**[AGPL-3.0](LICENSE)**, núcleo público, sem porção fechada. A ACBrLib é
redistribuída sob LGPL; veja [NOTICE](NOTICE) para atribuição e recompilação.
