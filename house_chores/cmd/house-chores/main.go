// Obowiązki (Chores) is a Home Assistant app for household chores.
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
	_ "time/tzdata" // the alpine image has no time zone data

	"github.com/Fryk/house-chores-ha/house_chores/internal/config"
	"github.com/Fryk/house-chores-ha/house_chores/internal/ha"
	"github.com/Fryk/house-chores-ha/house_chores/internal/server"
	"github.com/Fryk/house-chores-ha/house_chores/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("stopped with error", "err", err)
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
		log.Info("fake mode: notifications go to the log and to the /dev page")
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
	log.Info("listening", "addr", httpSrv.Addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
