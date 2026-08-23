# Contribuindo

Obrigado pelo interesse. Este documento é curto de propósito — o que importa
está em dois lugares: as armadilhas abaixo e as
[limitações](docs/LIMITACOES.md).

## Rodando

```bash
make test        # unidade — sem lib nativa e sem certificado
make acbr-libs-baixar && make up   # o stack de verdade
```

O domínio inteiro é testável **sem a lib nativa e sem certificado**. Isso não é
sorte: a fase 1 do contrato não assina, e é justamente o que torna a camada de
tradução JSON→INI — o ativo real do projeto — verificável por quem não tem um
A1 na mão.

## Armadilhas que já custaram tempo

**O build da lib nativa é TUDO no Docker.** Não compile o `.so` no host. Para
validar o binding cgo, use `make test-cgo` (type-check + link contra as `.so`) ou
rebuilde a imagem — nunca `go build -tags acbrlib` avulso.

**Método novo na interface `Servico` = 6 lugares.** Interface, `acbrlib.h`, o
binding cgo, o stub `indisponivel`, o cliente `remoto` e o despacho em
`rpcserver.go`. O teste `TestDespachoCobreTodasAsInterfaces` pega o esquecimento
no despacho, mas não os outros — confira os seis.

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
3+ grava XML **e certificado** — se ligar para depurar, apague depois.

**Fixtures são sintéticas.** Nunca use XML, CNPJ ou certificado reais em teste.

## O que o código já decidiu

Antes de propor uma mudança estrutural, saiba o que está fechado e por quê:

- **Nada é persistido.** O certificado vem no payload e morre com a requisição.
- **Duas fases.** A fase 1 põe o documento nas mãos do cliente antes de qualquer
  byte sair; é isso que torna recuperável uma transmissão perdida. Não há atalho
  de uma chamada só.
- **Um módulo não importa outro módulo**, não conhece o servidor, e `platform/`
  não conhece domínio fiscal. Verificado por teste sobre o grafo de imports.
- **Campo desconhecido no JSON é 400.** Campo ignorado em silêncio vira documento
  transmitido com dado faltando.
- **Nada é reenviado automaticamente.** Retry cego duplica documento fiscal.

Discordar é legítimo — abra uma issue com o caso concreto. O que não ajuda é um
PR que contorna a regra sem tocar no raciocínio dela.

## Testes de lockstep

`internal/{cte,mdfe,nfse}/testdata/*.tsv` guardam as chaves de INI que a lib
aceita. Dois testes comparam em direções opostas:

1. a lib aceita e **não enviamos** → lacuna de cobertura;
2. **enviamos** e a lib não lê → chave morta, o dado é descartado em silêncio.

Falhar ali não é "conserte o código": é **decida** — passar a enviar, ou declarar
com o motivo. Foi um desses baselines que registrou por meses que o CT-e não
enviava `tpAmb`.

## Pull requests

- um assunto por PR;
- `make fmt-check vet test` verdes;
- teste que falharia sem a sua mudança;
- comentário explicando **por que**, não o que — o código já diz o que.

Domínio e comentários em **português**: o domínio é brasileiro e os termos não
têm tradução útil.
