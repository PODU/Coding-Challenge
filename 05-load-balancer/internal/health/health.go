// Package health periodically probes every backend and updates its alive flag.
// Each round probes all backends in parallel, so one slow backend doesn't hold
// up the rest.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/lb/internal/backend"
	"github.com/example/lb/internal/pool"
)

// Checker probes backends on a fixed interval.
type Checker struct {
	pool     *pool.BackendPool
	path     string
	interval time.Duration
	timeout  time.Duration
	client   *http.Client
	logger   *slog.Logger
}

// New returns a Checker that requests path on each backend every interval,
// giving each probe at most timeout.
func New(p *pool.BackendPool, path string, interval, timeout time.Duration, logger *slog.Logger) *Checker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Checker{
		pool:     p,
		path:     path,
		interval: interval,
		timeout:  timeout,
		client: &http.Client{
			Timeout: timeout,
			// Never follow redirects: a 3xx from the health path is unhealthy.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// Run probes once straight away and then every interval until ctx is done.
func (c *Checker) Run(ctx context.Context) {
	c.CheckOnce(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CheckOnce(ctx)
		}
	}
}

// CheckOnce runs one probe round and waits for it to finish. Only actual
// up/down changes are logged.
func (c *Checker) CheckOnce(ctx context.Context) {
	backends := c.pool.All()
	done := make(chan struct{}, len(backends))

	for _, b := range backends {
		go func(b *backend.Backend) {
			defer func() { done <- struct{}{} }()
			ok := c.probe(ctx, b)
			if ok != b.Alive() {
				b.SetAlive(ok)
				c.logger.Info("backend health transition",
					"backend", b.URL.String(),
					"state", stateString(ok),
				)
			}
		}(b)
	}

	for range backends {
		<-done
	}
}

// probe reports whether GET <backend><path> returned 200.
func (c *Checker) probe(ctx context.Context, b *backend.Backend) bool {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	target := b.URL.JoinPath(c.path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func stateString(alive bool) string {
	if alive {
		return "healthy"
	}
	return "unhealthy"
}
