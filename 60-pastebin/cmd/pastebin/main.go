// Command pastebin is a Pastebin-style web service with a JSON API and a web
// UI, stored in SQLite. It is configured through environment variables; see
// internal/config.
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

	"pastebin/internal/clock"
	"pastebin/internal/config"
	"pastebin/internal/httpapi"
	"pastebin/internal/paste"
	"pastebin/internal/store"
	"pastebin/internal/sweep"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	st, err := openStore(cfg, logger)
	if err != nil {
		return err
	}
	defer st.Close()

	clk := clock.Real{}
	svc := paste.NewService(st, clk, paste.Config{
		MaxBytes: cfg.MaxBytes,
		MaxTTL:   cfg.MaxTTL,
	})

	srv, err := httpapi.NewServer(httpapi.Options{
		Service:  svc,
		Store:    st,
		BaseURL:  cfg.BaseURL,
		MaxBytes: cfg.MaxBytes,
		Logger:   logger,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go sweep.New(st, clk, cfg.SweepInterval, logger).Run(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr, "base_url", cfg.BaseURL, "store", storeKind(cfg))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

func openStore(cfg config.Config, logger *slog.Logger) (store.Store, error) {
	if cfg.UseMemStore {
		logger.Info("using in-memory store")
		return store.NewMemStore(), nil
	}
	logger.Info("using sqlite store", "path", cfg.DBPath)
	return store.NewSQLiteStore(cfg.DBPath)
}

func storeKind(cfg config.Config) string {
	if cfg.UseMemStore {
		return "memory"
	}
	return "sqlite"
}
