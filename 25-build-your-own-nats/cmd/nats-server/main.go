// Command nats-server is a small NATS-compatible message broker. It supports
// the core pub/sub subset of the protocol.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codingchallenges/nats/internal/server"
)

func main() {
	addr := flag.String("addr", "0.0.0.0", "bind address")
	port := flag.Int("port", 4222, "TCP port to listen on")
	flag.Parse()

	logger := log.New(os.Stderr, "[nats-server] ", log.LstdFlags|log.Lmsgprefix)

	srv := server.New(server.Options{Addr: *addr, Port: *port, Logger: logger})
	if err := srv.Listen(); err != nil {
		logger.Fatalf("startup failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Printf("received %s, shutting down", sig)
		srv.Shutdown()
	}()

	if err := srv.Serve(); err != nil {
		logger.Fatalf("serve error: %v", err)
	}
}
