// Package httpapi exposes the REST surface over a store.Store.
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/ranking"
	"github.com/codingchallenges/topstories/internal/store"
)

// DefaultTopN is used by /stories/top when n is absent.
const DefaultTopN = 30

// Server wires the store and optional static web assets into an http.Handler.
type Server struct {
	store  store.Store
	static fs.FS // optional; nil disables the frontend routes
	mux    *http.ServeMux
}

// New builds a Server. staticFS may be nil to disable the web frontend.
func New(s store.Store, staticFS fs.FS) *Server {
	srv := &Server{store: s, static: staticFS, mux: http.NewServeMux()}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /stories", s.handleStories)
	s.mux.HandleFunc("GET /stories/top", s.handleTop)
	if s.static != nil {
		s.mux.Handle("GET /", http.FileServer(http.FS(s.static)))
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleStories serves GET /stories with filtering and pagination.
func (s *Server) handleStories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f, err := parseFilter(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stories := ranking.Apply(s.store.All(), f)

	limit, err := parseInt(q.Get("limit"), 50)
	if err != nil || limit < 0 {
		writeError(w, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := parseInt(q.Get("offset"), 0)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid offset")
		return
	}

	stories = paginate(stories, offset, limit)
	writeJSON(w, http.StatusOK, stories)
}

// handleTop serves GET /stories/top ranked by score.
func (s *Server) handleTop(w http.ResponseWriter, r *http.Request) {
	n, err := parseInt(r.URL.Query().Get("n"), DefaultTopN)
	if err != nil || n < 0 {
		writeError(w, http.StatusBadRequest, "invalid n")
		return
	}
	top := ranking.TopN(s.store.All(), n)
	writeJSON(w, http.StatusOK, top)
}

func parseFilter(q url.Values) (ranking.Filter, error) {
	var f ranking.Filter
	if v := q.Get("min_score"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, errInvalid("min_score")
		}
		f.MinScore = n
	}
	if v := q.Get("since"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, errInvalid("since")
		}
		f.Since = t
	}
	if v := q.Get("until"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, errInvalid("until")
		}
		f.Until = t
	}
	if v := q.Get("type"); v != "" {
		f.Types = splitCSV(v)
	}
	if v := q.Get("q"); v != "" {
		f.Keywords = splitCSV(v)
	}
	return f, nil
}

// parseTime accepts RFC3339 or a unix-seconds integer.
func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(sec, 0).UTC(), nil
	}
	return time.Time{}, errInvalid("time")
}

func paginate(in []model.Story, offset, limit int) []model.Story {
	if offset >= len(in) {
		return []model.Story{}
	}
	end := len(in)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return in[offset:end]
}

func parseInt(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func errInvalid(field string) error {
	return errors.New("invalid " + field)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
