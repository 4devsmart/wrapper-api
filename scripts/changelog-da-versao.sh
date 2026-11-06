#!/bin/sh
# Imprime a seção de uma versão do CHANGELOG.md. Sem seção, sai com erro: é o que
# impede o release de promover uma versão sem registro.
#
# Uso: scripts/changelog-da-versao.sh v1.7.0 [arquivo]
set -eu

versao="$1"
arquivo="${2:-CHANGELOG.md}"

secao="$(awk -v titulo="## $versao" '
  $0 == titulo || index($0, titulo " ") == 1 { dentro = 1; next }
  /^## / { dentro = 0 }
  dentro
' "$arquivo")"

if [ -z "$(printf '%s' "$secao" | tr -d '[:space:]')" ]; then
  echo "$arquivo não tem seção para $versao" >&2
  exit 1
fi
printf '%s\n' "$secao"
