#!/bin/sh
# Atualiza o cache local dos fontes do ACBr (SVN oficial, pinado por revisão) e
# do FortesReport CE (git) em /work (montado de ./acbr-source). Roda DENTRO de
# um container com svn+git (o host não tem svn). Resumível: o SVN do SourceForge
# derruba conexões em checkouts grandes; o `svn update` retoma de onde parou.
#
# Uso (via Makefile): make acbr-fonte   (pin em ACBR_REV)
set -eu

: "${ACBR_REV:?defina ACBR_REV}"
ACBR_SVN="${ACBR_SVN:-https://svn.code.sf.net/p/acbr/code/trunk2}"
FRCE_REPO="${FRCE_REPO:-https://github.com/fortesinformatica/fortesreport-ce.git}"
# Sem ref explícito NÃO cai em master: build irreprodutível é pior que build que
# não roda, porque só aparece meses depois, num binário diferente do que foi
# testado. O Makefile passa ACBR_FRCE_REF.
: "${FRCE_REF:?defina FRCE_REF (o Makefile passa ACBR_FRCE_REF)}"
SVNFLAGS="--non-interactive --trust-server-cert"

cd /work

# Não enviar metadados de VCS para o contexto/imagem do build da base.
cat > .dockerignore <<'IGN'
**/.svn
**/.git
IGN

# --- ACBr (SVN, pinado, checkout esparso só do necessário) ---
if [ ! -d acbr/.svn ]; then
	svn checkout $SVNFLAGS --depth empty -r "$ACBR_REV" "$ACBR_SVN" acbr
fi
cd acbr
svn cleanup 2>/dev/null || true

# Fontes/Pacotes/Projetos: necessários para compilar as libs (NFSe/CTe/MDFe).
for dir in Fontes Pacotes Projetos; do
	n=0
	until svn update $SVNFLAGS -r "$ACBR_REV" --set-depth infinity "$dir"; do
		n=$((n + 1))
		[ "$n" -ge 12 ] && { echo "falha persistente em $dir"; exit 1; }
		echo "retry $n em $dir (reset de conexão)..."; svn cleanup; sleep 5
	done
done

# Schemas XSD por serviço (validação): NFSe (multi-provedor), CT-e e MDF-e.
svn update $SVNFLAGS -r "$ACBR_REV" --set-depth empty Exemplos Exemplos/ACBrDFe Exemplos/ACBrDFe/Schemas
for svc in NFSe CTe MDFe; do
	n=0
	until svn update $SVNFLAGS -r "$ACBR_REV" --set-depth infinity "Exemplos/ACBrDFe/Schemas/$svc"; do
		n=$((n + 1))
		[ "$n" -ge 12 ] && { echo "falha persistente em schemas $svc"; exit 1; }
		echo "retry $n em schemas $svc..."; svn cleanup; sleep 5
	done
done
echo "ACBr: $(svn info | grep -i '^Revision' || true)"
cd /work

# --- FortesReport CE (git, pinado) ---
if [ ! -d frce/.git ]; then
	git clone "$FRCE_REPO" frce
fi
cd frce
git fetch --all --quiet || true
git checkout --detach "$FRCE_REF" --quiet
echo "FRCE: $(git rev-parse --short HEAD) (pinado em $FRCE_REF)"

echo "OK, cache em ./acbr-source pronto (ACBR_REV=$ACBR_REV)"
