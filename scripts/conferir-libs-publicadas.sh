#!/bin/sh
# Confere que o release das libs existe e é da revisão pinada. Sem token: o
# repositório é público.
set -eu

REPO="${ACBR_REPO:-4devsmart/wrapper-api}"
: "${ACBR_REV:?defina ACBR_REV}"
: "${ACBR_LIBS_RELEASE:?defina ACBR_LIBS_RELEASE}"

revisao="$(curl -fsSL --retry 3 "https://github.com/$REPO/releases/download/$ACBR_LIBS_RELEASE/revisao.txt" | tr -d '[:space:]')" || {
	echo "O release $ACBR_LIBS_RELEASE não tem as libs. Rode 'make acbr-libs-publicar' primeiro." >&2
	exit 1
}
if [ "$revisao" != "$ACBR_REV" ]; then
	echo "O release $ACBR_LIBS_RELEASE tem a r$revisao, e o Makefile pede a r$ACBR_REV." >&2
	exit 1
fi
echo "libs r$ACBR_REV publicadas em $ACBR_LIBS_RELEASE"
