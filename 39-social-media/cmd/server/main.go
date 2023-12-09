// Command server runs the HTTP API, the scheduler and the publishing workers
// in one process.
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

	"socialmedia/internal/api"
	"socialmedia/internal/clock"
	"socialmedia/internal/config"
	"socialmedia/internal/provider"
	"socialmedia/internal/publisher"
	"socialmedia/internal/queue"
	"socialmedia/internal/scheduler"
	"socialmedia/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer st.Close()

	reg := buildRegistry(cfg, log)
	log.Info("providers registered", "targets", reg.Names())

	q := queue.NewMemory(cfg.QueueCapacity)
	clk := clock.System{}

	pub := publisher.New(publisher.Config{
		Store:       st,
		Queue:       q,
		Registry:    reg,
		Clock:       clk,
		Workers:     cfg.Workers,
		MaxAttempts: cfg.MaxAttempts,
		Logger:      log,
	})
	pub.Start(ctx)

	sch := scheduler.New(scheduler.Config{
		Store:    st,
		Queue:    q,
		Clock:    clk,
		Interval: cfg.SchedulerTick,
		Logger:   log,
	})
	go func() {
		if err := sch.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("scheduler stopped", "err", err)
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewServer(api.Config{Store: st, Queue: q, Registry: reg, Clock: clk, Logger: log}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		return err
	}

	// Stop taking requests, then let the workers finish what's queued.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", "err", err)
	}
	q.Close()
	pub.Wait()
	log.Info("shutdown complete")
	return nil
}

// buildRegistry registers the providers named in SMT_TARGETS, falling back to
// the mock if none of them are usable.
func buildRegistry(cfg config.Config, log *slog.Logger) *provider.Registry {
	var providers []provider.Provider
	for _, target := range cfg.EnabledTargets {
		switch target {
		case "mock":
			providers = append(providers, provider.NewMock())
		case "linkedin":
			providers = append(providers, provider.NewLinkedIn(cfg.LinkedInToken, cfg.LinkedInAuthor))
		case "x":
			providers = append(providers, provider.NewX(cfg.XToken))
		default:
			log.Warn("ignoring unknown target", "target", target)
		}
	}
	if len(providers) == 0 {
		providers = append(providers, provider.NewMock())
	}
	return provider.NewRegistry(providers...)
}
