#!/usr/bin/env bash
# Publica o cache local (acbr-libs/) como anexos de um release, para que
# scripts/baixar-acbr-libs.sh possa consumi-lo.
#
# Rode UMA VEZ por revisão do ACBr, depois de compilar as libs do fonte. Exige o
# gh CLI autenticado.
set -euo pipefail

REPO="${ACBR_REPO:-4devsmart/wrapper-api}"
REV="${ACBR_REV:?defina ACBR_REV (o Makefile passa)}"
ORIGEM="${ACBR_LIBS_DIR:-acbr-libs}"
TAG="acbrlib-r${REV}"

ARQUIVOS=(
  libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so
  libacbrnfe64.so libacbrboleto64.so schemas.tar.gz
)

for f in "${ARQUIVOS[@]}"; do
  [[ -s "$ORIGEM/$f" ]] || { echo "ERRO: falta $ORIGEM/$f" >&2; exit 1; }
done

# O SHA256SUMS é gerado aqui e vai junto: é ele que o download verifica.
(cd "$ORIGEM" && sha256sum "${ARQUIVOS[@]}" > SHA256SUMS)
echo "checksums:"; sed 's/^/  /' "$ORIGEM/SHA256SUMS"

if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "release $TAG já existe — subindo/atualizando anexos"
  gh release upload "$TAG" --repo "$REPO" --clobber \
    "${ARQUIVOS[@]/#/$ORIGEM/}" "$ORIGEM/SHA256SUMS"
else
  gh release create "$TAG" --repo "$REPO" \
    --title "ACBrLib r${REV}" \
    --notes "Binários da ACBrLib (variante MT, linux/amd64) compilados do SVN oficial trunk2, revisão ${REV}.

Licenciados sob LGPL — ver NOTICE. Consumidos por \`make acbr-libs-baixar\`.

Este release existe para que os ~56 MB de binário fiquem FORA do clone." \
    "${ARQUIVOS[@]/#/$ORIGEM/}" "$ORIGEM/SHA256SUMS"
fi
echo "publicado: https://github.com/${REPO}/releases/tag/${TAG}"
