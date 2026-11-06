#!/bin/sh
# Coleta, do SVN oficial, o que scripts/acbr-analise.py compara. Só lê: nada aqui
# escreve no repositório nem em acbr-source. Roda DENTRO de um container com svn
# e patch, porque o host não tem svn.
#
# /saida recebe o bruto, /patches traz docker/acbr-patches.
set -eu

: "${ACBR_REV:?defina ACBR_REV}"
: "${ALVO:=HEAD}"
URL="${ACBR_SVN:-https://svn.code.sf.net/p/acbr/code/trunk2}"
FLAGS="--non-interactive --trust-server-cert"

# O SVN do SourceForge derruba conexões à toa.
tenta() {
	n=0
	until "$@"; do
		n=$((n + 1))
		[ "$n" -ge 5 ] && return 1
		sleep 3
	done
}

tenta svn info $FLAGS -r "$ALVO" "$URL" >/saida/info.txt
REV_ALVO="$(sed -n 's/^Last Changed Rev: //p' /saida/info.txt)"
echo "$ACBR_REV" >/saida/rev-base.txt
echo "$REV_ALVO" >/saida/rev-alvo.txt

for caminho in \
	Fontes/ACBrDFe/ACBrNFSeX Fontes/ACBrDFe/ACBrCTe Fontes/ACBrDFe/ACBrMDFe \
	Fontes/ACBrBoleto Projetos/ACBrLib/Fontes/NFSe Projetos/ACBrLib/Fontes/CTe \
	Projetos/ACBrLib/Fontes/MDFe Projetos/ACBrLib/Fontes/Boleto Projetos/ACBrLib/Fontes/Comum; do
	nome="$(echo "$caminho" | tr '/' '_')"
	tenta svn log $FLAGS -v -r "$((ACBR_REV + 1)):$REV_ALVO" "$URL/$caminho" >"/saida/log_$nome.txt"
done

INI=Fontes/ACBrDFe/ACBrNFSeX/ACBrNFSeXServicos.ini
tenta svn cat $FLAGS "$URL/$INI@$ACBR_REV" >/saida/servicos-base.ini
tenta svn cat $FLAGS "$URL/$INI@$REV_ALVO" >/saida/servicos-alvo.ini

ESQUEMAS=Exemplos/ACBrDFe/Schemas/NFSe/PadraoNacional
tenta svn ls $FLAGS -R "$URL/$ESQUEMAS@$ACBR_REV" | grep '/$' >/saida/esquemas-base.txt || true
tenta svn ls $FLAGS -R "$URL/$ESQUEMAS@$REV_ALVO" | grep '/$' >/saida/esquemas-alvo.txt || true

# Cada patch contra as duas revisões dos arquivos que ele toca.
: >/saida/patches.txt
for patch in /patches/*.patch; do
	nome="$(basename "$patch")"
	for arquivo in $(LC_ALL=C sed -n 's#^+++ b/\([^[:space:]]*\).*#\1#p' "$patch"); do
		for lado in base alvo; do
			if [ "$lado" = base ]; then rev="$ACBR_REV"; else rev="$REV_ALVO"; fi
			mkdir -p "/tmp/$lado/$(dirname "$arquivo")"
			tenta svn cat $FLAGS "$URL/$arquivo@$rev" >"/tmp/$lado/$arquivo"
		done
		if cmp -s "/tmp/base/$arquivo" "/tmp/alvo/$arquivo"; then
			mudou=nao
		else
			mudou=sim
		fi
		if (cd /tmp/alvo && patch -p1 --dry-run --fuzz=0 --binary -s <"$patch" >/dev/null 2>&1); then
			aplica=sim
		else
			aplica=nao
		fi
		printf '%s\t%s\t%s\t%s\n' "$nome" "$arquivo" "$mudou" "$aplica" >>/saida/patches.txt
	done
done
