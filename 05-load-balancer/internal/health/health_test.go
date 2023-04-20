package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/lb/internal/backend"
	"github.com/example/lb/internal/pool"
)

func backendFor(t *testing.T, rawURL string) *backend.Backend {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return backend.New(u, nil, nil)
}

func TestProbeHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected probe path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := backendFor(t, srv.URL)
	c := New(pool.New([]*backend.Backend{b}), "/health", time.Second, time.Second, nil)
	if !c.probe(context.Background(), b) {
		t.Fatal("expected 200 health path to be healthy")
	}
}

func TestProbeUnhealthyStatuses(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusFound, http.StatusNotFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		b := backendFor(t, srv.URL)
		c := New(pool.New([]*backend.Backend{b}), "/health", time.Second, time.Second, nil)
		if c.probe(context.Background(), b) {
			t.Errorf("status %d should be unhealthy", code)
		}
		srv.Close()
	}
}

func TestProbeConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	b := backendFor(t, srv.URL)
	srv.Close() // nothing is listening now

	c := New(pool.New([]*backend.Backend{b}), "/health", time.Second, time.Second, nil)
	if c.probe(context.Background(), b) {
		t.Fatal("connection refused should be unhealthy")
	}
}

func TestProbeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := backendFor(t, srv.URL)
	c := New(pool.New([]*backend.Backend{b}), "/health", time.Second, 20*time.Millisecond, nil)
	if c.probe(context.Background(), b) {
		t.Fatal("slow backend should time out as unhealthy")
	}
}

// CheckOnce flips state on transitions: a backend that starts serving 500
// becomes unhealthy, and recovers to healthy when it serves 200 again.
func TestCheckOnceTransitions(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	b := backendFor(t, srv.URL)
	p := pool.New([]*backend.Backend{b})
	c := New(p, "/health", time.Second, time.Second, nil)

	c.CheckOnce(context.Background())
	if !b.Alive() {
		t.Fatal("backend should be alive after 200 probe")
	}

	healthy.Store(false)
	c.CheckOnce(context.Background())
	if b.Alive() {
		t.Fatal("backend should be dropped after 500 probe")
	}
	if len(p.Healthy()) != 0 {
		t.Fatal("pool should report no healthy backends")
	}

	healthy.Store(true)
	c.CheckOnce(context.Background())
	if !b.Alive() {
		t.Fatal("backend should recover after 200 probe")
	}
	if len(p.Healthy()) != 1 {
		t.Fatal("pool should report the recovered backend")
	}
}

// Run probes immediately and keeps probing until the context is cancelled.
func TestRunStopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := backendFor(t, srv.URL)
	b.SetAlive(false)
	c := New(pool.New([]*backend.Backend{b}), "/health", 50*time.Millisecond, time.Second, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	// Initial probe should mark it healthy quickly.
	deadline := time.After(time.Second)
	for !b.Alive() {
		select {
		case <-deadline:
			t.Fatal("backend never became healthy")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
