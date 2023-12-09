// Package api is the REST API for posts. Times are RFC 3339 in UTC, and
// errors come back as {"error": "..."}.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/provider"
	"socialmedia/internal/queue"
	"socialmedia/internal/store"
)

type Server struct {
	store    store.Store
	queue    queue.Queue
	registry *provider.Registry
	clock    clock.Clock
	log      *slog.Logger
}

type Config struct {
	Store    store.Store
	Queue    queue.Queue
	Registry *provider.Registry
	Clock    clock.Clock
	Logger   *slog.Logger
}

func NewServer(cfg Config) *Server {
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Server{
		store:    cfg.Store,
		queue:    cfg.Queue,
		registry: cfg.Registry,
		clock:    cfg.Clock,
		log:      cfg.Logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /api/v1/posts", s.handleCreate)
	mux.HandleFunc("GET /api/v1/posts", s.handleList)
	mux.HandleFunc("GET /api/v1/posts/{id}", s.handleGet)
	mux.HandleFunc("PATCH /api/v1/posts/{id}", s.handlePatch)
	mux.HandleFunc("DELETE /api/v1/posts/{id}", s.handleDelete)
	mux.HandleFunc("POST /api/v1/posts/{id}/publish", s.handlePublishNow)

	// The web UI, if there is a ./web directory.
	mux.Handle("GET /", http.FileServer(http.Dir("web")))
	return mux
}

type createRequest struct {
	Author      string     `json:"author"`
	Body        string     `json:"body"`
	Visibility  *string    `json:"visibility"`
	Target      string     `json:"target"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

type patchRequest struct {
	Body        *string    `json:"body"`
	Visibility  *string    `json:"visibility"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

type listResponse struct {
	Items []*model.Post `json:"items"`
	Total int           `json:"total"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "database unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !s.decode(w, r, &req) {
		return
	}

	now := s.clock.Now()
	visibility := model.VisibilityPublic
	if req.Visibility != nil {
		visibility = model.Visibility(*req.Visibility)
	}

	p := &model.Post{
		ID:         uuid.NewString(),
		Author:     req.Author,
		Body:       req.Body,
		Visibility: visibility,
		Target:     req.Target,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if req.ScheduledAt != nil {
		t := req.ScheduledAt.UTC()
		p.ScheduledAt = &t
		p.Status = model.StatusScheduled
	} else {
		p.Status = model.StatusQueued
	}

	if err := p.Validate(now); err != nil {
		s.writeDomainError(w, err)
		return
	}
	prov, err := s.registry.Get(p.Target)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if err := prov.Validate(*p); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if err := s.store.Create(r.Context(), p); err != nil {
		s.writeError(w, http.StatusInternalServerError, "could not create post")
		s.log.Error("create post", "err", err)
		return
	}

	// Scheduled posts wait for the scheduler. If enqueueing fails the post is
	// still saved as queued, so it isn't lost.
	if p.Status == model.StatusQueued {
		if err := s.queue.Enqueue(r.Context(), queue.Message{PostID: p.ID}); err != nil {
			s.log.Warn("enqueue on create failed; post remains queued", "post_id", p.ID, "err", err)
		}
	}

	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ListFilter{
		Status: q.Get("status"),
		Target: q.Get("target"),
		Author: q.Get("author"),
		Limit:  atoiDefault(q.Get("limit"), 50),
		Offset: atoiDefault(q.Get("offset"), 0),
	}
	items, total, err := s.store.List(r.Context(), f)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "could not list posts")
		s.log.Error("list posts", "err", err)
		return
	}
	if items == nil {
		items = []*model.Post{}
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Total: total})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req patchRequest
	if !s.decode(w, r, &req) {
		return
	}

	now := s.clock.Now()

	current, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if !current.Status.Editable() {
		s.writeError(w, http.StatusConflict, "post is no longer editable")
		return
	}

	f := store.EditFields{}
	proposed := *current
	if req.Body != nil {
		f.Body = req.Body
		proposed.Body = *req.Body
	}
	if req.Visibility != nil {
		v := model.Visibility(*req.Visibility)
		if !model.ValidVisibility(v) {
			s.writeError(w, http.StatusBadRequest, "invalid visibility")
			return
		}
		f.Visibility = &v
		proposed.Visibility = v
	}
	if req.ScheduledAt != nil {
		t := req.ScheduledAt.UTC()
		if !t.After(now) {
			s.writeError(w, http.StatusBadRequest, "scheduled_at must be in the future")
			return
		}
		f.ScheduledAt = &t
		proposed.ScheduledAt = &t
	}

	// Check the edited post against the network's limits too.
	if prov, perr := s.registry.Get(proposed.Target); perr == nil {
		if verr := prov.Validate(proposed); verr != nil {
			s.writeDomainError(w, verr)
			return
		}
	}

	updated, err := s.store.Edit(r.Context(), id, f, now)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	_, err := s.store.Cancel(r.Context(), r.PathValue("id"), s.clock.Now())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublishNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.store.PublishNow(r.Context(), id, s.clock.Now())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if err := s.queue.Enqueue(r.Context(), queue.Message{PostID: p.ID}); err != nil {
		s.log.Warn("enqueue on publish-now failed; post remains queued", "post_id", p.ID, "err", err)
	}
	writeJSON(w, http.StatusAccepted, p)
}

func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// writeDomainError picks the HTTP status for an error from the model or store.
func (s *Server) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrValidation):
		s.writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "post not found")
	case errors.Is(err, store.ErrNotEditable):
		s.writeError(w, http.StatusConflict, "post is no longer editable")
	case errors.Is(err, store.ErrNotClaimable):
		s.writeError(w, http.StatusConflict, "post cannot be published in its current state")
	default:
		s.writeError(w, http.StatusInternalServerError, "internal error")
		s.log.Error("unhandled error", "err", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
