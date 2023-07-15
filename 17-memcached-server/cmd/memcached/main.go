// Command memcached is a memcached-compatible cache server speaking the
// classic text protocol over TCP.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"memcached/internal/server"
	"memcached/internal/store"
)

func main() {
	var (
		port    = flag.Int("p", 11211, "TCP port to listen on")
		host    = flag.String("l", "127.0.0.1", "interface/host to bind")
		memMB   = flag.Int64("m", 64, "max memory for items in MiB (0 = unlimited); triggers LRU eviction")
		maxConn = flag.Int("c", 1024, "soft max concurrent connections")
		verbose = flag.Bool("v", false, "verbose per-command logging")
	)
	flag.Parse()

	st := store.New(*memMB * 1024 * 1024)
	srv := server.New(server.Config{
		Addr:     net.JoinHostPort(*host, strconv.Itoa(*port)),
		MaxConns: *maxConn,
		Verbose:  *verbose,
	}, st)

	// Expiry is checked lazily on access; this sweep just frees memory held by
	// expired keys nobody asks for.
	stopJanitor := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopJanitor:
				return
			case <-ticker.C:
				st.ReapExpired()
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "shutting down...")
		close(stopJanitor)
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
