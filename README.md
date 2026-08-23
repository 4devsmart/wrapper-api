# wrapper-api

API REST para documentos fiscais brasileiros — **CT-e**, **MDF-e**, **NFS-e** e
**boletos**. JSON puro, Docker, **sem estado**.

Você manda o documento em JSON, a API devolve o XML. Você transmite esse XML por
uma segunda chamada. Nada é guardado no servidor — nem o certificado.

> **English** — Stateless JSON REST API (Go) wrapping the native ACBrLib for
> Brazilian fiscal documents. Domain code and comments are in Portuguese: the
> domain is Brazil-specific and the terms have no useful translation.

---

## Subindo

```bash
git clone https://github.com/4devsmart/wrapper-api && cd wrapper-api
cp .env.example .env            # defina API_TOKEN
make acbr-libs-baixar           # libs nativas (~56 MB, fora do clone)
docker compose up -d
curl localhost:8080/healthz
```

Abra **`http://localhost:8080/docs`** — Swagger UI com todos os campos de todos
os documentos, servido do próprio binário.

Como serviço no compose de outro projeto, sem credencial de registry:

```yaml
services:
  fiscal-api:
    image: ghcr.io/4devsmart/wrapper-api/api:v1
    environment:
      API_TOKEN: ${FISCAL_API_TOKEN}
      ACBR_WORKERS: /run/wrapper/fiscal.sock
    ports: ["8080:8080"]
    volumes: [fiscal-run:/run/wrapper]
  fiscal-worker:
    image: ghcr.io/4devsmart/wrapper-api/api:v1
    command: ["worker"]
    volumes: [fiscal-run:/run/wrapper]
volumes: { fiscal-run: }
```

Configuração é **só por variável de ambiente**. Todas estão em `.env.example` —
são oito.

---

## Emitir: duas chamadas

**1. Montar** — devolve o XML e a chave. Sem certificado, sem rede.

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

**2. Transmitir** — o certificado entra aqui, e só aqui.

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

### Por que duas chamadas

Porque a fase 1 põe a chave nas suas mãos **antes** de qualquer byte sair. Se a
fase 2 der timeout, você não sabe se o documento foi autorizado — e é a chave
que permite descobrir, em vez de reenviar e duplicar.

```bash
# depois de um 502 desfecho_indeterminado:
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
| `cte` | `xml` · `simp/xml` · `transmissao` · `eventos/{6 tipos}` · `consulta` · `status-servico` · `cadastro` · `pdf` · `pdf/evento` |
| `mdfe` | `xml` · `transmissao` · `eventos/{5 tipos}` · `consulta` · `status-servico` · `nao-encerrados` · `pdf` · `pdf/evento` |
| `nfse` | `xml` · `transmissao` · `eventos/{2 tipos}` · `consulta` · `consulta-dps` · `consultas/{5 tipos}` · `distribuicao` · `pdf` · `municipios/{codigo}` |
| `boletos` | `pdf` · `remessa` · `retorno` · `registro` |

Eventos — CT-e: `cancelamento`, `carta-correcao`, `epec`, `comprovante-entrega`,
`insucesso-entrega`, `prestacao-desacordo`. MDF-e: `encerramento`,
`cancelamento`, `inclusao-condutor`, `inclusao-dfe`, `pagamento-operacao`.
NFS-e: `cancelamento`, `substituicao`.

**Quase tudo é POST**, inclusive consultas: toda operação que fala com o fisco
carrega o certificado no corpo, e certificado não vai em query string.

Sem autenticação: `/healthz`, `/readyz`, `/docs`, `/openapi.yaml`.

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
| `provedor_nao_suportado` | 422 | município sem provedor de NFS-e conhecido | — |
| `operacao_nao_suportada` | 422 | o provedor não implementa esta operação | — |
| `desfecho_indeterminado` | 502 | **pode ter sido transmitido** | **não** — consulte |
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

- **não há proteção contra duplicidade** — número e série são sua disciplina;
- **CT-e e MDF-e não recuperam o PDF pela chave**; só a NFS-e recupera;
- **NFS-e é multi-provedor** e a capacidade é descoberta em runtime — consulte
  `/v1/nfse/municipios/{codigo}` antes de montar;
- **a validação da fase 1 não é XSD** (o schema exige assinatura); o XSD roda na
  fase 2, ainda antes de transmitir;
- **não paralelize a distribuição DF-e do mesmo CNPJ** — o cursor é seu.

---

## Como funciona

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

A lib fiscal é nativa (ABI C, compilada do fonte oficial do ACBr) e um defeito
nela derruba o processo que a hospeda — sinal do SO, sem `recover`. Por isso ela
**não roda dentro da API**: o worker a carrega, e um crash mata a requisição em
curso, não o servidor.

**Nós não geramos XML.** O cliente manda JSON, traduzimos para o INI que a lib
consome, e é ela que monta, assina e transmite. Essa tradução é o grosso do
trabalho de domínio.

Um módulo não importa outro módulo, não conhece o servidor, e `platform/` não
conhece domínio fiscal — verificado por teste sobre o grafo de imports, não por
convenção.

---

## Desenvolvendo

```bash
make test        # unidade — sem lib nativa e sem certificado
make test-cgo    # type-check + link do binding contra as .so reais
make openapi     # regenera os schemas da spec a partir dos modelos Go
make help
```

O domínio inteiro é testável **sem a lib e sem certificado** — consequência
direta de a fase 1 não assinar.

Os schemas do `openapi.yaml` são **gerados dos modelos Go**, com as descrições
vindas dos comentários do fonte: o contrato publicado é o contrato compilado, e
o CI reprova se a spec envelhecer. Não há `required` — no domínio fiscal a
obrigatoriedade é condicional (modal, provedor, tipo de emitente), e um
`required` estático mentiria em metade dos casos.

[CONTRIBUTING.md](CONTRIBUTING.md) tem as armadilhas que já custaram tempo.

---

## Licença

**[AGPL-3.0](LICENSE)** — núcleo público, sem porção fechada. A ACBrLib é
redistribuída sob LGPL; veja [NOTICE](NOTICE) para atribuição e recompilação.
