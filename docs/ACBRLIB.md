# As libs nativas (ACBrLib)

O serviço é um wrapper: quem fala com a SEFAZ e com as prefeituras é a
**ACBrLib**, uma biblioteca nativa em Free Pascal com ABI C, na variante
**MultiThread (MT)**.

## Por que elas não estão no repositório

São ~56 MB de binário. Versioná-las faria todo mundo que clona para corrigir um
bug de INI baixar 56 MB que não vai nem abrir. Elas são **anexos de release**, e
`acbr-libs/` é cache local (está no `.gitignore`).

```bash
make acbr-libs-baixar          # popula acbr-libs/ a partir do release
docker compose up -d --build
```

## Plataforma

**`linux/amd64`, sempre.** A ACBrLib só é distribuída e compilada para x86_64.
Em hosts ARM tudo roda emulado.

## O que a imagem precisa ter, e por quê

Cada item abaixo custou uma investigação. Estão no `Dockerfile`:

- **provider `legacy` do OpenSSL.** Certificados A1 ICP-Brasil são PKCS#12 com
  algoritmos legados (RC2/3DES/SHA1) que o OpenSSL 3 desabilita por padrão. Sem
  ele, **nenhum** certificado carrega: o erro é
  `digital envelope routines::unsupported`, que não diz nada sobre isso.
- **symlinks sem versão para libxml2/libxmlsec1.** O wrapper LibXmlSec do ACBr
  faz `dlopen` por nomes sem versão (`libxml2.so`); o Debian só instala os
  versionados. Sem os links, `CarregarXML` e a **assinatura** falham.
- **GTK2 + Xvfb.** A lib é compilada com widgetset GTK2 (o FortesReport
  rasteriza o DANFSE/DACTE) e chama `gtk_init` já no carregamento: precisa de um
  `$DISPLAY` válido mesmo para operações que não imprimem. Por isso o Xvfb sobe
  no worker, que é o único processo que abre o `.so`. A variante `nogui` **não**
  serve: ela não rasteriza, e o TBitmap nulo vira SIGSEGV.
- **cache de fontes gravável** (`XDG_CACHE_HOME`). Sem ele o fontconfig reclama e
  o render sai errado.

## Compilando do fonte

A fonte oficial é o **SVN do ACBr, `trunk2`**, pinado por revisão:

```
https://svn.code.sf.net/p/acbr/code/trunk2
```

**Nunca use o mirror do GitHub**: ele fica meses atrasado, e a tabela de
provedor por município da NFS-e muda rápido.

São três comandos, e o build em si é **offline**: o download e a compilação são
etapas separadas de propósito, para que compilar de novo não dependa do
SourceForge estar de pé.

```bash
make acbr-fonte      # SVN do ACBr + git do FortesReport em acbr-source/ (~640 MB)
make acbr-compilar   # as 5 .so e os schemas, num container (LENTO, ~10 min)
make acbr-extrair    # copia os artefatos da imagem para acbr-libs/
```

`acbr-source/` é cache local e está no `.gitignore`. O `acbr-fonte` é retomável:
o SVN do SourceForge derruba conexão em checkout grande, e o script tem retry.

### O que é pinado, e o que não é

Duas entradas entram no binário, e **as duas** precisam de pino:

| entrada | pino | onde |
|---|---|---|
| fonte do ACBr | revisão do SVN | `ACBR_REV` |
| Fortes Report CE | commit do git | `ACBR_FRCE_REF` |

O FortesReport rasteriza o DACTE, o DAMDFE e o DANFSE, então ele está dentro
dos `.so` tanto quanto o ACBr. Ele seguia o `master`, o que fazia metade do
binário flutuar enquanto o ACBr era pinado com cuidado: uma recompilação em
2026-08 pegou um FortesReport de **2026-08-06** onde a anterior tinha usado o de
**2025-09-22**. Quase um ano de diferença, sem nada apontando.

Hoje o script **recusa rodar** sem `FRCE_REF`, em vez de cair no `master`: build
irreprodutível é pior que build que não roda, porque a divergência só aparece
meses depois, num binário diferente do que foi testado.

