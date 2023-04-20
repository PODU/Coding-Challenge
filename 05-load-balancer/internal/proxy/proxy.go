// Package proxy is the http.Handler that forwards each request to a backend
// chosen by the scheduler.
package proxy

import (
	"log/slog"
	"net/http"

	"github.com/example/lb/internal/backend"
	"github.com/example/lb/internal/pool"
	"github.com/example/lb/internal/scheduler"
)

// Handler is the load balancer's HTTP handler.
type Handler struct {
	pool      *pool.BackendPool
	scheduler scheduler.Scheduler
	logger    *slog.Logger
}

// NewHandler returns a Handler that picks from p using s.
func NewHandler(p *pool.BackendPool, s scheduler.Scheduler, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{pool: p, scheduler: s, logger: logger}
}

// ServeHTTP proxies r to a healthy backend, or answers 503 if there is none.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := h.scheduler.Next(r, h.pool.Healthy())
	if b == nil {
		h.logger.Warn("no healthy backends",
			"method", r.Method, "path", r.URL.Path, "client", r.RemoteAddr)
		http.Error(w, "no healthy backends available", http.StatusServiceUnavailable)
		return
	}

	h.logger.Info("routing request",
		"method", r.Method,
		"path", r.URL.Path,
		"client", r.RemoteAddr,
		"backend", b.URL.String(),
	)

	b.IncConns()
	defer b.DecConns()
	b.Proxy.ServeHTTP(w, r)
}

// NewErrorHandler returns the onError callback for backend.New. A failed
// request marks the backend down until the health checker sees it recover, and
// the client gets a 502.
func NewErrorHandler(logger *slog.Logger) func(*backend.Backend, http.ResponseWriter, *http.Request, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return func(b *backend.Backend, w http.ResponseWriter, r *http.Request, err error) {
		b.SetAlive(false)
		logger.Error("backend request failed",
			"backend", b.URL.String(),
			"path", r.URL.Path,
			"err", err,
		)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}
}
