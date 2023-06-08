// Command server runs the URL shortener HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/example/urlshortener/internal/shortener"
	"github.com/example/urlshortener/web"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	baseURL := flag.String("base-url", "http://localhost:8080", "external base URL used to build short links")
	storePath := flag.String("store", "", "path to JSON file for persistent storage (empty = in-memory only)")
	flag.Parse()

	store, err := newStore(*storePath)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	h := shortener.NewHandler(store, *baseURL)
	h.SetGUI(guiHandler())

	srv := &http.Server{
		Addr:         *addr,
		Handler:      h.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (base-url=%s, store=%s)", *addr, *baseURL, storeDesc(*storePath))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func newStore(path string) (shortener.Store, error) {
	if path == "" {
		return shortener.NewMemStore(), nil
	}
	return shortener.NewFileStore(path)
}

func storeDesc(path string) string {
	if path == "" {
		return "in-memory"
	}
	return path
}

func guiHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(web.IndexHTML)
	})
}
