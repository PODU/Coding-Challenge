// Package server implements the dropd REST API and its per-user content store.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"dropbox/internal/wire"
)

// Server serves the store over HTTP, behind bearer-token auth.
type Server struct {
	store *Store
	authn *Auth
}

// New returns a Server that keeps its data under dataDir.
func New(dataDir string, cfg Config) *Server {
	return &Server{store: NewStore(dataDir), authn: NewAuth(cfg)}
}

// Store returns the underlying store.
func (s *Server) Store() *Store { return s.store }

// Auth returns the authenticator.
func (s *Server) Auth() *Auth { return s.authn }

// Handler returns the HTTP handler for the v1 API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /v1/files", s.protected(s.handleList))
	mux.HandleFunc("GET /v1/files/{path...}", s.protected(s.handleGet))
	mux.HandleFunc("PUT /v1/files/{path...}", s.protected(s.handlePut))
	mux.HandleFunc("DELETE /v1/files/{path...}", s.protected(s.handleDelete))
	return mux
}

func (s *Server) protected(h func(w http.ResponseWriter, r *http.Request, user string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authn.User(bearer(r))
		if !ok {
			writeErr(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}
		h(w, r, user)
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req wire.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	tok, ok := s.authn.Login(req.User, req.Password)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, wire.LoginResponse{Token: tok})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request, user string) {
	files, err := s.store.List(user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wire.ListResponse{Files: files})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, user string) {
	f, fi, err := s.store.Get(user, r.PathValue("path"))
	if err != nil {
		s.writeStoreErr(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("X-Content-SHA256", fi.Hash)
	w.Header().Set("X-Version", strconv.FormatInt(fi.Version, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size, 10))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, user string) {
	fi, err := s.store.Put(user, r.PathValue("path"), parseIfMatch(r), r.Header.Get("X-Content-SHA256"), r.Body)
	if err != nil {
		s.writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fi)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, user string) {
	if err := s.store.Delete(user, r.PathValue("path"), parseIfMatch(r)); err != nil {
		s.writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIfMatch(r *http.Request) int64 {
	h := r.Header.Get("If-Match")
	if h == "" {
		return -1
	}
	h = strings.Trim(h, "\"")
	v, err := strconv.ParseInt(h, 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func (s *Server) writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrVersionConflict):
		writeErr(w, http.StatusConflict, "version conflict")
	case errors.Is(err, ErrHashMismatch):
		writeErr(w, http.StatusUnprocessableEntity, "sha-256 mismatch")
	case errors.Is(err, ErrBadPath):
		writeErr(w, http.StatusBadRequest, "bad path")
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, wire.ErrorResponse{Error: msg})
}
