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

A revisão em uso está em `ACBR_REV`, no `Makefile`. Ao bumpar, regenere também o
snapshot de chaves do lockstep (`internal/{cte,mdfe,nfse}/testdata/`): é ele que
denuncia campo que a lib passou a aceitar e nós não enviamos, ou que escrevemos
e ela ignora em silêncio.

## Licença

A ACBrLib é **LGPL**. Redistribuímos os binários com atribuição, a revisão do
fonte pinada e instruções de recompilação: ver [NOTICE](../NOTICE). O `.so` é
substituível: quem quiser trocar a lib por outra compilação sua só precisa
colocá-la em `acbr-libs/`.
