// Command server runs the chat server: the browser client and WebSocket
// endpoint over HTTP, and optionally a plain TCP listener for telnet clients.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"realtimechat/internal/chat"
	"realtimechat/internal/transport"
	"realtimechat/web"
)

func main() {
	httpAddr := flag.String("http", ":8080", "HTTP/WebSocket listen address")
	tcpAddr := flag.String("tcp", "", "optional raw-TCP listen address (e.g. :7007); empty disables it")
	flag.Parse()

	hub := chat.NewHub()
	go hub.Run()

	srv := transport.NewServer(hub, web.Files)
	httpServer := &http.Server{Addr: *httpAddr, Handler: srv.Handler()}

	var tcpLn net.Listener
	if *tcpAddr != "" {
		ln, err := net.Listen("tcp", *tcpAddr)
		if err != nil {
			log.Fatalf("tcp listen %s: %v", *tcpAddr, err)
		}
		tcpLn = ln
		go transport.ServeTCP(tcpLn, hub)
		log.Printf("raw-TCP adapter listening on %s", *tcpAddr)
	}

	go func() {
		browse := *httpAddr
		if strings.HasPrefix(browse, ":") {
			browse = "localhost" + browse
		}
		log.Printf("chat server listening on %s (open http://%s/)", *httpAddr, browse)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	hub.Shutdown()
	if tcpLn != nil {
		_ = tcpLn.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	log.Println("bye")
}
