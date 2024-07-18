// Command blog is the CLI entry point for the blogging engine. Subcommands:
//
//	blog serve   [flags]   run the HTTP server (default)
//	blog export  [flags]   render a static snapshot of the public blog
//	blog migrate-json-to-sqlite [flags]   copy a JSON store into SQLite
//	blog hash-password [password]         print a bcrypt hash for ADMIN_PASSWORD_HASH
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"blog/internal/blog"
	"blog/internal/render"
	"blog/internal/store"
	"blog/internal/web"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(runServe(os.Args[1:]))
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "export":
		os.Exit(runExport(os.Args[2:]))
	case "migrate-json-to-sqlite":
		os.Exit(runMigrate(os.Args[2:]))
	case "hash-password":
		os.Exit(runHashPassword(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		// "blog -addr :9000" means "blog serve -addr :9000".
		if strings.HasPrefix(os.Args[1], "-") {
			os.Exit(runServe(os.Args[1:]))
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `blog - a tiny self-hosted blogging engine

Usage:
  blog serve   [--addr :8080] [--store json|sqlite] [--data ./data] [--db blog.db] [--dev]
  blog export  [--out ./public] [other serve flags]
  blog migrate-json-to-sqlite [--data ./data] [--db blog.db]
  blog hash-password [password]

Environment:
  ADMIN_USER            admin username for the editor
  ADMIN_PASSWORD_HASH   bcrypt hash of the admin password (see hash-password)
`)
}

type storeFlags struct {
	kind string
	data string
	db   string
}

func addStoreFlags(fs *flag.FlagSet) *storeFlags {
	sf := &storeFlags{}
	fs.StringVar(&sf.kind, "store", "sqlite", "storage backend: json|sqlite")
	fs.StringVar(&sf.data, "data", "./data", "data directory for the json store")
	fs.StringVar(&sf.db, "db", "./blog.db", "database file for the sqlite store")
	return sf
}

func openStore(sf *storeFlags, r *render.Renderer) (store.Store, error) {
	switch sf.kind {
	case "json":
		return store.OpenJSON(sf.data, r.Render)
	case "sqlite":
		return store.OpenSQLite(sf.db, r.Render)
	default:
		return nil, fmt.Errorf("unknown store %q (want json|sqlite)", sf.kind)
	}
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	sf := addStoreFlags(fs)
	addr := fs.String("addr", ":8080", "listen address")
	dev := fs.Bool("dev", false, "re-parse templates per request")
	title := fs.String("title", "My Blog", "site title")
	desc := fs.String("description", "A blog built with Go.", "site description")
	baseURL := fs.String("base-url", "http://localhost:8080", "public base URL for the feed")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	r := render.New()
	st, err := openStore(sf, r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	defer st.Close()

	cfg := web.Config{
		Site:              web.Site{Title: *title, Description: *desc},
		AdminUser:         os.Getenv("ADMIN_USER"),
		AdminPasswordHash: os.Getenv("ADMIN_PASSWORD_HASH"),
		BaseURL:           strings.TrimRight(*baseURL, "/"),
		Dev:               *dev,
		Now:               blog.SystemClock,
	}
	if cfg.AdminUser == "" || cfg.AdminPasswordHash == "" {
		fmt.Fprintln(os.Stderr, "warning: ADMIN_USER/ADMIN_PASSWORD_HASH not set; editor is disabled")
	}

	srv, err := web.New(cfg, st, r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("blog listening on %s (store=%s)\n", *addr, sf.kind)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\nshutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		return 0
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "server error:", err)
			return 1
		}
		return 0
	}
}

func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	sf := addStoreFlags(fs)
	out := fs.String("out", "./public", "output directory")
	title := fs.String("title", "My Blog", "site title")
	desc := fs.String("description", "A blog built with Go.", "site description")
	baseURL := fs.String("base-url", "http://localhost:8080", "public base URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	r := render.New()
	st, err := openStore(sf, r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	defer st.Close()

	cfg := web.Config{
		Site:    web.Site{Title: *title, Description: *desc},
		BaseURL: strings.TrimRight(*baseURL, "/"),
		Now:     blog.SystemClock,
	}
	srv, err := web.New(cfg, st, r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if err := os.MkdirAll(filepath.Join(*out, "posts"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// Render each page by sending requests straight to the handler.
	capture := func(path string) ([]byte, int) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Body.Bytes(), rec.Code
	}

	body, _ := capture("/")
	if err := os.WriteFile(filepath.Join(*out, "index.html"), body, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	body, _ = capture("/feed.xml")
	if err := os.WriteFile(filepath.Join(*out, "feed.xml"), body, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	now := blog.SystemClock()
	posts, err := st.List(context.Background(), store.ListOpts{
		Status: blog.Published, PublishedBefore: &now, Order: store.NewestFirst,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	for _, p := range posts {
		body, code := capture(fmt.Sprintf("/posts/%d", p.ID))
		if code != http.StatusOK {
			continue
		}
		dst := filepath.Join(*out, "posts", fmt.Sprintf("%d.html", p.ID))
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}
	fmt.Printf("exported %d posts to %s\n", len(posts), *out)
	return 0
}

func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate-json-to-sqlite", flag.ContinueOnError)
	data := fs.String("data", "./data", "json data directory (source)")
	db := fs.String("db", "./blog.db", "sqlite database file (destination)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	r := render.New()
	src, err := store.OpenJSON(*data, r.Render)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening json store:", err)
		return 1
	}
	defer src.Close()

	dst, err := store.OpenSQLite(*db, r.Render)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening sqlite store:", err)
		return 1
	}
	defer dst.Close()

	posts, comments, err := store.Migrate(context.Background(), dst, src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration error:", err)
		return 1
	}
	fmt.Printf("migrated %d posts and %d comments to %s\n", posts, comments, *db)
	return 0
}

func runHashPassword(args []string) int {
	var password string
	if len(args) > 0 {
		password = args[0]
	} else {
		fmt.Fprint(os.Stderr, "Password: ")
		sc := bufio.NewScanner(os.Stdin)
		if sc.Scan() {
			password = strings.TrimRight(sc.Text(), "\r\n")
		}
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "error: empty password")
		return 2
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Println(string(hash))
	return 0
}