Fica de fora do pino o que vem do Debian: a imagem base `bookworm-slim` e os
pacotes `fpc`/`lazarus` do builder. Dentro de uma mesma release do Debian a
variação é pequena, mas ela existe, e é o que sobra entre "reprodutível" e
"idêntico bit a bit".

## O ciclo do release

As libs entram e saem do repositório por dois comandos, e os dois passam por
`acbr-libs/`:

| ponta | comando | quem roda |
|---|---|---|
| publicar | `make acbr-libs-publicar` | quem mantém, uma vez por revisão |
| consumir | `make acbr-libs-baixar` | qualquer clone, e os jobs `cgo` e `publicar` do CI |

`ACBR_REV`, no `Makefile`, é a **fonte única** da revisão. Dela sai o nome da
tag (`acbrlib-r<REV>`), e é dela que o download monta a URL. Trocar a revisão
num lugar só é o que mantém as duas pontas em acordo.

São sete arquivos: os cinco `.so`, o `schemas.tar.gz` e o `SHA256SUMS`.

### Publicando uma revisão nova

1. `make acbr-fonte && make acbr-compilar && make acbr-extrair`, com `ACBR_REV`
   e `ACBR_FRCE_REF` já apontando para o que você quer. Os seis arquivos ficam
   em `acbr-libs/`.
2. `make acbr-libs-publicar`. Ele gera o `SHA256SUMS` a partir do que está no
   diretório, cria a tag `acbrlib-r<REV>` e o release, e sobe os sete arquivos.
   Rodar de novo na mesma tag **atualiza** os anexos em vez de falhar.
3. Suba o `ACBR_REV` no `Makefile`.
4. Regenere o snapshot de chaves do lockstep
   (`internal/{cte,mdfe,nfse}/testdata/lerini_chaves.tsv`). **Não há script
   aqui**: ele sai dos procedimentos de leitura de INI do fonte, que não vem no
   clone, então é trabalho manual a partir da árvore que você acabou de
   compilar. É o que denuncia chave que a lib passou a aceitar e nós não
   enviamos, ou que enviamos e ela ignora em silêncio.
5. Rode `make enums-conferir`, que compara os valores publicados com os XSD do
   pacote de schemas. É por ali que uma nota técnica que mexe em código
   aparece.
6. Commite. Quem clonar depois disso baixa a revisão nova.

O `SHA256SUMS` é gerado na publicação e conferido no download **antes** de os
arquivos entrarem em `acbr-libs/`: download truncado que chega ao diretório vira
`.so` corrompido embutido na imagem, e SIGSEGV em runtime longe da causa. Como o
repositório é público, o download não usa token.

### Na estreia do repositório

Isto acontece uma vez só, e a ordem intuitiva não funciona nas duas direções.

Um repositório recém-criado não tem commit, e **um release precisa de um commit
para ancorar a tag**. Publicar antes de empurrar falha com um 422 do GitHub que
só diz `Repository is empty.`

Empurrar a `main` primeiro resolve isso e cria o problema oposto: os jobs `cgo`
e `publicar` do CI baixam justamente esses `.so`, então o primeiro run do
repositório nasce vermelho, no commit inicial.

A saída é empurrar só a **tag**. Ela popula o repositório sem acordar o Actions,
porque gatilho nenhum casa com ela:

| workflow | gatilho | tag `acbrlib-r*` |
|---|---|---|
| `ci.yml` | `branches: ["**"]` | não, só push de branch |
| `publicar.yml` | `branches: [main]` | não |
| `release.yml` | `tags: ["v*"]` | não casa o padrão |

```bash
git tag acbrlib-r47859 main && git push origin acbrlib-r47859
make acbr-libs-publicar
git push -u origin main
```

Assim a `main` estreia com o CI verde. O `scripts/publicar-acbr-libs.sh` detecta
o repositório vazio antes de chamar o `gh` e imprime essas três linhas, com a
revisão já preenchida.

## Licença

A ACBrLib é **LGPL**. Redistribuímos os binários com atribuição, a revisão do
fonte pinada e instruções de recompilação: ver [NOTICE](../NOTICE). O `.so` é
substituível: quem quiser trocar a lib por outra compilação sua só precisa
colocá-la em `acbr-libs/`.
