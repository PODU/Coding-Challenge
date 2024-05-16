// Command dropd is the sync server: a per-user file store behind a small REST
// API. Clients treat its copy as authoritative.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"dropbox/internal/atomic"
	"dropbox/internal/server"
)

func main() {
	log.SetFlags(0)
	fs := flag.NewFlagSet("dropd", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "listen address")
	data := fs.String("data", "./data", "storage root directory")
	cfgPath := fs.String("config", "./dropd.json", "users/tokens config file")
	tlsCert := fs.String("tls-cert", "", "optional TLS certificate file")
	tlsKey := fs.String("tls-key", "", "optional TLS key file")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: dropd [flags] [serve|useradd <user>]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])

	args := fs.Args()
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "serve":
		runServe(*addr, *data, *cfgPath, *tlsCert, *tlsKey)
	case "useradd":
		if len(args) < 2 {
			log.Fatal("usage: dropd useradd <user> [--password-stdin]")
		}
		runUserAdd(*cfgPath, args[1])
	default:
		fs.Usage()
		os.Exit(2)
	}
}

func loadConfig(path string) (server.Config, error) {
	var cfg server.Config
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(b) == 0 {
		return cfg, nil
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

func runServe(addr, data, cfgPath, cert, key string) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if len(cfg.Users) == 0 {
		log.Printf("warning: no users configured in %s (create one with: dropd useradd <user>)", cfgPath)
	}
	srv := server.New(data, cfg)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
	log.Printf("dropd listening on %s, data=%s, users=%d", addr, data, len(cfg.Users))
	if cert != "" && key != "" {
		log.Fatal(httpSrv.ListenAndServeTLS(cert, key))
	}
	log.Fatal(httpSrv.ListenAndServe())
}

func runUserAdd(cfgPath, name string) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	pw, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("read password: %v", err)
	}
	password := strings.TrimRight(string(pw), "\r\n")
	if password == "" {
		log.Fatal("empty password (pipe one via --password-stdin)")
	}
	u := server.User{Name: name, PasswordHash: server.HashPassword(password), Token: newToken()}
	replaced := false
	for i := range cfg.Users {
		if cfg.Users[i].Name == name {
			cfg.Users[i] = u
			replaced = true
		}
	}
	if !replaced {
		cfg.Users = append(cfg.Users, u)
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := atomic.WriteBytes(cfgPath, 0o600, b); err != nil {
		log.Fatalf("write config: %v", err)
	}
	fmt.Printf("user %q ready. token: %s\n", name, u.Token)
}

func newToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
