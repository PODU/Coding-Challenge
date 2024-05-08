package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"pastebin/internal/paste"
)

type createRequest struct {
	Content   string `json:"content"`
	ExpiresIn string `json:"expires_in"`
	Language  string `json:"language"`
}

type createResponse struct {
	ID        string  `json:"id"`
	URL       string  `json:"url"`
	RawURL    string  `json:"raw_url"`
	DeleteKey string  `json:"delete_key"`
	ExpiresAt *string `json:"expires_at"`
}

type getResponse struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	Language  string  `json:"language"`
	CreatedAt string  `json:"created_at"`
	ExpiresAt *string `json:"expires_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// apiCreate handles POST /api/pastes.
func (s *Server) apiCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		// A body that is too large surfaces here as well.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "paste too large")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	res, err := s.svc.Create(r.Context(), paste.CreateParams{
		Content:   req.Content,
		Language:  req.Language,
		ExpiresIn: req.ExpiresIn,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	p := res.Paste
	writeJSON(w, http.StatusCreated, createResponse{
		ID:        p.ID,
		URL:       s.baseURL + "/" + p.ID,
		RawURL:    s.baseURL + "/" + p.ID + "/raw",
		DeleteKey: res.DeleteKey,
		ExpiresAt: rfc3339Ptr(p.ExpiresAt),
	})
}

// apiGet handles GET /api/pastes/{id}.
func (s *Server) apiGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.svc.Get(r.Context(), id)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, getResponse{
		ID:        p.ID,
		Content:   p.Content,
		Language:  p.Language,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: rfc3339Ptr(p.ExpiresAt),
	})
}

// apiDelete handles DELETE /api/pastes/{id} with the X-Delete-Key header.
func (s *Server) apiDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := r.Header.Get("X-Delete-Key")
	if err := s.svc.Delete(r.Context(), id, key); err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// health handles GET /healthz and pings the store.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeServiceError maps paste errors to HTTP statuses.
func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paste.ErrInvalid):
		writeAPIError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, paste.ErrTooLarge):
		writeAPIError(w, http.StatusRequestEntityTooLarge, "paste too large")
	case errors.Is(err, paste.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, "invalid delete key")
	case errors.Is(err, paste.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "paste not found")
	case errors.Is(err, paste.ErrExpired):
		writeAPIError(w, http.StatusGone, "paste expired")
	default:
		s.log.Error("internal error", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
	}
}
