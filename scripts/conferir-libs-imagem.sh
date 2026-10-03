#!/usr/bin/env bash
# Confere que as .so DENTRO de uma imagem são as do SHA256SUMS de um cache de
# libs (acbr-libs/, populado por `make acbr-libs-baixar` a partir do release em
# ACBR_LIBS_RELEASE).
#
# Existe porque a imagem v1.3.0 saiu com a lib de NFS-e oficial, sem os patches
# de docker/acbr-patches, enquanto o próprio release carregava as patchadas.
# Nada falhou: o Dockerfile copia o que estiver em acbr-libs/, e ninguém
# comparava o resultado. A fumaça prova que a lib carrega, não qual lib é.
#
# Uso:  scripts/conferir-libs-imagem.sh <imagem> [diretório-do-cache]
set -euo pipefail

IMAGEM="${1:?uso: $0 <imagem> [diretório-do-cache]}"
CACHE="${2:-acbr-libs}"
SOS=(libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so libacbrnfe64.so libacbrboleto64.so)

if [[ ! -f "$CACHE/SHA256SUMS" ]]; then
  echo "ERRO: $CACHE/SHA256SUMS não existe. Rode 'make acbr-libs-baixar'." >&2
  exit 1
fi

# --entrypoint direto no sha256sum: o entrypoint da imagem sobe a API ou o
# worker, e aqui só interessa ler os arquivos.
caminhos=("${SOS[@]/#//usr/lib/}")
na_imagem="$(docker run --rm --entrypoint sha256sum "$IMAGEM" "${caminhos[@]}")"

divergiu=0
for so in "${SOS[@]}"; do
  esperado="$(awk -v f="$so" '$2 == f || $2 == "*"f {print $1}' "$CACHE/SHA256SUMS")"
  obtido="$(awk -v f="/usr/lib/$so" '$2 == f {print $1}' <<<"$na_imagem")"
  if [[ -z "$esperado" ]]; then
    echo "ERRO: $so não está em $CACHE/SHA256SUMS" >&2
    divergiu=1
  elif [[ "$esperado" != "$obtido" ]]; then
    echo "ERRO: $so na imagem é ${obtido:0:16}, o release declara ${esperado:0:16}" >&2
    divergiu=1
  else
    echo "  $so ${obtido:0:16}"
  fi
done

if [[ "$divergiu" == 1 ]]; then
  echo "ERRO: $IMAGEM não carrega as libs de $CACHE/SHA256SUMS." >&2
  exit 1
fi
echo "libs da imagem conferem com $CACHE/SHA256SUMS"
