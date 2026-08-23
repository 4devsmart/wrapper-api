# Limitações conhecidas

Preferimos dizer o que **não** fazemos a deixar você descobrir em produção.

## O modelo: sem estado

O serviço **não persiste nada**. Não há banco, bucket, volume de dados nem
cadastro. O certificado A1 chega no corpo da requisição que transmite, é usado
em memória e vai embora com ela.

O que isso implica, e não é negociável:

- **Guardar o documento é responsabilidade do cliente.** A fase 1 devolve o XML
  e a identidade dele (chave, ou `id_dps` na NFS-e). Se você não gravar, não há
  segunda via aqui.
- **O certificado trafega em toda transmissão.** Use TLS na borda. O PFX vai da
  API ao worker por socket unix e nunca sai do host, mas entre o seu sistema e a
  API ele viaja — proteja esse trecho.
- **Não há proteção contra duplicidade.** O `409` que um banco local daria antes
  de transmitir não existe. Sobram a rejeição do próprio fisco e a sua disciplina
  com número e série.
- **Sem métricas nem tracing.** Há `/healthz` (o processo está vivo) e `/readyz`
  (o motor fiscal atende). Nada de Prometheus ou OTel.

## Transmissão: o que fazer quando a resposta se perde

**Timeout na fase 2 não é falha — é desfecho desconhecido.** O documento pode ter
sido autorizado. A API devolve `502 desfecho_indeterminado` justamente para não
convidar você a repetir.

O certo é **consultar pela identidade que a fase 1 devolveu**:

| Documento | Rota de recuperação |
|---|---|
| CT-e | `POST /v1/cte/consulta` com a `chave` |
| MDF-e | `POST /v1/mdfe/consulta` com a `chave` |
| NFS-e | `POST /v1/nfse/consulta-dps` com o `id_dps` |

Repetir a transmissão às cegas é como se duplica documento fiscal. A API **nunca**
reenvia sozinha, e o cliente RPC tem o keep-alive desligado de propósito para que
nem a stdlib o faça.

**Não existe rota de recibo.** A SEFAZ desativou a recepção assíncrona; é tudo
síncrono e o protocolo volta na mesma chamada. Na 4.00 o CT-e força síncrono e
recusa lote com mais de um documento; o MDF-e força sem sequer olhar o parâmetro.

**Janela de emissão.** XML gerado hoje e transmitido semanas depois é rejeitado
por data de emissão atrasada. Gere e transmita no mesmo fluxo.

## Validação: o que a fase 1 garante, e o que não

| Documento | Regras de negócio na fase 1 |
|---|---|
| CT-e, MDF-e | ✅ sim (`ValidarRegrasdeNegocios`) |
| NFS-e | ❌ a lib não expõe — a resposta traz `validacao.suportada: false` |

Em nenhum caso é **validação de XSD**: o schema exige a tag `Signature`, e o
documento só é assinado no envio. O XSD é validado na fase 2, ainda **antes** de
transmitir — falhou ali, nada saiu.

## Escopo fiscal

- **NF-e e NFC-e não são emitidas.** Distribuição DF-e e manifestação do
  destinatário estão portadas no binding mas ainda não expostas (etapa própria).
- **MDF-e cobre apenas o modal rodoviário.** Aquaviário, ferroviário e aéreo,
  `unidCarga`/`unidTransp` com produtos perigosos e `infANTT.infPag` não são
  gerados.
- **CT-e** cobre todos os modais, mas o `peri` aéreo e o `tarifa.CL` divergem
  entre modelo e INI, e o multimodal (COTM) não é lido por INI pela lib.
- **NFS-e é multi-provedor e a capacidade é descoberta em runtime.** Não existe
  tabela do que cada município aceita: quando o provedor não implementa a
  operação, a resposta é `422 operacao_nao_suportada`. Cancelamento e
  substituição não existem em todos. Consulte
  `GET /v1/nfse/municipios/{codigo}` antes.
- **Boletos: `ConsultarTitulos` não é exposto.** O método existe no binding, mas
  a seção de filtro que ele exigiria (`[BoletoConsulta]`) não aparece no fonte
  oficial do ACBr — o formato do INI seria chute.

## Representação gráfica (PDF)

| Documento | Recupera o PDF pela chave? |
|---|---|
| NFS-e | ✅ sim (`ObterDANFSE`) |
| CT-e, MDF-e | ❌ **não** — só a partir do XML autorizado |

**Perdeu o XML do CT-e ou do MDF-e, perdeu o DACTE/DAMDFE.** Não há como
regenerá-lo de lugar nenhum. `POST /v1/{cte,mdfe}/pdf` aceita o XML e é a única
via.

O render exige ambiente gráfico (GTK2 sob Xvfb), já embutido na imagem. É o
componente que mais crasha — por isso ele fica **fora** do caminho da emissão.

## Distribuição DF-e

O cursor (NSU) vai e volta no payload: sem estado, é o cliente que guarda onde
parou. **Não paralelize a distribuição do mesmo CNPJ** — sem o lock que existia
com banco, chamadas simultâneas embaralham o cursor e você perde documentos sem
perceber.

## Operação

- **Plataforma `linux/amd64`.** A ACBrLib só é distribuída para x86_64; em hosts
  ARM tudo roda emulado.
- **Concorrência fiscal é explícita e baixa por padrão**: nº de workers ×
  `ACBR_WORKER_SLOTS`, default 1×1. Aumentar dá vazão e amplia quantas
  requisições um crash da lib leva junto.
- **O log da ACBr a partir do nível 3 grava XML e certificado em disco.** Num
  serviço cuja promessa é não persistir nada, isso torna a promessa falsa. O boot
  **recusa** nível ≥ 3 com `MODO=producao`. Se ligar para depurar, apague depois.
- **Campo desconhecido no JSON é `400`.** É deliberado: campo com nome errado que
  o servidor ignora vira documento transmitido com dado faltando, e isso só
  aparece depois de o fisco autorizar.

## Modelo de acesso

Um token (`API_TOKEN`) dá acesso a tudo. Não há OAuth2, escopo nem token por
cliente final — o multi-tenant fino é do sistema que consome a API, que é o dono
dos certificados.
