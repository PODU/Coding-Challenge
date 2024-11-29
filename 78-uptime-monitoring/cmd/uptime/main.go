// Command uptime runs the monitoring service: it probes the monitors from a
// YAML config, stores results, sends alerts and serves an API and dashboard.
// On SIGINT/SIGTERM it waits for running checks before closing the store.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/uptime/internal/alert"
	"github.com/example/uptime/internal/checker"
	"github.com/example/uptime/internal/clock"
	"github.com/example/uptime/internal/config"
	"github.com/example/uptime/internal/scheduler"
	"github.com/example/uptime/internal/store"

	"github.com/example/uptime/internal/api"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(*cfgPath, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string, log *slog.Logger) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	monitors, err := cfg.BuildMonitors()
	if err != nil {
		return err
	}

	st, err := openStore(cfg.Storage)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Persist config & seed the state tracker from the last known status so a
	// restart doesn't false-alert on the first probe.
	tracker := alert.NewStateTracker(cfg.Defaults.AlertThreshold)
	for _, m := range monitors {
		if err := st.UpsertMonitor(ctx, m); err != nil {
			return err
		}
		tracker.SetThreshold(m.ID, m.AlertThreshold)
		if last, err := st.LatestStatus(ctx, m.ID); err == nil {
			tracker.Seed(m.ID, last)
		}
	}

	alerter := buildAlerter(cfg.Alerters, log)
	clk := clock.New()
	metrics := &scheduler.Metrics{}

	sched := scheduler.New(scheduler.Options{
		Clock:   clk,
		Checker: checker.NewMulti(clk),
		Store:   st,
		Tracker: tracker,
		Alerter: alerter,
		Logger:  log,
		Metrics: metrics,
	})

	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           api.New(st, tracker, metrics, log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("http server listening", "addr", cfg.Server.Listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			stop()
		}
	}()

	log.Info("scheduler starting", "monitors", len(monitors))
	done := make(chan struct{})
	go func() {
		sched.Run(ctx, monitors)
		close(done)
	}()

	<-ctx.Done()
	log.Info("shutdown signal received; draining")

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	<-done // wait for scheduler loops + in-flight checks to drain
	log.Info("shutdown complete")
	return nil
}

func openStore(sc config.StorageConfig) (store.Store, error) {
	switch sc.Driver {
	case "memory":
		return store.NewMemory(), nil
	default: // sqlite
		return store.OpenSQLite(sc.DSN)
	}
}

func buildAlerter(cfgs []config.AlerterConfig, log *slog.Logger) alert.Alerter {
	var alerters []alert.Alerter
	for _, a := range cfgs {
		switch a.Type {
		case "log":
			alerters = append(alerters, alert.NewLogAlerter(log))
		case "webhook":
			alerters = append(alerters, alert.NewWebhookAlerter(a.URL, nil))
		case "email":
			var auth smtp.Auth // unauthenticated by default; configure as needed
			alerters = append(alerters, alert.NewEmailAlerter(a.SMTP, a.From, a.To, auth))
		default:
			log.Warn("unknown alerter type; ignoring", "type", a.Type)
		}
	}
	if len(alerters) == 0 {
		// With nothing configured, at least log transitions.
		alerters = append(alerters, alert.NewLogAlerter(log))
	}
	return alert.NewMultiAlerter(alerters...)
}
