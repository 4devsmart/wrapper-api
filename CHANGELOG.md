# Changelog

O que mudou em cada versão, para quem opera o serviço. As versões seguem
[SemVer](https://semver.org/lang/pt-BR/), e a página de cada release sai daqui.

Toda versão tem seção aqui, escrita antes da tag: o release falha sem ela, e nada
é promovido. Use `### Antes de atualizar` para o que exige ação de quem atualiza.
As versões até a v1.6.0 foram preenchidas depois, a partir dos commits.

## v1.7.0 - 2026-11-06

### Antes de atualizar

**Tempo de parada: defina 120s na API e no worker.** Uma transmissão pode levar até 90s, e o `docker stop` mata o container em 10s. Sem um prazo maior, um deploy no meio de uma emissão corta a transmissão: a nota pode sair sem que a sua aplicação receba a resposta. A imagem não tem como definir esse prazo: ele é de quem inicia o container, e vale com log ligado ou não.

| Como você sobe | O que definir |
|---|---|
| `docker-compose.yml` deste repositório | Nada: já vem com `stop_grace_period: 120s` |
| Compose próprio | `stop_grace_period: 120s` nos dois serviços |
| `docker run` | `--stop-timeout 120` nos dois containers |
| Kubernetes | `terminationGracePeriodSeconds: 120` |

Se aumentar `ACBR_WORKER_TIMEOUT_SECONDS`, suba este prazo junto.

**`ACBR_LOG_PATH` em produção.** Só afeta quem definiu essa variável. Com `MODO=producao`, `ACBR_LOG_PATH` sem `ACBR_LOG_NIVEL` passa a ter o boot recusado, porque o nível padrão grava XML e certificado em disco. Remova a variável ou defina um nível abaixo de 3.

**Teste um evento de CT-e ou MDF-e em homologação.** O `dhEvento` agora é convertido para o fuso do documento, e o formato enviado à lib mudou.

### Adicionado

- nfse: log por operação, com rota, motivo, ambiente, desfecho e códigos de rejeição, sem dados pessoais (#44)
- nfse: tabela de vigências do Ambiente Nacional no fonte, uma linha por município e regime (#43)

### Corrigido

- nfse: nota autorizada não volta mais como `422` por citar "implementado" e "provedor" no XML (#45)
- acbr: o buffer de resposta deixa espaço para o NUL da lib, que caía fora dele (#46)
- config: produção sobe pelo compose, porque o worker deixou de exigir `API_TOKEN` (#47)
- compose: `ACBR_WORKER_SLOTS` e `ACBR_WORKER_TIMEOUT_SECONDS` chegam à API, onde fica o pool (#47)
- encerramento: o prazo cobre o teto de uma chamada, em vez de 60s fixos (#47)
- acbr: esperar na fila não encurta a transmissão (#49)
- readyz: não disputa a vaga com as emissões, e não acusa indisponível um worker ocupado (#49)
- cte, mdfe: `dhEvento` e o `dhEmi` do EPEC saem no fuso do documento (#50)

### Alterado

- publicação: só a imagem aprovada vira `:<sha>` e `:latest` (#48)
- release: só a versão mais alta move `:stable` e `:vN`; a tag precisa ser `vX.Y.Z` (#48)
- nfse: a rota é decidida em um lugar só, e `MunicipioADN` virou `SessaoNacional` (#42, #43)
- testes: golden de rotas e de XML por combinação de provedor (#41)

## v1.6.0 - 2026-11-06

### Adicionado

- nfse: ME/EPP de Brasília sai pelo Ambiente Nacional desde 01/11 (#33)

## v1.5.2 - 2026-11-06

### Alterado

- acbr: chave da DPS sai do cLocEmi informado (patch 0004) (#32)

## v1.5.1 - 2026-11-06

Sem mudança de código: republica as libs nativas desta revisão.

## v1.5.0 - 2026-10-21

### Corrigido

- nfse: distribuição tenta o município antes do ADN; tomador sem documento é recusado (#29)

### Alterado

- deps: Go 1.27.3 nos dois lugares (#30)
- deps: sobe o ACBr para r48784 (#31)

## v1.4.1 - 2026-10-21

### Corrigido

- nfse: distribuição DF-e sai sempre pelo ADN, sem pedir município (#27)

## v1.4.0 - 2026-10-05

### Adicionado

- nfse: eventos identificam a nota e o cancelamento já feito (#26)

## v1.3.2 - 2026-10-03

### Corrigido

- nfse: municípios da API própria do provedor saem no Padrão Nacional (#24)

### Alterado

- release: confere as libs dentro da imagem e registra o digest (#25)

## v1.3.1 - 2026-09-16

### Alterado

- acbr: as libs publicadas passam a levar os patches locais (#23)

## v1.3.0 - 2026-09-16

### Adicionado

- nfse: INI completo, lote assíncrono e cancelamento no ABRASF (#22)

## v1.2.0 - 2026-09-09

### Corrigido

- nfse: cancelamento e substituição pelo webservice que o provedor tem (#21)

## v1.1.2 - 2026-09-08

### Corrigido

- release: notas dizem onde estão os binários da ACBrLib (#18)
- release: confere os anexos da ACBrLib antes de apontar para eles (#19)
- nfse: DANFSE por XML seleciona o provedor antes de carregar (#20)

## v1.1.1 - 2026-09-07

### Alterado

- release: binários da ACBrLib passam a ser anexos da tag da versão (#17)

## v1.1.0 - 2026-09-07

### Adicionado

- ci: o release passa a criar a própria página (#11)

### Corrigido

- release: pacote da ACBrLib não pode levar o selo Latest (#16)

### Alterado

- deps: sobe o ACBr para r48100 (#14)
- lockstep: compara INI gerado com chaves extraídas do fonte (#15)

## v1.0.0 - 2026-08-24

Primeira versão pública.
