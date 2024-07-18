// Package web serves the public blog and the password-protected editor.
package web

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"blog/internal/blog"
	"blog/internal/render"
	"blog/internal/store"
)

// Site is shown on every page and in the feed.
type Site struct {
	Title       string
	Description string
}

// Config configures a Server.
type Config struct {
	Site Site
	// AdminPasswordHash is a bcrypt hash. If either is empty the editor is
	// disabled.
	AdminUser         string
	AdminPasswordHash string
	// BaseURL is used for links in the RSS feed. No trailing slash.
	BaseURL string
	// Dev reloads templates on every request.
	Dev bool
	// Now defaults to the system clock.
	Now blog.Clock
	// FeedLimit is the number of feed items; 0 means 20.
	FeedLimit int
	// SessionTTL is how long a login lasts; 0 means 24h.
	SessionTTL time.Duration
}

type session struct {
	user      string
	csrf      string
	authed    bool
	expiresAt time.Time
}

// Server is the blog's HTTP handler.
type Server struct {
	cfg      Config
	store    store.Store
	renderer *render.Renderer
	now      blog.Clock
	mux      *http.ServeMux

	tmplMu    sync.RWMutex
	templates map[string]*template.Template

	sessMu   sync.Mutex
	sessions map[string]*session

	rlMu    sync.Mutex
	rlHits  map[string][]time.Time // per-IP comment timestamps
	rlLimit int
	rlWin   time.Duration
}

// pages are each parsed together with the base layout.
var pages = []string{"home", "post", "login", "editor_list", "editor_form", "error"}

// New parses the templates up front, unless in Dev mode.
func New(cfg Config, st store.Store, r *render.Renderer) (*Server, error) {
	if cfg.Now == nil {
		cfg.Now = blog.SystemClock
	}
	if cfg.FeedLimit == 0 {
		cfg.FeedLimit = 20
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	if cfg.Site.Title == "" {
		cfg.Site.Title = "My Blog"
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		renderer: r,
		now:      cfg.Now,
		sessions: make(map[string]*session),
		rlHits:   make(map[string][]time.Time),
		rlLimit:  10,
		rlWin:    time.Minute,
	}
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) funcMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(h string) template.HTML { return template.HTML(h) },
		"date":     formatDate,
		"rfc3339":  formatRFC3339,
	}
}

func (s *Server) parseTemplates() error {
	set := make(map[string]*template.Template)
	for _, name := range pages {
		t, err := template.New("base.tmpl").Funcs(s.funcMap()).ParseFS(
			templatesFS, "templates/base.tmpl", "templates/"+name+".tmpl")
		if err != nil {
			return err
		}
		set[name] = t
	}
	s.tmplMu.Lock()
	s.templates = set
	s.tmplMu.Unlock()
	return nil
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	if s.cfg.Dev {
		if err := s.parseTemplates(); err != nil {
			log.Printf("template parse error: %v", err)
		}
	}
	s.tmplMu.RLock()
	t := s.templates[name]
	s.tmplMu.RUnlock()
	if t == nil {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template execute error (%s): %v", name, err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, code int, msg string) {
	data := struct {
		Site    Site
		Code    int
		Message string
	}{s.cfg.Site, code, msg}
	s.render(w, code, "error", data)
}

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

func (s *Server) editorEnabled() bool {
	return s.cfg.AdminUser != "" && s.cfg.AdminPasswordHash != ""
}

func formatDate(t any) string {
	tm := asTime(t)
	if tm == nil {
		return "Draft"
	}
	return tm.Format("Jan 2, 2006")
}

func formatRFC3339(t any) string {
	tm := asTime(t)
	if tm == nil {
		return ""
	}
	return tm.UTC().Format(time.RFC3339)
}

func asTime(t any) *time.Time {
	switch v := t.(type) {
	case time.Time:
		return &v
	case *time.Time:
		return v
	default:
		return nil
	}
}
