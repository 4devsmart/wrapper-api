#!/usr/bin/env bash
# Publica o cache local (acbr-libs/) como anexos de um release, para que
# scripts/baixar-acbr-libs.sh possa consumi-lo.
#
# Rode UMA VEZ por revisão do ACBr, depois de compilar as libs do fonte, e
# depois de a tag da versão existir. Exige o gh CLI autenticado.
#
# ORDEM: o release precisa existir antes dos anexos. Numa revisão nova do ACBr:
#
#   1. make acbr-fonte && make acbr-compilar && make acbr-extrair
#   2. suba ACBR_REV e ACBR_LIBS_RELEASE no Makefile, e mescle
#   3. git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z
#   4. make acbr-libs-publicar
#
set -euo pipefail

REPO="${ACBR_REPO:-4devsmart/wrapper-api}"
REV="${ACBR_REV:?defina ACBR_REV (o Makefile passa)}"
ORIGEM="${ACBR_LIBS_DIR:-acbr-libs}"
# Anexa a um release v* já existente, criado pelo release.yml ao empurrar a tag.
# Não cria release próprio: a aba de Releases tem só versões deste serviço.
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
  echo "ERRO: o release '$TAG' não existe." >&2
  echo "      Ele é criado pelo workflow release.yml ao empurrar a tag:" >&2
  echo >&2
  echo "        git tag -a $TAG -m $TAG && git push origin $TAG" >&2
  echo >&2
  echo "      Depois rode este comando de novo para anexar os binários." >&2
  exit 1
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
