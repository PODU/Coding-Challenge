// Command dnsfwd is a TTL-caching DNS forwarder. It listens for UDP DNS queries
// and answers them from a local cache or by forwarding to a configured upstream
// recursive resolver, with an optional blocklist for sinkholing names.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dns-forwarder/internal/blocklist"
	"dns-forwarder/internal/cache"
	"dns-forwarder/internal/server"
	"dns-forwarder/internal/upstream"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dnsfwd:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		listen      = flag.String("listen", ":1053", "UDP listen address host:port (use :53 in prod; needs privileges)")
		upstreamArg = flag.String("upstream", "8.8.8.8:53", "upstream resolver host:port")
		timeout     = flag.Duration("timeout", 5*time.Second, "per-query upstream timeout")
		cacheOn     = flag.Bool("cache", true, "enable the TTL cache")
		maxTTL      = flag.Uint("max-ttl", 0, "cap cached TTL seconds (0 = off)")
		minTTL      = flag.Uint("min-ttl", 0, "floor cached TTL seconds (0 = off)")
		blockPath   = flag.String("blocklist", "", "path to newline-delimited blocklist file")
		sinkhole    = flag.String("sinkhole", "nxdomain", "block mode: nxdomain or zero")
		tcpFallback = flag.Bool("tcp-fallback", false, "retry truncated (TC=1) responses over TCP")
		logLevel    = flag.String("log-level", "info", "log level: debug, info, warn, error")
	)
	flag.Parse()

	logger := newLogger(*logLevel)

	if *sinkhole != "nxdomain" && *sinkhole != "zero" {
		return fmt.Errorf("invalid -sinkhole %q (want nxdomain or zero)", *sinkhole)
	}

	var bl *blocklist.List
	if *blockPath != "" {
		l, err := blocklist.Load(*blockPath)
		if err != nil {
			return fmt.Errorf("loading blocklist: %w", err)
		}
		bl = l
		logger.Info("blocklist loaded", "path", *blockPath, "entries", bl.Len())
	}

	var c *cache.Cache
	if *cacheOn {
		c = cache.NewWithOptions(cache.Options{
			MaxTTL: uint32(*maxTTL),
			MinTTL: uint32(*minTTL),
		})
	}

	up := upstream.New(*upstreamArg, *timeout)
	up.TCPFallback = *tcpFallback

	srv := server.New(server.Config{
		Listen:   *listen,
		Timeout:  *timeout,
		Sinkhole: *sinkhole,
		Logger:   logger,
	}, up, c, bl)

	if err := srv.Start(); err != nil {
		return fmt.Errorf("starting server on %s: %w (port 53 needs elevated privileges; try -listen :1053)", *listen, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info("dns-forwarder ready", "listen", srv.LocalAddr().String(), "upstream", *upstreamArg)

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped",
		"queries", srv.Counters.Queries.Load(),
		"hits", srv.Counters.Hits.Load(),
		"forwards", srv.Counters.Forwards.Load(),
		"blocked", srv.Counters.Blocked.Load(),
		"errors", srv.Counters.Errors.Load())
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
