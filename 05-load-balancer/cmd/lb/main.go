// Command lb is a small HTTP load balancer. Requests go round-robin to the
// healthy backends; a background checker takes backends out of rotation when
// they fail and puts them back when they recover.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/lb/internal/backend"
	"github.com/example/lb/internal/config"
	"github.com/example/lb/internal/health"
	"github.com/example/lb/internal/pool"
	"github.com/example/lb/internal/proxy"
	"github.com/example/lb/internal/scheduler"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(args, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	urls, err := cfg.ParsedBackends()
	if err != nil {
		return err
	}

	// The error handler has to exist before the backends that use it.
	transport := proxy.NewTransport(cfg.DialTimeout, cfg.RespHdrTimeout)
	onError := proxy.NewErrorHandler(logger)

	backends := make([]*backend.Backend, 0, len(urls))
	for _, u := range urls {
		backends = append(backends, backend.New(u, transport, onError))
	}

	bp := pool.New(backends)
	sched := newScheduler(cfg.Strategy)
	handler := proxy.NewHandler(bp, sched, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	checker := health.New(bp, cfg.HealthPath, cfg.HealthInterval, cfg.HealthTimeout, logger)
	healthDone := make(chan struct{})
	go func() {
		defer close(healthDone)
		checker.Run(ctx)
	}()

	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: handler,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("load balancer started",
			"listen", cfg.Listen,
			"backends", cfg.Backends,
			"strategy", cfg.Strategy,
			"health_path", cfg.HealthPath,
			"health_interval", cfg.HealthInterval.String(),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		stop()
		<-healthDone
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	shutdownErr := srv.Shutdown(shutdownCtx)
	<-healthDone
	if shutdownErr != nil {
		return shutdownErr
	}
	if err := <-serverErr; err != nil {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

// newScheduler picks the scheduler for a strategy name. Validate only accepts
// round_robin today, so that is all this needs to handle.
func newScheduler(string) scheduler.Scheduler {
	return scheduler.NewRoundRobin()
}

func newLogger(cfg config.Config) *slog.Logger {
	lvl, _ := cfg.LogLevelValue()
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(lvl)}))
}
