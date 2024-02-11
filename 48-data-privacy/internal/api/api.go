// Package api is the vault's HTTP API: the tokenize and detokenize endpoints,
// bearer-key auth with scopes, and the usual middleware.
package api

import (
	"log/slog"
	"net/http"

	"datavault/internal/vault"
)

// MaxBodyBytes limits the size of request bodies.
const MaxBodyBytes = 1 << 20

// Server is the HTTP API.
type Server struct {
	vault   *vault.Vault
	keys    *KeyStore
	logger  *slog.Logger
	maxBody int64
}

// NewServer returns a Server. A nil logger discards everything.
func NewServer(v *vault.Vault, ks *KeyStore, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(discard{}, nil))
	}
	return &Server{vault: v, keys: ks, logger: logger, maxBody: MaxBodyBytes}
}

// Handler returns the API with all middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// No auth on health checks.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	mux.Handle("POST /tokenize", s.requireScope(ScopeTokenize, http.HandlerFunc(s.handleTokenize)))
	mux.Handle("POST /detokenize", s.requireScope(ScopeDetokenize, http.HandlerFunc(s.handleDetokenize)))

	var h http.Handler = mux
	h = s.securityHeaders(h)
	h = s.accessLog(h)
	h = s.requestID(h)
	h = s.recoverPanic(h)
	return h
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
