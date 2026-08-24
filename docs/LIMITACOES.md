# Limitações conhecidas

Preferimos dizer o que **não** fazemos a deixar você descobrir em produção.

O escopo, o que o serviço faz, o que não faz e por quê: está no
[README](../README.md#o-que-fazemos-e-o-que-não-fazemos). Aqui ficam as
consequências práticas, as que mordem em produção.

## O modelo: sem estado

O serviço **não persiste nada**. Não há banco, bucket, volume de dados nem
cadastro. O certificado A1 chega no corpo da requisição que transmite, é usado
em memória e vai embora com ela.

O que isso implica, e não é negociável:

- **Guardar o documento é responsabilidade do cliente.** A geração devolve o XML
  e a identidade dele (chave, ou `id_dps` na NFS-e). Se você não gravar, não há
  segunda via aqui.
- **O certificado trafega em toda transmissão.** Use TLS na borda. O PFX vai da
  API ao worker por socket unix e nunca sai do host, mas entre o seu sistema e a
  API ele viaja: proteja esse trecho.
- **Não há proteção contra duplicidade.** O `409` que um banco local daria antes
  de transmitir não existe. Sobram a rejeição do próprio fisco e a sua disciplina
  com número e série.
- **Sem métricas nem tracing.** Há `/healthz` (o processo está vivo) e `/readyz`
  (o motor fiscal atende). Nada de Prometheus ou OTel.

## O que ainda não foi exercitado contra o fisco

O caminho até a montagem do documento é verificado contra a biblioteca real, em
container: o XML é montado, a chave é calculada, a validação de regras da
biblioteca aprova, e o XML volta a ser aceito por ela (o DACTE é gerado a partir
dele). A imagem publicada é testada a cada build, e as libs nativas são
compiladas a partir do fonte pinado.

O que **não** foi exercitado é o último passo: o envio de verdade à SEFAZ, que
exige um certificado A1 válido. Ele não roda em CI e não roda sem certificado
real de alguém.

Na prática, antes de apontar para produção:

1. transmita em **homologação** com o seu certificado, e confira que volta
   protocolo e `cstat`;
2. confira o `verProc` do XML autorizado: é ele que identifica quem gerou o
   documento, e o default é o nome deste serviço mais o commit;
3. só então mude `MODO` para `producao`.

## Transmissão: o que fazer quando a resposta se perde

**Timeout na transmissão não é falha: é desfecho desconhecido.** O documento pode ter
sido autorizado. A API devolve `502 desfecho_indeterminado` justamente para não
convidar você a repetir.

O certo é **consultar pela identidade que a geração devolveu**:

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

## Validação: o que a geração garante, e o que não

| Documento | Regras de negócio ao gerar |
|---|---|
| CT-e, MDF-e | ✅ sim (`ValidarRegrasdeNegocios`) |
| NFS-e | ❌ a lib não expõe: a resposta traz `validacao.suportada: false` |

Em nenhum caso é **validação de XSD**: o schema exige a tag `Signature`, e o
documento só é assinado no envio. O XSD é validado na transmissão, ainda **antes** de
transmitir: falhou ali, nada saiu.

## Escopo fiscal

- **NF-e e NFC-e não são emitidas.** Distribuição DF-e e manifestação do
  destinatário estão portadas no binding mas ainda não expostas (etapa própria).
- **CT-e e MDF-e cobrem o modal rodoviário, e só ele.** Aéreo, aquaviário,
  ferroviário, dutoviário e multimodal saíram do contrato. Não é campo aceito e
  ignorado: `ide.modal` fora do rodoviário é `422 modal_nao_suportado`, e os
  grupos `infModal.aereo`, `.aquav`, `.ferrov`, `.duto` e `.multimodal` não
  existem no schema, então mandá-los é `400 campo desconhecido`.

  A decisão fechou três lacunas que existiam antes: o `peri` aéreo e o
  `tarifa.CL` do CT-e divergiam entre modelo e INI, e o multimodal (COTM) nunca
  foi lido por INI pela biblioteca. Os três eram aceitos no contrato e
  descartados na montagem, que é o pior desfecho: o documento saía autorizado
  sem eles.

  As chaves de INI dos modais fora de escopo estão listadas em
  `internal/{cte,mdfe}/testdata/nao_enviadas.tsv`, para o lockstep não tratá-las
  como lacuna nova a cada bump da biblioteca.
- **Todo campo do contrato ou chega ao documento, ou está registrado.** O teste
  de espelho preenche o modelo inteiro e cobra cada campo na saída; o que não
  sai está em `internal/*/testdata/nao_espelhadas*.tsv` com o motivo. Os casos
  mais comuns são campo que a biblioteca calcula (dígito verificador, QR Code),
  campo que existe só no outro layout da NFS-e, e o par CNPJ/CPF, onde só um
  viaja. "Aceito e descartado em silêncio" deixou de ser um estado possível.
- **NFS-e é multi-provedor e a capacidade é descoberta em runtime.** Não existe
  tabela do que cada município aceita: quando o provedor não implementa a
  operação, a resposta é `422 operacao_nao_suportada`. Cancelamento e
  substituição não existem em todos. Consulte
  `GET /v1/nfse/municipios/{codigo}` antes.
- **Não dá para escolher o provedor de NFS-e.** Quem decide é o município, pela
  tabela embutida na biblioteca fiscal. Testado contra a lib: a chave de
  configuração `Provedor` é recusada em todas as formas, então não há override
  possível por aqui. Se um município migrou e a tabela da sua versão está
  atrasada, o caminho é atualizar a biblioteca.
- **Boletos: `ConsultarTitulos` não é exposto.** O método existe no binding, mas
  a seção de filtro que ele exigiria (`[BoletoConsulta]`) não aparece no fonte
  oficial do ACBr: o formato do INI seria chute.

## Representação gráfica (PDF)

| Documento | Recupera o PDF pela chave? |
|---|---|
| NFS-e | ✅ sim (`ObterDANFSE`) |
| CT-e, MDF-e | ❌ **não**: só a partir do XML autorizado |

**Perdeu o XML do CT-e ou do MDF-e, perdeu o DACTE/DAMDFE.** Não há como
regenerá-lo de lugar nenhum. `POST /v1/{cte,mdfe}/pdf` aceita o XML e é a única
via.

O render exige ambiente gráfico (GTK2 sob Xvfb), já embutido na imagem. É o
componente que mais crasha, por isso ele fica **fora** do caminho da emissão.

## Distribuição DF-e

O cursor (NSU) vai e volta no payload: sem estado, é o cliente que guarda onde
parou. **Não paralelize a distribuição do mesmo CNPJ**, sem o lock que existia
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
cliente final: o multi-tenant fino é do sistema que consome a API, que é o dono
dos certificados.
