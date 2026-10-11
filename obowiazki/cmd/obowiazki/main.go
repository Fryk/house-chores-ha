// Obowiązki — appka Home Assistant do domowych obowiązków.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // obraz alpine nie ma danych stref czasowych

	"github.com/Fryk/house-chores-ha/obowiazki/internal/config"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/ha"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/server"
	"github.com/Fryk/house-chores-ha/obowiazki/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("zatrzymano z błędem", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	log.Info("start", "mode", cfg.Mode, "port", cfg.Port, "data_dir", cfg.DataDir)

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	var client ha.Client
	if cfg.Mode == config.ModeSupervisor {
		client = ha.NewSupervisor(cfg.SupervisorURL, cfg.SupervisorToken, log)
	} else {
		client = ha.NewFake(log)
		log.Info("tryb fake: powiadomienia trafiają do logu i na stronę /dev")
	}

	srv, err := server.New(cfg, st, client, log)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go client.Run(ctx, srv.HandleAction)

	httpSrv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.ListenAndServe() }()
	log.Info("nasłuchuję", "addr", httpSrv.Addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("zatrzymuję")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
