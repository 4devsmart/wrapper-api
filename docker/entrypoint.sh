#!/bin/sh
# Entrypoint do runtime: escolhe o processo pelo primeiro argumento.
#
#   api     (default) servidor HTTP. NÃO linka a lib fiscal nativa: delega ao
#           worker por RPC (ACBR_WORKERS), então um crash da lib não o derruba.
#   worker  processo que carrega a lib nativa.
#
# A lib é compilada com widgetset GTK2 (o FortesReport rasteriza o DANFSE/DACTE)
# e chama gtk_init já no carregamento: precisa de um $DISPLAY válido mesmo para
# operações que não imprimem. Por isso o Xvfb sobe SÓ no worker: é o único
# processo que abre o .so.
set -eu

subir_xvfb() {
	export DISPLAY="${DISPLAY:-:99}"
	LOCK="/tmp/.X${DISPLAY#:}-lock"
	rm -f "$LOCK" 2>/dev/null || true

	Xvfb "$DISPLAY" -screen 0 1024x768x24 -nolisten tcp >/tmp/xvfb.log 2>&1 &

	# Aguarda o socket do X aparecer (até ~5s).
	SOCK="/tmp/.X11-unix/X${DISPLAY#:}"
	i=0
	while [ "$i" -lt 50 ]; do
		[ -S "$SOCK" ] && break
		i=$((i + 1))
		sleep 0.1
	done

	# Falha cedo e claro se o Xvfb não subiu: a lib GTK2 chama gtk_init no load e
	# daria SIGSEGV (TBitmap nulo) sem display: erro confuso. Melhor abortar aqui.
	if [ ! -S "$SOCK" ]; then
		echo "entrypoint: Xvfb não inicializou (sem $SOCK). Log:" >&2
		cat /tmp/xvfb.log >&2 2>/dev/null || true
		exit 1
	fi
}

case "${1:-api}" in
api)
	exec /usr/local/bin/api
	;;
worker)
	subir_xvfb
	exec /usr/local/bin/fiscal-worker
	;;
*)
	# Escape hatch: shell de depuração, `docker compose run api sh`, etc.
	exec "$@"
	;;
esac
