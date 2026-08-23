# wrapper-api

API REST em **Go** para documentos fiscais brasileiros — **CT-e**, **MDF-e**,
**NFS-e** e **boletos** — com interface **JSON pura**, rodando em **Docker** e
**sem guardar nada**.

> **English** — A stateless JSON REST API (Go) for Brazilian fiscal documents:
> CT-e (transport bill), MDF-e (freight manifest), NFS-e (municipal service
> invoice) and bank slips (*boletos*). It wraps a native fiscal library
> (ACBrLib) via cgo. Domain code and comments are in Portuguese — the domain is
> Brazil-specific.

## O que o torna diferente

**Não persiste nada.** Sem banco, sem bucket, sem volume de dados, sem cadastro.
O certificado A1 chega no corpo da requisição que transmite, é usado em memória e
vai embora com ela. Os containers são descartáveis e escalam na horizontal.

**Duas fases, para não perder transmissão.** Emitir não é uma chamada só:

```
1. POST /v1/cte/xml          → devolve o XML e a CHAVE. Sem certificado, sem rede.
2. POST /v1/cte/transmissao  → assina e envia. O certificado entra aqui.
```

A fase 1 põe o documento e a identidade dele nas suas mãos **antes** de qualquer
byte sair. É isso que torna recuperável uma transmissão cuja resposta se perdeu:
você consulta pela chave em vez de reenviar e duplicar.

**A lib nativa roda em outro processo.** Um defeito nela é um sinal do SO, sem
`recover` possível. O `fiscal-worker` a carrega; a API só fala RPC com ele por
socket unix. Um crash mata a requisição em curso, não o servidor.

## Subindo

```bash
cp .env.example .env          # defina API_TOKEN
make acbr-libs-baixar         # baixa as libs nativas (ver docs/ACBRLIB.md)
docker compose up -d
curl localhost:8080/healthz
```

Ou como serviço no compose de outro projeto, sem credencial de registry:

```yaml
services:
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
volumes:
  fiscal-run:
```

Configuração é **só por variável de ambiente** — não há arquivo de config nem
página de administração. Todas as variáveis estão em `.env.example`.

## O contrato

`GET /v1/capacidades` diz o que esta build sabe fazer. O resumo:

| Módulo | Rotas |
|---|---|
| **cte** | `xml` · `simp/xml` · `transmissao` · `eventos/{tipo}` · `consulta` · `status-servico` · `cadastro` · `pdf` · `pdf/evento` |
| **mdfe** | `xml` · `transmissao` · `eventos/{tipo}` · `consulta` · `status-servico` · `nao-encerrados` · `pdf` · `pdf/evento` |
| **nfse** | `xml` · `transmissao` · `eventos/{tipo}` · `consulta` · `consulta-dps` · `consultas/{tipo}` · `distribuicao` · `pdf` · `municipios/{codigo}` |
| **boletos** | `pdf` · `remessa` · `retorno` · `registro` |

Eventos do CT-e: cancelamento, carta-correcao, epec, comprovante-entrega,
insucesso-entrega, prestacao-desacordo. Do MDF-e: encerramento, cancelamento,
inclusao-condutor, inclusao-dfe, pagamento-operacao. Da NFS-e: cancelamento,
substituicao.

**Quase tudo é POST**, inclusive consultas: toda operação que fala com o fisco
carrega o certificado no corpo, e certificado não vai em query string.

Especificação completa em `GET /openapi.yaml`; índice navegável em `GET /docs`.

## Arquitetura

```
Cliente ──HTTP/JSON──▶ api (Go, sem cgo)
  1. servidor: mux, auth, teto de corpo        internal/servidor
  2. um módulo por documento                   internal/{cte,mdfe,nfse,boleto}
  3. vocabulário comum aos documentos          internal/fiscal
  4. infraestrutura sem domínio fiscal         internal/platform
  5. binding da lib nativa                     internal/acbr
         │
         └── RPC (JSON sobre socket unix) ──▶ fiscal-worker (cgo)
                                              carrega a lib; é quem pode crashar
```

Um módulo **não importa outro módulo**, não conhece o servidor, e `platform/` não
conhece domínio fiscal. Isso é verificado por teste sobre o grafo de imports
(`internal/modulo/fronteira_test.go`), não por convenção.

**Nós não geramos XML.** O cliente manda JSON, traduzimos para o INI que a lib
consome, e é ela que monta, assina e transmite. Essa tradução é o grosso do
trabalho de domínio.

## Desenvolvendo

```bash
make test        # unidade (sem lib nativa: o binding vira stub)
make test-cgo    # type-check + link do binding contra as .so reais
make build       # api + fiscal-worker (sem cgo)
make help        # todos os alvos
```

Os testes de domínio rodam **sem certificado e sem a lib** — é uma consequência
direta de a fase 1 não assinar.

## Limitações

Leia **[docs/LIMITACOES.md](docs/LIMITACOES.md)** antes de colocar em produção.
Os pontos que mais surpreendem:

- perdeu o XML do CT-e/MDF-e, perdeu o PDF — não há segunda via;
- timeout na transmissão **não** autoriza repetir: consulte pela chave;
- não há proteção contra duplicidade — número e série são sua disciplina;
- NFS-e é multi-provedor e a capacidade é descoberta em runtime.

## Licença

**[AGPL-3.0](LICENSE)** — núcleo público, sem porção fechada. A ACBrLib é
redistribuída sob LGPL; veja **[NOTICE](NOTICE)** para atribuição e instruções de
recompilação.
