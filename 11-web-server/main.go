// Command web-server is a small static HTTP/1.1 server built on raw TCP
// sockets. It hand-parses requests and serves files from a configurable
// document root.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"webserver/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "host:port to listen on")
	dir := flag.String("dir", "./www", "document root to serve files from")
	flag.Parse()

	srv, err := server.New(*addr, *dir)
	if err != nil {
		log.Fatalf("web-server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("web-server: %v", err)
	}
}
