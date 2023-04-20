package proxy_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/lb/internal/backend"
	"github.com/example/lb/internal/health"
	"github.com/example/lb/internal/pool"
	"github.com/example/lb/internal/proxy"
	"github.com/example/lb/internal/scheduler"
)

// idBackend returns an httptest backend that tags responses with id and serves
// 200 on /health while alive is nil or true.
func idBackend(t *testing.T, id string, alive *atomic.Bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if alive != nil && !alive.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("X-Backend-ID", id)
		fmt.Fprintf(w, "response from %s", id)
	}))
}

func newBackend(t *testing.T, rawURL string, onErr func(*backend.Backend, http.ResponseWriter, *http.Request, error)) *backend.Backend {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return backend.New(u, nil, onErr)
}

func get(t *testing.T, lb *httptest.Server) (string, int) {
	t.Helper()
	resp, err := http.Get(lb.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode
}

// End to end: 3 backends, 6 requests -> 1&4 / 2&5 / 3&6 mapping.
func TestRoundRobinEndToEnd(t *testing.T) {
	b1 := idBackend(t, "A", nil)
	b2 := idBackend(t, "B", nil)
	b3 := idBackend(t, "C", nil)
	defer b1.Close()
	defer b2.Close()
	defer b3.Close()

	backends := []*backend.Backend{
		newBackend(t, b1.URL, nil),
		newBackend(t, b2.URL, nil),
		newBackend(t, b3.URL, nil),
	}
	h := proxy.NewHandler(pool.New(backends), scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	var got []string
	for i := 0; i < 6; i++ {
		body, code := get(t, lb)
		if code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, code)
		}
		got = append(got, body)
	}

	want := []string{
		"response from A", "response from B", "response from C",
		"response from A", "response from B", "response from C",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request %d: got %q, want %q (full: %v)", i+1, got[i], want[i], got)
		}
	}
}

// Failover and recovery driven by the real health checker.
func TestFailureRoutingAndRecovery(t *testing.T) {
	var aliveB atomic.Bool
	aliveB.Store(true)
	b1 := idBackend(t, "A", nil)
	b2 := idBackend(t, "B", &aliveB)
	b3 := idBackend(t, "C", nil)
	defer b1.Close()
	defer b2.Close()
	defer b3.Close()

	backends := []*backend.Backend{
		newBackend(t, b1.URL, nil),
		newBackend(t, b2.URL, nil),
		newBackend(t, b3.URL, nil),
	}
	p := pool.New(backends)
	checker := health.New(p, "/health", 20*time.Millisecond, time.Second, nil)
	checker.CheckOnce(context.Background()) // initial confirm; all alive

	h := proxy.NewHandler(p, scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	// Take B offline and let the checker drop it.
	aliveB.Store(false)
	checker.CheckOnce(context.Background())
	if len(p.Healthy()) != 2 {
		t.Fatalf("expected 2 healthy after B down, got %d", len(p.Healthy()))
	}

	for i := 0; i < 12; i++ {
		body, code := get(t, lb)
		if code != http.StatusOK {
			t.Fatalf("request %d returned status %d (no errors should leak)", i, code)
		}
		if strings.Contains(body, "from B") {
			t.Fatalf("request %d routed to down backend B: %q", i, body)
		}
	}

	// Recover B.
	aliveB.Store(true)
	checker.CheckOnce(context.Background())
	if len(p.Healthy()) != 3 {
		t.Fatalf("expected 3 healthy after recovery, got %d", len(p.Healthy()))
	}
	sawB := false
	for i := 0; i < 12; i++ {
		body, _ := get(t, lb)
		if strings.Contains(body, "from B") {
			sawB = true
		}
	}
	if !sawB {
		t.Fatal("backend B never rejoined rotation after recovery")
	}
}

// All backends down -> client gets 503.
func TestAllDownReturns503(t *testing.T) {
	b1 := idBackend(t, "A", nil)
	defer b1.Close()
	be := newBackend(t, b1.URL, nil)
	be.SetAlive(false)

	h := proxy.NewHandler(pool.New([]*backend.Backend{be}), scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	_, code := get(t, lb)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when all down, got %d", code)
	}
}

// A backend that errors mid-request triggers the ErrorHandler: 502 to client
// and the backend marked down.
func TestErrorHandlerMarksDown(t *testing.T) {
	onErr := proxy.NewErrorHandler(nil)
	// Point at an address with nothing listening.
	be := newBackend(t, "http://127.0.0.1:1", onErr)

	h := proxy.NewHandler(pool.New([]*backend.Backend{be}), scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	_, code := get(t, lb)
	if code != http.StatusBadGateway {
		t.Fatalf("expected 502 on dead backend, got %d", code)
	}
	if be.Alive() {
		t.Fatal("backend should be marked down after mid-request failure")
	}
}

// Large body streams through the proxy intact.
func TestStreamingLargeBody(t *testing.T) {
	const size = 5 << 20 // 5 MiB
	payload := bytes.Repeat([]byte("x"), size)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write(payload)
	}))
	defer srv.Close()

	be := newBackend(t, srv.URL, nil)
	h := proxy.NewHandler(pool.New([]*backend.Backend{be}), scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	resp, err := http.Get(lb.URL + "/big")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != size {
		t.Fatalf("got %d bytes, want %d", len(body), size)
	}
}

// Hammer the handler concurrently while health flips backends up/down: no
// panic, no request served to a down backend. Run with -race.
func TestConcurrentLoadWithFlapping(t *testing.T) {
	var aliveB atomic.Bool
	aliveB.Store(true)
	b1 := idBackend(t, "A", nil)
	b2 := idBackend(t, "B", &aliveB)
	b3 := idBackend(t, "C", nil)
	defer b1.Close()
	defer b2.Close()
	defer b3.Close()

	backends := []*backend.Backend{
		newBackend(t, b1.URL, nil),
		newBackend(t, b2.URL, nil),
		newBackend(t, b3.URL, nil),
	}
	p := pool.New(backends)
	checker := health.New(p, "/health", 5*time.Millisecond, 500*time.Millisecond, nil)
	h := proxy.NewHandler(p, scheduler.NewRoundRobin(), nil)
	lb := httptest.NewServer(h)
	defer lb.Close()

	stop := make(chan struct{})
	var flapper sync.WaitGroup
	flapper.Add(1)
	go func() {
		defer flapper.Done()
		for {
			select {
			case <-stop:
				return
			default:
				aliveB.Store(!aliveB.Load())
				checker.CheckOnce(context.Background())
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var clients sync.WaitGroup
	for c := 0; c < 16; c++ {
		clients.Add(1)
		go func() {
			defer clients.Done()
			for i := 0; i < 60; i++ {
				resp, err := http.Get(lb.URL + "/")
				if err != nil {
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}
	clients.Wait()
	close(stop)
	flapper.Wait()
}
