#!/usr/bin/env bash
# Baixa as libs nativas da ACBrLib dos anexos de release e as coloca no cache
# local (acbr-libs/), verificando os checksums.
#
# Por que assets de release e não Git LFS: são ~56 MB de binário. Versioná-los
# faria todo mundo que clona para corrigir um bug de INI baixar 56 MB que não vai
# nem abrir, e num repositório público o LFS ainda consumiria cota de banda do
# dono a cada download de terceiro. Como o repositório é público, aqui não há
# token: é uma URL simples.
set -euo pipefail

REPO="${ACBR_REPO:-4devsmart/wrapper-api}"
REV="${ACBR_REV:?defina ACBR_REV (o Makefile passa)}"
DESTINO="${ACBR_LIBS_DIR:-acbr-libs}"
# Os binários são anexos de um release v*, não de um release próprio: a aba de
# Releases tem só versões deste serviço. Como a tag não carrega mais o número da
# revisão, o revisao.txt é o que amarra os .so ao ACBR_REV pinado, e ele é
# conferido antes dos checksums.
TAG="${ACBR_LIBS_RELEASE:?defina ACBR_LIBS_RELEASE (o Makefile passa)}"
BASE="https://github.com/${REPO}/releases/download/${TAG}"

ARQUIVOS=(
  libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so
  libacbrnfe64.so libacbrboleto64.so schemas.tar.gz
)

mkdir -p "$DESTINO"

# Cache quente: se tudo já bate com o SHA256SUMS local, não baixa nada.
if [[ -f "$DESTINO/SHA256SUMS" ]]; then
  if quebrados="$(cd "$DESTINO" && sha256sum -c --quiet SHA256SUMS 2>/dev/null)"; then
    if [[ "$(tr -d '[:space:]' < "$DESTINO/revisao.txt" 2>/dev/null)" == "$REV" ]]; then
      echo "acbr-libs: cache já íntegro para r${REV}: nada a baixar."
      exit 0
    fi
    echo "acbr-libs: cache íntegro mas de outra revisão, rebaixando"
  fi
  echo "acbr-libs: cache inválido, rebaixando:"
  sed 's/^/  · /' <<<"$quebrados"
fi

echo "acbr-libs: baixando r${REV} do release $TAG de $REPO"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

baixar() {
  local nome="$1"
  if ! curl -fsSL --retry 3 --retry-delay 2 -o "$tmp/$nome" "$BASE/$nome"; then
    echo "ERRO: falha ao baixar $nome de $BASE" >&2
    echo "      O release '$TAG' existe? Veja https://github.com/${REPO}/releases" >&2
    echo "      Para publicá-lo a partir de um cache local: make acbr-libs-publicar" >&2
    exit 1
  fi
}

for f in "${ARQUIVOS[@]}"; do
  echo "  · $f"
  baixar "$f"
done
baixar SHA256SUMS
baixar revisao.txt

# A tag do release não diz mais qual revisão do ACBr está ali dentro. Sem esta
# conferência, apontar ACBR_LIBS_RELEASE para um release de outra revisão traria
# .so de fonte diferente do pinado, com os checksums batendo, que é a
# divergência silenciosa que o pino existe para evitar.
publicada="$(tr -d '[:space:]' < "$tmp/revisao.txt")"
if [[ "$publicada" != "$REV" ]]; then
  echo "ERRO: o release '$TAG' carrega a revisão r${publicada}, e ACBR_REV pina r${REV}." >&2
  echo "      Aponte ACBR_LIBS_RELEASE para o release da revisão certa," >&2
  echo "      ou publique esta revisão com: make acbr-libs-publicar" >&2
  exit 1
fi

# Verificar ANTES de mover: um download truncado que chega em acbr-libs/ vira
# .so corrompido embutido na imagem e SIGSEGV em runtime, longe da causa.
echo "acbr-libs: conferindo checksums"
if ! (cd "$tmp" && sha256sum -c --quiet SHA256SUMS); then
  echo "ERRO: checksum não confere: download corrompido ou release adulterado." >&2
  exit 1
fi

mv "$tmp"/* "$DESTINO"/
chmod +x "$DESTINO"/*.so
echo "acbr-libs: pronto em $DESTINO (revisão $REV, release $TAG)"
