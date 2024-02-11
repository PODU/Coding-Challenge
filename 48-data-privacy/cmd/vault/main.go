// Command vault runs the data privacy vault and its admin helpers.
//
// Subcommands:
//
//	vault serve                          start the HTTP(S) API service
//	vault genkey                         print a fresh base64 32-byte master key
//	vault apikey NAME --scopes a,b       mint an API key and print its JSON entry
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"datavault/internal/api"
	"datavault/internal/config"
	"datavault/internal/crypto"
	"datavault/internal/store"
	"datavault/internal/vault"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe()
	case "genkey":
		err = runGenKey()
	case "apikey":
		err = runAPIKey(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `vault — Data Privacy Vault

Usage:
  vault serve                          start the HTTP(S) API (reads env)
  vault genkey                         print a fresh base64 32-byte master key
  vault apikey NAME --scopes a,b       mint an API key, print its JSON entry

Environment (serve):
  ADDR              listen address (default :8443)
  STORE_BACKEND     memory | bolt | redis (default bolt)
  BOLT_PATH         bbolt file path (default vault.db)
  REDIS_ADDR        redis address (default localhost:6379)
  REDIS_PASSWORD    redis password (optional)
  REDIS_DB          redis db number (default 0)
  VAULT_MASTER_KEY  base64 of 32 bytes (REQUIRED)
  VAULT_DEDUP_KEY   base64 of 32 bytes (optional; derived from master if unset)
  API_KEYS          inline JSON array of key entries (preferred)
  API_KEYS_FILE     path to JSON array of key entries
  TLS_CERT/TLS_KEY  enable HTTPS when both are set
`)
}

func runGenKey() error {
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	fmt.Println(base64.StdEncoding.EncodeToString(key))
	return nil
}

func runAPIKey(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: vault apikey NAME --scopes tokenize,detokenize")
	}
	name := args[0]
	scopes := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "--scopes" && i+1 < len(args) {
			for _, s := range strings.Split(args[i+1], ",") {
				if s = strings.TrimSpace(s); s != "" {
					scopes = append(scopes, s)
				}
			}
			i++
		}
	}
	if len(scopes) == 0 {
		return errors.New("at least one --scopes value is required (tokenize and/or detokenize)")
	}

	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	key := "vk_" + base64.RawURLEncoding.EncodeToString(secret)
	sum := sha256.Sum256([]byte(key))

	entry := map[string]any{
		"name":       name,
		"key_sha256": hex.EncodeToString(sum[:]),
		"scopes":     scopes,
	}
	out, _ := json.MarshalIndent([]any{entry}, "", "  ")

	fmt.Println("API key (give this to the caller, store it nowhere else):")
	fmt.Println("  " + key)
	fmt.Println()
	fmt.Println("Add this entry to API_KEYS / API_KEYS_FILE (only the hash is stored):")
	fmt.Println(string(out))
	return nil
}

func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	st, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	kr, err := crypto.NewKeyringSingle(cfg.MasterKey)
	if err != nil {
		return err
	}
	v := vault.New(st, kr, cfg.DedupKey)

	ks, err := api.LoadKeyStore(cfg.APIKeysJSON, cfg.APIKeysFile)
	if err != nil {
		return err
	}

	srv := api.NewServer(v, ks, logger)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		tls := cfg.TLSCert != "" && cfg.TLSKey != ""
		logger.Info("starting server",
			"addr", cfg.Addr, "backend", cfg.StoreBackend, "tls", tls)
		if tls {
			errCh <- httpServer.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			logger.Warn("TLS disabled: set TLS_CERT and TLS_KEY for production, or terminate TLS at a reverse proxy")
			errCh <- httpServer.ListenAndServe()
		}
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

func openStore(cfg *config.Config) (store.Store, error) {
	switch cfg.StoreBackend {
	case "memory":
		return store.NewMemStore(), nil
	case "bolt", "":
		return store.NewBoltStore(cfg.BoltPath)
	case "redis":
		return store.NewRedisStore(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	default:
		return nil, fmt.Errorf("unknown STORE_BACKEND %q", cfg.StoreBackend)
	}
}
