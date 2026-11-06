#!/usr/bin/env bash
# Publica o cache local (acbr-libs/) como anexos de um release, para que
# scripts/baixar-acbr-libs.sh possa consumi-lo.
#
# Rode UMA VEZ por revisão do ACBr, depois de compilar as libs do fonte e ANTES do
# commit que sobe ACBR_REV: o `make finish` e o publicar.yml baixam deste release, e
# a revisão nova não passa sem ele. Exige o gh CLI autenticado.
#
#   make acbr-fonte acbr-compilar acbr-extrair ACBR_REV=49010
#   make acbr-libs-publicar ACBR_REV=49010 ACBR_LIBS_RELEASE=acbrlib-r49010
#
set -euo pipefail

REPO="${ACBR_REPO:-4devsmart/wrapper-api}"
REV="${ACBR_REV:?defina ACBR_REV (o Makefile passa)}"
ORIGEM="${ACBR_LIBS_DIR:-acbr-libs}"
# Release próprio da revisão, acbrlib-rNNNNN: a lib não depende de versão do wrapper
# existir, e várias versões usam as mesmas libs. Sai como pré-release para nunca
# levar o selo "Latest", que é das versões do serviço.
TAG="${ACBR_LIBS_RELEASE:?defina ACBR_LIBS_RELEASE (o Makefile passa)}"

ARQUIVOS=(
  libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so
  libacbrnfe64.so libacbrboleto64.so schemas.tar.gz
)

for f in "${ARQUIVOS[@]}"; do
  [[ -s "$ORIGEM/$f" ]] || { echo "ERRO: falta $ORIGEM/$f" >&2; exit 1; }
done

# O SHA256SUMS é gerado aqui e vai junto: é ele que o download verifica. O
# revisao.txt amarra os binários ao ACBR_REV, já que a tag do release não
# carrega mais o número da revisão.
(cd "$ORIGEM" && sha256sum "${ARQUIVOS[@]}" > SHA256SUMS)
echo "$REV" > "$ORIGEM/revisao.txt"
echo "checksums:"; sed 's/^/  /' "$ORIGEM/SHA256SUMS"

if ! gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  gh release create "$TAG" --repo "$REPO" --prerelease --latest=false \
    --title "ACBrLib r$REV" --notes "Binários da ACBrLib r$REV com os patches de docker/acbr-patches."
fi

gh release upload "$TAG" --repo "$REPO" --clobber \
  "${ARQUIVOS[@]/#/$ORIGEM/}" "$ORIGEM/SHA256SUMS" "$ORIGEM/revisao.txt"

echo "publicado: https://github.com/${REPO}/releases/tag/${TAG}"

# Confere o resultado em vez de confiar nas flags. A pergunta que importa para
# quem abre o repositório é qual release leva o selo "Latest": tem de ser uma
# tag v*, nunca um pacote da ACBrLib.
ultimo=$(gh release list --repo "$REPO" --limit 30 \
  --json tagName,isLatest --jq '.[] | select(.isLatest) | .tagName' 2>/dev/null || true)
case "$ultimo" in
  v*)
    echo "latest do repositório: $ultimo"
    ;;
  "")
    echo "AVISO: nenhum release marcado como latest" >&2
    ;;
  *)
    echo "ERRO: o latest do repositório é '$ultimo', e deveria ser uma tag v*." >&2
    echo "      Conserte com: gh release edit $ultimo --repo $REPO --prerelease --latest=false" >&2
    exit 1
    ;;
esac
