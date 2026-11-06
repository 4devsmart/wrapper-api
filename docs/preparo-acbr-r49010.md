# Preparo do update para a r49010

Ensaio completo do bump, feito em branch local e sem publicar nada. A `main`, as
`.so` publicadas e o `ACBR_REV` da `main` continuam na r48784. Apague este arquivo
no commit que aplicar o update.

## Por que atualizar

27 municípios divergem da tabela compilada na r48784. 19 foram para o Padrão
Nacional, entre eles Cuiabá, Aparecida de Goiânia, Cascavel, Novo Hamburgo, Marília
e Taubaté. O relatório completo sai de `make acbr-analise ALVO=49010`.

## O que este ensaio provou

- Os 4 patches aplicam sem fuzz, e os 4 continuam necessários.
- As `.so` compilam, os 4 bindings carregam, e `TestRaizDoXMLPorProvedor` passa nos
  119 municípios.
- **Em produção**, Padrão Nacional e Brasília pelo Ambiente Nacional saem com o XML
  idêntico ao da r48784.
- **Em homologação**, o Padrão Nacional muda: a NT 009 está ligada só lá
  (`HomVersaoNT=009.1.01`), com `finNFSe` novo e `xDescServ` em outra posição.

## O que o ensaio encontrou e já está corrigido nesta branch

- **`-g` no modo Linux da NFS-e (r48988):** a `.so` ia a 86 MB, com símbolos
  indefinidos, e o worker não linkava. O `acbrlib.Dockerfile` remove a opção.
- **CT-e, `gCompraGov`:** a NT 2026/004 moveu a leitura de `[ide]` para
  `[gCompraGov]`. O builder escrevia em `[ide]`, e o dado seria descartado em silêncio.
- **`gerar-chaves-lerini.py`:** passa a ler a classe base da Reforma, e aceita a
  seção com e sem índice.

## XML que muda nos dois ambientes

| Provedor | Mudança | Origem | O que fazer |
|---|---|---|---|
| Centi | `ItemListaServico` sai como enviado: `01.05`, antes `105` | ACBR-9943 | Testar em homologação |
| ISSBarueri | Código postal só para tomador estrangeiro | ACBR-10030 | Testar em homologação |
| eGoverneISS | Tags vazias da prestação deixam de sair | r48874 | Nenhuma |

## Antes de aplicar

1. Testar em homologação: emissão e cancelamento pelo Padrão Nacional, Brasília pelo
   Ambiente Nacional, Centi e ISSBarueri, e um CT-e de compra governamental.
2. Na NT 009, a lib recusa em homologação deduções, reembolsos e finalidade diferente
   de regular. Quem usa deduções passa a receber erro só lá.
3. Decidir os campos da NT 2026/004 do CT-e, hoje em `nao_enviadas.tsv`: `tpPagAnt`,
   `vTPrestLiq`, `vPrestLiq` e `vICMSPrevisto`.
4. Preencher `Desde` em `notasTecnicas` quando o Portal Nacional publicar a data da
   NT 009 em produção.

## Para publicar

1. Publicar as `.so` deste ensaio ou recompilar, e subir `ACBR_LIBS_RELEASE`.
2. Escrever a seção da versão no `CHANGELOG.md`, listando os 27 municípios.
3. Apagar este arquivo.

## Reportar ao ACBr

O cabeçalho de cada patch em `docker/acbr-patches/` já traz sintoma e correção.
Falta acrescentar o `-g` no `CustomOptions` do modo `Linux-x86_64-MT` de
`ACBrLibNFSe.lpi`, que só existe nesse projeto.
