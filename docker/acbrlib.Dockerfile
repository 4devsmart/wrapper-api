# Builder das ACBrLib MT (NFSe/CTe/MDFe) + schemas, a partir do CACHE LOCAL dos
# fontes (./acbr-source, populado por `make acbr-fonte` via client SVN/git em
# container). Este build é OFFLINE: só compila, e o contexto é ./acbr-source.
# O DANFSE/DACTE/DAMDFE usa Fortes Report CE (frce, no cache).
#
#   make acbr-fonte     # busca/atualiza ./acbr-source (svn pinado + git)
#   make acbr-compilar  # docker build -f docker/acbrlib.Dockerfile acbr-source
#
# Plataforma fixa linux/amd64 (a ACBrLib é x86_64).

FROM --platform=linux/amd64 debian:bookworm-slim AS builder

ENV DEBIAN_FRONTEND=noninteractive
# UTF-8 é obrigatório: o trunk2 tem arquivos com acento; sem locala UTF-8 o
# svn falha com "E000022: Can't convert string ... to native encoding".
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

# Revisão dos fontes (apenas informativo aqui; o checkout real é feito por
# `make acbr-fonte`, que popula ./acbr-source, o contexto deste build).
ARG ACBR_REV=47859

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates make \
        fpc lazarus lcl-nogui \
        libssl-dev libxml2-dev \
        libgtk2.0-dev libcups2-dev libcairo2-dev \
    && rm -rf /var/lib/apt/lists/*

# Descobre a versão do Lazarus instalada (para --lazarusdir).
RUN echo "lazbuild: $(which lazbuild)"; lazbuild --version || true

WORKDIR /src
# Fontes do CACHE LOCAL (contexto = ./acbr-source, populado por `make
# acbr-fonte`). Sem SVN/git aqui: o build é offline e determinístico. O
# .dockerignore do contexto exclui .svn/.git.
COPY acbr /src/acbr
COPY frce /src/frce

# Registra TODOS os .lpk necessários como package links do lazbuild:
#   - Fortes Report CE (frce + cairo_canvas, se houver)
#   - pacotes Lazarus do ACBr
RUN set -eux; \
    find /src/frce -name '*.lpk' -exec lazbuild --add-package-link {} \; ; \
    find /src/acbr/Pacotes/Lazarus -name '*.lpk' -exec lazbuild --add-package-link {} \; ; \
    # Printer4Lazarus (componente do Lazarus), necessário ao Fortes no Linux.
    find /usr/lib/lazarus -iname 'printer4lazarus.lpk' -exec lazbuild --add-package-link {} \; || true

# O ACBr embute recursos .rc (ex.: ACBrConsultaCPFServicos.rc, ACBrNFeServicos.rc).
# O compilador de .rc padrão do FPC é o 'windres' (inexistente no Linux nativo).
# O 'fpcres' (já incluso no FPC) compila .rc, mas o FPC o invoca com sintaxe
# windres (-O em vez de -of, e --preprocessor que o fpcres não conhece). Solução:
# um shim chamado 'windres' no PATH que traduz os argumentos para o fpcres.
RUN set -eux; command -v fpcres; \
    printf '%s\n' \
      '#!/bin/sh' \
      'set -eu' \
      'args=""' \
      'while [ "$#" -gt 0 ]; do' \
      '  case "$1" in' \
      '    --preprocessor=*) : ;;' \
      '    --preprocessor)   shift ;;' \
      '    -O)               shift; args="$args -of $1" ;;' \
      '    -O*)              args="$args -of ${1#-O}" ;;' \
      '    *)                args="$args $1" ;;' \
      '  esac' \
      '  shift' \
      'done' \
      'exec fpcres $args' \
      > /usr/local/bin/windres; \
    chmod +x /usr/local/bin/windres; \
    /usr/local/bin/windres --version >/dev/null 2>&1 || true

# Widgetset GTK2 do LCL (camada tardia: não invalida o cache do checkout SVN).
# Necessário para o FortesReport rasterizar o DANFSE (o nogui não tem backend
# gráfico, TBitmap nulo, SIGSEGV). Em runtime, roda sob Xvfb (display virtual).
RUN apt-get update && apt-get install -y --no-install-recommends lcl-gtk2 \
    && rm -rf /var/lib/apt/lists/*

# Compila a variante MultiThread para Linux x86_64 com widgetset GTK2.
ARG LIB_DIR=/src/acbr/Projetos/ACBrLib/Fontes/NFSe
RUN set -eux; cd "${LIB_DIR}"; \
    lazbuild --widgetset=gtk2 --build-mode="Linux-x86_64-MT" ACBrLibNFSe.lpi; \
    mkdir -p /artifacts; \
    find "${LIB_DIR}" -name 'libacbrnfse*.so' -o -name 'acbrnfse*.so' | xargs -I{} cp -v {} /artifacts/; \
    ls -la /artifacts

# CT-e (modelo 57) e MDF-e (modelo 58): mesma variante MT + widgetset GTK2 (o
# DACTE/DAMDFE também usa FortesReport). Os fontes da lib já vieram no checkout
# (Projetos/ACBrLib/Fontes). Cada um gera seu próprio .so.
RUN set -eux; cd /src/acbr/Projetos/ACBrLib/Fontes/CTe; \
    lazbuild --widgetset=gtk2 --build-mode="Linux-x86_64-MT" ACBrLibCTe.lpi; \
    find . -name 'libacbrcte*.so' -o -name 'acbrcte*.so' | xargs -I{} cp -v {} /artifacts/
RUN set -eux; cd /src/acbr/Projetos/ACBrLib/Fontes/MDFe; \
    lazbuild --widgetset=gtk2 --build-mode="Linux-x86_64-MT" ACBrLibMDFe.lpi; \
    find . -name 'libacbrmdfe*.so' -o -name 'acbrmdfe*.so' | xargs -I{} cp -v {} /artifacts/; \
    ls -la /artifacts

# NF-e (modelo 55): mesma variante MT + GTK2. Usada APENAS para Distribuição DF-e
# (receber NF-e contra o CNPJ) e Manifestação do Destinatário, não para emissão.
RUN set -eux; cd /src/acbr/Projetos/ACBrLib/Fontes/NFe; \
    lazbuild --widgetset=gtk2 --build-mode="Linux-x86_64-MT" ACBrLibNFe.lpi; \
    find . -name 'libacbrnfe*.so' -o -name 'acbrnfe*.so' | xargs -I{} cp -v {} /artifacts/; \
    ls -la /artifacts

# Boleto (não-fiscal): geração de boleto/PDF (FortesReport → GTK2) + CNAB. Mesma
# variante MT. Agnóstico ao banco: o banco é só config ([Banco] TipoCobranca).
RUN set -eux; cd /src/acbr/Projetos/ACBrLib/Fontes/Boleto; \
    lazbuild --widgetset=gtk2 --build-mode="Linux-x86_64-MT" ACBrLibBoleto.lpi; \
    find . -name 'libacbrboleto*.so' -o -name 'acbrboleto*.so' | xargs -I{} cp -v {} /artifacts/; \
    ls -la /artifacts

# Schemas XSD (do cache local, já no contexto): NFSe (multi-provedor), CT-e, MDF-e, NF-e.
RUN set -eux; \
    mkdir -p /artifacts/schemas /artifacts/schemas-cte /artifacts/schemas-mdfe /artifacts/schemas-nfe; \
    cp -r /src/acbr/Exemplos/ACBrDFe/Schemas/NFSe/. /artifacts/schemas/; \
    cp -r /src/acbr/Exemplos/ACBrDFe/Schemas/CTe/.  /artifacts/schemas-cte/; \
    cp -r /src/acbr/Exemplos/ACBrDFe/Schemas/MDFe/. /artifacts/schemas-mdfe/; \
    # NF-e: o trunk2 não distribui os XSD de NF-e no diretório Schemas. Distribuição
    # DF-e e Manifestação NÃO exigem XSD local (a SEFAZ valida no servidor); o dir
    # fica vazio. Tolerante para não quebrar o build se a pasta não existir.
    if [ -d /src/acbr/Exemplos/ACBrDFe/Schemas/NFe ]; then \
        cp -r /src/acbr/Exemplos/ACBrDFe/Schemas/NFe/. /artifacts/schemas-nfe/; \
    fi; \
    echo "schemas nfse:$(ls /artifacts/schemas | wc -l) cte:$(ls /artifacts/schemas-cte | wc -l) mdfe:$(ls /artifacts/schemas-mdfe | wc -l) nfe:$(ls /artifacts/schemas-nfe | wc -l)"

# Estágio final: só os artefatos.
FROM --platform=linux/amd64 debian:bookworm-slim AS artifacts
COPY --from=builder /artifacts/ /artifacts/
