// Package httpapi has the JSON API, the server-rendered web pages and the
// middleware.
package httpapi

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"pastebin/internal/highlight"
	"pastebin/internal/paste"
	"pastebin/internal/store"
	"pastebin/web"
)

// Server serves the API and web UI.
type Server struct {
	svc       *paste.Service
	store     store.Store
	baseURL   string
	maxBytes  int64
	log       *slog.Logger
	templates map[string]*template.Template
}

// Options configures a Server.
type Options struct {
	Service  *paste.Service
	Store    store.Store
	BaseURL  string
	MaxBytes int64
	Logger   *slog.Logger
}

// NewServer parses the embedded templates and returns a Server.
func NewServer(opts Options) (*Server, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	tmpls, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	return &Server{
		svc:       opts.Service,
		store:     opts.Store,
		baseURL:   opts.BaseURL,
		maxBytes:  opts.MaxBytes,
		log:       opts.Logger,
		templates: tmpls,
	}, nil
}

// parseTemplates parses each page together with the layout, separately, so
// that the pages' "title" and "content" blocks don't overwrite each other.
func parseTemplates() (map[string]*template.Template, error) {
	pages := []string{"index", "view", "deleted", "error"}
	out := make(map[string]*template.Template, len(pages))
	for _, name := range pages {
		t, err := template.New("layout").ParseFS(web.Templates,
			"templates/layout.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", name, err)
		}
		out[name] = t
	}
	return out, nil
}

// Handler returns the routes wrapped in middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/pastes", s.apiCreate)
	mux.HandleFunc("GET /api/pastes/{id}", s.apiGet)
	mux.HandleFunc("GET /api/pastes/{id}/raw", s.handleRaw)
	mux.HandleFunc("DELETE /api/pastes/{id}", s.apiDelete)

	mux.HandleFunc("GET /healthz", s.health)

	// ServeMux rejects "GET /{id}/raw" alongside a "/static/" subtree as
	// ambiguous, so static files are routed before the mux instead (below).
	mux.HandleFunc("GET /{$}", s.webIndex)
	mux.HandleFunc("POST /{$}", s.webCreate)
	mux.HandleFunc("GET /{id}/raw", s.handleRaw)
	mux.HandleFunc("POST /{id}/delete", s.webDelete)
	mux.HandleFunc("GET /{id}", s.webView)

	staticFS, _ := fs.Sub(web.Static, "static")
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/static/") {
			fileServer.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})

	// Outermost first: recovery, logging, body limit.
	var h http.Handler = root
	h = withMaxBytes(s.maxBytes, h)
	h = withLogging(s.log, h)
	h = withRecovery(s.log, h)
	return h
}

func languageList() []highlight.Language { return highlight.Languages() }
