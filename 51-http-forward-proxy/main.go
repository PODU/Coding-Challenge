// Command http-forward-proxy is a forward proxy for HTTP and HTTPS (CONNECT).
//
// It parses absolute-form proxy requests, strips hop-by-hop headers, adds
// X-Forwarded-For, blocks forbidden hosts, filters response bodies for banned
// words, logs access to a file and stdout, and tunnels HTTPS via CONNECT.
//
// Usage:
//
//	go run . -addr :8989
//	curl --proxy "http://localhost:8989" "http://httpbin.org/ip"
//	curl --proxy "http://localhost:8989" "https://httpbin.org/ip"
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/podu2997/http-forward-proxy/internal/filter"
	"github.com/podu2997/http-forward-proxy/internal/logx"
	"github.com/podu2997/http-forward-proxy/internal/proxy"
)

func main() {
	addr := flag.String("addr", ":8989", "Listen address")
	forbiddenHosts := flag.String("forbidden-hosts", "forbidden-hosts.txt", "Path to forbidden hosts file")
	bannedWords := flag.String("banned-words", "banned-words.txt", "Path to banned words file")
	logFile := flag.String("log-file", "proxy.log", "Access log file path")
	dialTimeout := flag.Duration("dial-timeout", 10*time.Second, "Origin dial timeout")
	idleTimeout := flag.Duration("idle-timeout", 90*time.Second, "Keep-alive idle timeout")
	verbose := flag.Bool("verbose", false, "Verbose per-request debug logging")
	flag.Parse()

	if err := run(*addr, *forbiddenHosts, *bannedWords, *logFile, *dialTimeout, *idleTimeout, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(addr, forbiddenHosts, bannedWords, logFile string, dialTimeout, idleTimeout time.Duration, verbose bool) error {
	logger, err := logx.NewWithFile(logFile)
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", logFile, err)
	}
	defer logger.Close()

	f := filter.New()
	if existed, err := f.LoadHosts(forbiddenHosts); err != nil {
		return fmt.Errorf("loading forbidden hosts: %w", err)
	} else if !existed {
		logger.Infof("forbidden-hosts file %q not found; no hosts blocked", forbiddenHosts)
	}
	if existed, err := f.LoadWords(bannedWords); err != nil {
		return fmt.Errorf("loading banned words: %w", err)
	} else if !existed {
		logger.Infof("banned-words file %q not found; content filtering disabled", bannedWords)
	}

	handler := proxy.New(proxy.Config{
		Filter:      f,
		Logger:      logger,
		DialTimeout: dialTimeout,
		IdleTimeout: idleTimeout,
		Verbose:     verbose,
	})

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		// No ReadTimeout/WriteTimeout: CONNECT tunnels can stay open for a long
		// time. The header timeout still covers slow-loris clients.
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Infof("forward proxy listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Infof("shutdown signal received; draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
