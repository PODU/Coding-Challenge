// Command redis-server is a small Redis clone: RESP2 over TCP with an in-memory
// store that supports key expiry.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rediserver/internal/server"
	"rediserver/internal/store"
)

func main() {
	port := flag.Int("port", 6379, "Port to listen on")
	host := flag.String("host", "127.0.0.1", "Bind address (use 0.0.0.0 for all interfaces)")
	dbfile := flag.String("dbfile", "dump.rdb", "Snapshot file path for SAVE/load")
	noActiveExpiry := flag.Bool("no-active-expiry", false, "Disable the background expiry sweeper (lazy only)")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	st := store.New()
	if err := st.Load(*dbfile); err != nil {
		logger.Printf("warning: could not load snapshot %q: %v (starting empty)", *dbfile, err)
	} else if st.Len() > 0 {
		logger.Printf("loaded %d keys from %s", st.Len(), *dbfile)
	}

	srv := server.New(st, *dbfile, logger)

	stopExpiry := make(chan struct{})
	if !*noActiveExpiry {
		go srv.RunActiveExpiry(100*time.Millisecond, stopExpiry)
	}

	addr := net.JoinHostPort(*host, fmt.Sprintf("%d", *port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("cannot bind %s: %v", addr, err)
	}
	logger.Printf("redis-server listening on %s", ln.Addr())

	// On SIGINT/SIGTERM save a snapshot, then close the listener so Serve returns.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Printf("shutting down: saving snapshot to %s", *dbfile)
		close(stopExpiry)
		if err := st.Save(*dbfile); err != nil {
			logger.Printf("snapshot save failed: %v", err)
		}
		ln.Close()
	}()

	if err := srv.Serve(ln); err != nil {
		// After a signal this is net.ErrClosed, which is expected.
		logger.Printf("server stopped: %v", err)
	}
}
