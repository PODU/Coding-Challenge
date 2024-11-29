package checker

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/uptime/internal/clock"
	"github.com/example/uptime/internal/monitor"
)

func httpMonitor(target, method string) monitor.Monitor {
	return monitor.Monitor{
		ID:           "m",
		Kind:         monitor.KindHTTP,
		Target:       target,
		Method:       method,
		Timeout:      2 * time.Second,
		ExpectStatus: []monitor.StatusRange{{Lo: 200, Hi: 299}},
	}
}

func TestHTTPChecker_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewHTTP(clock.New())
	res := c.Check(context.Background(), httpMonitor(srv.URL, "HEAD"))
	if res.Status != monitor.StatusUp {
		t.Fatalf("status = %v, err = %q", res.Status, res.Err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status code = %d", res.StatusCode)
	}
}

func TestHTTPChecker_DownOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := NewHTTP(clock.New())
	res := c.Check(context.Background(), httpMonitor(srv.URL, "GET"))
	if res.Status != monitor.StatusDown {
		t.Fatalf("expected down for 500, got %v", res.Status)
	}
	if res.StatusCode != 500 {
		t.Fatalf("status code = %d", res.StatusCode)
	}
}

func TestHTTPChecker_TimeoutIsDown(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // block until released, forcing a context deadline
	}))
	defer srv.Close()
	defer close(release)

	m := httpMonitor(srv.URL, "HEAD")
	m.Timeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	c := NewHTTP(clock.New())
	res := c.Check(ctx, m)
	if res.Status != monitor.StatusDown {
		t.Fatalf("expected down on timeout, got %v", res.Status)
	}
	if res.Err != "timeout" {
		t.Fatalf("err = %q, want timeout", res.Err)
	}
}

func TestHTTPChecker_GET_CapturesTTFB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay the first byte too: on Windows the monotonic clock ticks
		// coarsely, so an instant loopback response can measure TTFB as 0.
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush() // send headers (first byte)
		}
		time.Sleep(40 * time.Millisecond) // delay body
		_, _ = w.Write([]byte("delayed-body"))
	}))
	defer srv.Close()

	m := httpMonitor(srv.URL, "GET")
	m.ExpectStatus = []monitor.StatusRange{{Lo: 200, Hi: 200}}
	c := NewHTTP(clock.New())
	res := c.Check(context.Background(), m)
	if res.Status != monitor.StatusUp {
		t.Fatalf("status = %v, err = %q", res.Status, res.Err)
	}
	if res.TTFB <= 0 {
		t.Fatal("expected TTFB > 0 for GET")
	}
	if res.Latency < res.TTFB {
		t.Fatalf("latency (%s) should be >= TTFB (%s)", res.Latency, res.TTFB)
	}
}

func TestHTTPChecker_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // now nothing is listening

	c := NewHTTP(clock.New())
	res := c.Check(context.Background(), httpMonitor(url, "HEAD"))
	if res.Status != monitor.StatusDown {
		t.Fatalf("expected down for refused connection, got %v (%q)", res.Status, res.Err)
	}
}

func TestHTTPChecker_LatencyThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	m := httpMonitor(srv.URL, "HEAD")
	m.LatencyThreshold = 1 * time.Millisecond
	c := NewHTTP(clock.New())
	res := c.Check(context.Background(), m)
	if res.Status != monitor.StatusDown {
		t.Fatalf("expected down for latency breach, got %v", res.Status)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status code should still be 200, got %d", res.StatusCode)
	}
}

func TestTCPChecker_OpenAndClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	c := NewTCP(clock.New())
	m := monitor.Monitor{ID: "t", Kind: monitor.KindTCP, Target: addr, Timeout: time.Second}

	res := c.Check(context.Background(), m)
	if res.Status != monitor.StatusUp {
		t.Fatalf("expected up for open port, got %v (%q)", res.Status, res.Err)
	}

	_ = ln.Close()
	// Retry briefly to avoid a TIME_WAIT race where the port still accepts.
	got := monitor.StatusUp
	for i := 0; i < 20; i++ {
		res = c.Check(context.Background(), m)
		got = res.Status
		if got == monitor.StatusDown {
			break
		}
	}
	if got != monitor.StatusDown {
		t.Fatalf("expected down for closed port, got %v", got)
	}
}

func TestMultiDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	m := NewMulti(clock.New())
	res := m.Check(context.Background(), httpMonitor(srv.URL, "HEAD"))
	if res.Status != monitor.StatusUp {
		t.Fatalf("multi http dispatch failed: %v", res.Status)
	}
}
