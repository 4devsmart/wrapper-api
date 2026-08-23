// Command api é o servidor HTTP do wrapper fiscal.
//
// Compila SEM cgo e NÃO abre a lib nativa: fala RPC com o fiscal-worker sobre
// socket unix (ver internal/acbr/rpc.go). Um SIGSEGV dentro da lib mata o
// worker e a requisição em curso — este processo continua de pé.
//
// Não persiste nada. O certificado A1 chega no corpo da requisição que
// transmite, é usado em memória e vai embora com ela.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/boleto"
	"github.com/4devsmart/wrapper-api/internal/cte"
	"github.com/4devsmart/wrapper-api/internal/mdfe"
	"github.com/4devsmart/wrapper-api/internal/modulo"
	"github.com/4devsmart/wrapper-api/internal/nfse"
	"github.com/4devsmart/wrapper-api/internal/platform/config"
	"github.com/4devsmart/wrapper-api/internal/platform/versao"
	"github.com/4devsmart/wrapper-api/internal/servidor"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("falha ao carregar configuração", "err", err)
		os.Exit(1)
	}
	if len(cfg.ACBr.Workers) == 0 {
		// Sem worker a API sobe e responde 503 no /readyz — deliberado: é melhor
		// subir dizendo o que falta do que recusar o boot e não explicar nada.
		slog.Warn("nenhum fiscal-worker configurado; nada será transmitido", "dica", "defina ACBR_WORKERS")
	}

	svc := acbr.New(cfg.ACBr)
	defer func() { _ = svc.Close() }()

	// Módulos por documento. MDF-e, NFS-e e boletos entram nas etapas 4 a 6;
	// a lista vazia continua sendo estado válido.
	modulos := []modulo.Modulo{
		cte.NovoModulo(svc.CTe),
		mdfe.NovoModulo(svc.MDFe),
		nfse.NovoModulo(svc.NFSe),
		boleto.NovoModulo(svc.Boleto),
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           servidor.Novo(cfg, svc, modulos...).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Sem WriteTimeout: uma transmissão espera a SEFAZ, e cortar a resposta
		// no meio deixaria o cliente sem o protocolo de um documento que PODE ter
		// sido autorizado. Quem limita a duração é o timeout do worker.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("api escutando",
			"addr", cfg.HTTPAddr, "modo", cfg.Modo,
			"workers", len(cfg.ACBr.Workers), "modulos", len(modulos),
			"commit", versao.Curto())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor encerrou com erro", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("sinal recebido, encerrando a api…")

	// Prazo generoso pelo mesmo motivo do WriteTimeout ausente: pode haver uma
	// transmissão em curso falando com a SEFAZ.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("erro no shutdown", "err", err)
	}
	slog.Info("api encerrada")
}
