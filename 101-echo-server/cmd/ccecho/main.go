// Command ccecho is an RFC 862 echo server over TCP or UDP.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	echo "ccecho"
)

func main() {
	var (
		port  = flag.Int("port", echo.DefaultPort, "Port to listen on (default 7, the RFC 862 echo port)")
		host  = flag.String("host", "", `Host/interface to bind (default "" = all interfaces)`)
		udp   = flag.Bool("udp", false, "Run in UDP mode instead of the default TCP mode")
		idle  = flag.Duration("idle", 0, "Per-connection idle/read timeout, 0 = none")
		drain = flag.Duration("drain", echo.DefaultDrainTimeout, "Max time to wait for active conns on shutdown")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	protocol := "tcp"
	if *udp {
		protocol = "udp"
	}

	cfg := echo.Config{
		Host:         *host,
		Port:         *port,
		Protocol:     protocol,
		IdleTimeout:  *idle,
		DrainTimeout: *drain,
		Logger:       logger,
	}

	srv, err := echo.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccecho: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ccecho: %v\n", err)
		os.Exit(1)
	}

	logger.Info("ccecho shut down cleanly")
}
