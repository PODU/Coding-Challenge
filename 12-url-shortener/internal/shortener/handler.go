package shortener

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MaxURLLength is the longest URL /shorten accepts.
const MaxURLLength = 2048

const maxBodyBytes = 64 * 1024

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Key      string `json:"key"`
	LongURL  string `json:"long_url"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Handler serves the shortener API on top of a Store.
type Handler struct {
	store   Store
	baseURL string // no trailing slash
	gui     http.Handler
}

// SetGUI sets the handler for GET /. Without one, a plain-text note is served.
func (h *Handler) SetGUI(g http.Handler) { h.gui = g }

// NewHandler returns a Handler that builds short links under baseURL, e.g.
// "http://localhost:8080".
func NewHandler(store Store, baseURL string) *Handler {
	return &Handler{
		store:   store,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Routes uses Go 1.22 method patterns, so other methods get a 405 for free.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.handleShorten)
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("GET /{key}", h.handleRedirect)
	mux.HandleFunc("DELETE /{key}", h.handleDelete)
	mux.HandleFunc("GET /{$}", h.handleRoot)
	return mux
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if h.gui != nil {
		h.gui.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "URL shortener. POST /shorten {\"url\":\"...\"}\n")
}

func (h *Handler) handleShorten(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req shortenRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	longURL := strings.TrimSpace(req.URL)
	if err := validateURL(longURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Shortening the same URL twice returns the same key.
	if key, ok := h.store.FindByURL(longURL); ok {
		writeJSON(w, http.StatusOK, h.response(key, longURL))
		return
	}

	key := GenerateKey(longURL, func(candidate string) bool {
		_, taken := h.store.Get(candidate)
		return taken
	})
	if err := h.store.Save(key, longURL); err != nil {
		writeError(w, http.StatusInternalServerError, "could not persist URL")
		return
	}
	writeJSON(w, http.StatusCreated, h.response(key, longURL))
}

func (h *Handler) handleRedirect(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	longURL, ok := h.store.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown short key")
		return
	}
	http.Redirect(w, r, longURL, http.StatusFound)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !h.store.Delete(key) {
		writeError(w, http.StatusNotFound, "unknown short key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) response(key, longURL string) shortenResponse {
	return shortenResponse{
		Key:      key,
		LongURL:  longURL,
		ShortURL: h.baseURL + "/" + key,
	}
}

func validateURL(raw string) error {
	if raw == "" {
		return badURLError("url is required")
	}
	if len(raw) > MaxURLLength {
		return badURLError("url exceeds maximum length")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return badURLError("url is not parseable")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return badURLError("url must use http or https scheme")
	}
	if u.Host == "" {
		return badURLError("url must include a host")
	}
	return nil
}

type badURLError string

func (e badURLError) Error() string { return string(e) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
