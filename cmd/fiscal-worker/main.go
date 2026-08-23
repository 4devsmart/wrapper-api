// Command fiscal-worker é o ÚNICO processo que carrega a lib fiscal nativa.
//
// A API (cmd/api) compila sem cgo e fala com ele por RPC sobre socket unix (ver
// internal/acbr/rpc.go). O motivo é confiabilidade: um SIGSEGV dentro da lib
// derruba o processo que a hospeda: aqui isso mata o worker e a requisição em
// curso, não o servidor HTTP inteiro. O supervisor (Docker) reinicia o worker.
//
// Compile com CGO_ENABLED=1 -tags acbrlib; sem a tag ele sobe com o stub e
// responde "lib indisponível" a tudo (útil em dev/CI).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("falha ao carregar configuração", "err", err)
		os.Exit(1)
	}

	// NewLocal (e não New): este processo É o dono da lib nativa.
	svc := acbr.NewLocal(cfg.ACBr)
	defer func() { _ = svc.Close() }()

	// Round-trip nativo por serviço (Inicializar → Versao → Finalizar): confirma
	// no boot que cada binding carrega, em vez de descobrir na primeira emissão.
	for _, b := range []struct {
		nome string
		sv   interface {
			Backend() string
			Version() (string, error)
		}
	}{{"NFSe", svc.NFSe}, {"CTe", svc.CTe}, {"MDFe", svc.MDFe}, {"NFe", svc.NFe}, {"Boleto", svc.Boleto}} {
		if v, err := b.sv.Version(); err != nil {
			slog.Warn("lib fiscal indisponível", "servico", b.nome, "backend", b.sv.Backend(), "err", err)
		} else {
			slog.Info("binding carregado", "servico", b.nome, "backend", b.sv.Backend(), "versao", v)
		}
	}

	lis, err := escutar(cfg.ACBr.WorkerListen)
	if err != nil {
		slog.Error("falha ao abrir o socket do worker", "socket", cfg.ACBr.WorkerListen, "err", err)
		os.Exit(1)
	}

	handler, reciclar := acbr.RPCHandlerReciclavel(svc, cfg.ACBr.WorkerSlots, cfg.ACBr.WorkerMaxCalls)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// Sem WriteTimeout: quem limita a duração de uma chamada é o cliente
		// (ACBR_WORKER_TIMEOUT_SECONDS). Cortar aqui deixaria a sessão nativa
		// órfã, ainda em execução, sem ninguém para ler o resultado.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Reciclagem: atingido o teto de chamadas, o worker sai de forma limpa e o
	// supervisor sobe um processo novo: memória corrompida por uma chamada não
	// sobrevive indefinidamente. Não é sobre crash (isso o isolamento cobre), é
	// sobre dado errado sem sintoma.
	go func() {
		select {
		case <-reciclar:
			slog.Info("teto de chamadas atingido; reciclando o worker", "max_calls", cfg.ACBr.WorkerMaxCalls)
			stop()
		case <-ctx.Done():
		}
	}()

	go func() {
		slog.Info("worker fiscal escutando", "socket", cfg.ACBr.WorkerListen, "slots", cfg.ACBr.WorkerSlots, "max_calls", cfg.ACBr.WorkerMaxCalls)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor do worker encerrou com erro", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("sinal recebido, encerrando o worker…")

	// Prazo generoso: uma transmissão em curso está falando com a SEFAZ e não
	// pode ser interrompida no meio: seria um documento em limbo.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("erro no shutdown do worker", "err", err)
	}
	slog.Info("worker encerrado")
}

// escutar abre o socket unix, removendo um arquivo órfão de um processo que
// morreu sem limpar (é o caso normal quando o worker crasha e é reiniciado).
func escutar(socket string) (net.Listener, error) {
	// O kernel limita sun_path a 108 bytes e devolve "bind: invalid argument"
	// quando estoura: mensagem que não diz nada sobre o tamanho do caminho.
	// Falhar aqui, nomeando o motivo, economiza a hora de depuração.
	const maxSunPath = 108
	if len(socket) >= maxSunPath {
		return nil, fmt.Errorf("caminho do socket tem %d bytes e o limite do kernel é %d: use um diretório mais curto em ACBR_WORKER_LISTEN (%s)",
			len(socket), maxSunPath-1, socket)
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		return nil, err
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	// A API roda com o mesmo usuário; 0660 mantém o socket fora do alcance de
	// outros usuários do container.
	if err := os.Chmod(socket, 0o660); err != nil {
		_ = lis.Close()
		return nil, err
	}
	return lis, nil
}
