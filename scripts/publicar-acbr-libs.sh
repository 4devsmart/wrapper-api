#!/usr/bin/env bash
# Publica o cache local (acbr-libs/) como anexos de um release, para que
# scripts/baixar-acbr-libs.sh possa consumi-lo.
#
# Rode UMA VEZ por revisão do ACBr, depois de compilar as libs do fonte. Exige o
# gh CLI autenticado.
#
# ORDEM, na estreia do repositório: um release precisa de um commit para ancorar
# a tag, então NÃO dá para publicar as libs antes de empurrar alguma coisa. E
# empurrar a main primeiro estreia o CI em vermelho, porque o job cgo baixa
# justamente estes .so. O caminho sem os dois problemas é empurrar só a TAG, que
# não casa com gatilho nenhum (ci.yml e publicar.yml pedem branch, release.yml
# pede v*):
#
#   git tag acbrlib-rNNNNN main && git push origin acbrlib-rNNNNN
#   make acbr-libs-publicar
#   git push -u origin main
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

# Sem nenhum ref no repositório, o gh falha com um 422 "Repository is empty" que
# não diz o que fazer. Antecipa com a saída acionável.
if [[ "$(gh repo view "$REPO" --json isEmpty --jq .isEmpty 2>/dev/null)" == "true" ]]; then
  echo "ERRO: $REPO está vazio, e um release precisa de um commit para ancorar a tag." >&2
  echo "      Empurre a tag primeiro (ela não dispara workflow nenhum):" >&2
  echo >&2
  echo "        git tag $TAG main && git push origin $TAG" >&2
  echo "        make acbr-libs-publicar" >&2
  echo "        git push -u origin main" >&2
  exit 1
fi

if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "release $TAG já existe: subindo/atualizando anexos"
  gh release upload "$TAG" --repo "$REPO" --clobber \
    "${ARQUIVOS[@]/#/$ORIGEM/}" "$ORIGEM/SHA256SUMS"
  # Idempotente, e corrige release antigo criado antes destas flags.
  gh release edit "$TAG" --repo "$REPO" --prerelease --latest=false >/dev/null
else
  # --prerelease e --latest=false: este release é um pacote de binários de
  # terceiro, não uma versão deste serviço. Sem as duas flags o GitHub marca o
  # mais recente como "Latest", e a aba de Releases passa a anunciar
  # "ACBrLib r48100" no lugar da v1.0.0 para quem abre o repositório. Aconteceu
  # em 2026-08 e de novo em 2026-09; das duas vezes foi corrigido à mão.
  # Versão deste serviço é tag v*, em versionamento semântico.
  gh release create "$TAG" --repo "$REPO" --prerelease --latest=false \
    --title "ACBrLib r${REV}" \
    --notes "Binários da ACBrLib (variante MT, linux/amd64) compilados do SVN oficial trunk2, revisão ${REV}.

Licenciados sob LGPL: ver NOTICE. Consumidos por \`make acbr-libs-baixar\`.

Este release existe para que os ~56 MB de binário fiquem FORA do clone." \
    "${ARQUIVOS[@]/#/$ORIGEM/}" "$ORIGEM/SHA256SUMS"
fi
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
