# Imagem do wrapper (multi-stage), com as ACBrLib nativas na variante MT.
#
# Os .so NÃO são versionados: vêm de acbr-libs/, que é cache local populado por
# `make acbr-libs-baixar` (assets de release) ou por compilação do fonte ACBr.
# Ver docs/ACBRLIB.md.
#
# Plataforma é sempre linux/amd64 — a ACBrLib só existe para x86_64.

# --- artefatos nativos ------------------------------------------------------
FROM --platform=linux/amd64 debian:bookworm-slim AS acbrlib
WORKDIR /artifacts
COPY acbr-libs/libacbrnfse64.so acbr-libs/libacbrcte64.so acbr-libs/libacbrmdfe64.so \
     acbr-libs/libacbrnfe64.so acbr-libs/libacbrboleto64.so ./
COPY acbr-libs/schemas.tar.gz /tmp/schemas.tar.gz
# Guard: um cache incompleto (download interrompido, ponteiro de LFS de um clone
# antigo) embutiria lixo na imagem e daria SIGSEGV em runtime, longe da causa.
# Falhar aqui custa segundos; falhar lá custa uma investigação.
RUN set -eu; \
    for f in libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so libacbrnfe64.so libacbrboleto64.so; do \
      if head -c 64 "$f" | grep -q 'git-lfs'; then \
        echo "ERRO: '$f' é um ponteiro Git LFS, não o binário." >&2; exit 1; fi; \
      if [ "$(stat -c%s "$f")" -lt 1000000 ]; then \
        echo "ERRO: '$f' tem $(stat -c%s "$f") bytes — cache incompleto." >&2; \
        echo "      Rode 'make acbr-libs-baixar' e rebuilde." >&2; exit 1; fi; \
    done
# NF-e não distribui XSD no trunk2 → schemas-nfe some no tar; garante o diretório
# para o COPY do runtime não falhar (distribuição/manifestação não exigem XSD).
RUN tar -xzf /tmp/schemas.tar.gz -C /artifacts && rm /tmp/schemas.tar.gz \
    && mkdir -p /artifacts/schemas-nfe

# --- build Go ---------------------------------------------------------------
FROM --platform=linux/amd64 golang:1.25.13-bookworm AS gobuild
WORKDIR /app

# Libs NEEDED pelo .so: o linker precisa delas para resolver as referências
# transitivas ao linkar o binário cgo.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libcairo2 libpango-1.0-0 libpangocairo-1.0-0 libglib2.0-0 libgtk2.0-0 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=acbrlib /artifacts/libacbrnfse64.so /artifacts/libacbrcte64.so \
     /artifacts/libacbrmdfe64.so /artifacts/libacbrnfe64.so /artifacts/libacbrboleto64.so /usr/lib/
RUN ldconfig

COPY go.mod ./
RUN go mod download
COPY . .

ARG APP_COMMIT=""
ARG APP_BUILD=""
ENV VERSAO_PKG=github.com/4devsmart/wrapper-api/internal/platform/versao

# A API compila SEM cgo e SEM a tag acbrlib: não linka a lib nativa e delega ao
# fiscal-worker por RPC. É o que impede um SIGSEGV na lib de derrubar o servidor.
# -buildvcs=false: o .git não vai no contexto e o estágio não tem git.
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false \
      -ldflags "-X ${VERSAO_PKG}.Commit=${APP_COMMIT} -X ${VERSAO_PKG}.Build=${APP_BUILD}" \
      -o /out/api ./cmd/api
# fiscal-worker: o ÚNICO binário com cgo + a lib nativa.
RUN CGO_ENABLED=1 go build -tags acbrlib -trimpath -buildvcs=false \
      -ldflags "-X ${VERSAO_PKG}.Commit=${APP_COMMIT} -X ${VERSAO_PKG}.Build=${APP_BUILD}" \
      -o /out/fiscal-worker ./cmd/fiscal-worker

# --- runtime ----------------------------------------------------------------
FROM --platform=linux/amd64 debian:bookworm-slim AS runtime
# upgrade -y aplica patches do SO (gtk2/glib puxam libs com CVE corrigida).
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends \
        ca-certificates libssl3 curl \
        libxml2 libxslt1.1 libxmlsec1 libxmlsec1-openssl \
        libcairo2 libpango-1.0-0 libpangocairo-1.0-0 libglib2.0-0 \
        fontconfig fonts-dejavu-core fonts-liberation \
        libgtk2.0-0 xvfb xauth \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 10001 app

# O wrapper LibXmlSec do ACBr faz dlopen por nomes SEM versão (libxml2.so,
# libxmlsec1.so…). O Debian só instala os versionados. Sem estes symlinks,
# CarregarXML e a ASSINATURA falham.
RUN set -eux; \
    for lib in libxml2 libxslt libexslt libxmlsec1 libxmlsec1-openssl; do \
      target="$(ldconfig -p | grep -oE "/[^ ]*${lib}\.so\.[0-9]+" | head -1)"; \
      [ -n "$target" ] && ln -sf "$target" "/usr/lib/x86_64-linux-gnu/${lib}.so" || true; \
    done; \
    ldconfig

# Certificados A1 ICP-Brasil são PKCS#12 com algoritmos legados (RC2/3DES/SHA1)
# que o OpenSSL 3 desabilita por padrão ("digital envelope routines::unsupported").
# Sem o provider 'legacy', NENHUM certificado carrega.
RUN printf '%s\n' \
      'openssl_conf = openssl_init' \
      '[openssl_init]' \
      'providers = provider_sect' \
      '[provider_sect]' \
      'default = default_sect' \
      'legacy = legacy_sect' \
      '[default_sect]' \
      'activate = 1' \
      '[legacy_sect]' \
      'activate = 1' \
      > /etc/ssl/openssl-legacy.cnf
ENV OPENSSL_CONF=/etc/ssl/openssl-legacy.cnf

# Cache de fontes gravável pelo não-root (fontconfig do FortesReport). Sem isto:
# "Fontconfig error: No writable cache directories".
ENV XDG_CACHE_HOME=/tmp/.cache

COPY --from=acbrlib /artifacts/libacbrnfse64.so /artifacts/libacbrcte64.so \
     /artifacts/libacbrmdfe64.so /artifacts/libacbrnfe64.so /artifacts/libacbrboleto64.so /usr/lib/
RUN ldconfig

COPY --from=acbrlib /artifacts/schemas/ /opt/acbr/schemas/
COPY --from=acbrlib /artifacts/schemas-cte/ /opt/acbr/schemas-cte/
COPY --from=acbrlib /artifacts/schemas-mdfe/ /opt/acbr/schemas-mdfe/
COPY --from=acbrlib /artifacts/schemas-nfe/ /opt/acbr/schemas-nfe/

COPY --from=gobuild /out/api /usr/local/bin/api
COPY --from=gobuild /out/fiscal-worker /usr/local/bin/fiscal-worker
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Socket RPC entre API e worker (único volume do stack — e não guarda dado).
RUN mkdir -p /run/wrapper && chown app:app /run/wrapper
# Socket do X (Xvfb) gravável pelo não-root.
RUN mkdir -p /tmp/.X11-unix && chmod 1777 /tmp/.X11-unix

USER app
EXPOSE 8080
# start-period curto: não há migration nem seed para esperar — o serviço não
# guarda estado, então subir é só abrir a porta.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
# O primeiro argumento escolhe o processo: "api" (default) ou "worker" — este
# sobe o Xvfb antes, por causa do GTK2/FortesReport. Mesma imagem nos dois
# serviços; o que muda é o comando.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["api"]
