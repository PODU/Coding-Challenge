package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"loadtester/internal/stats"
)

// sliceProvider yields a fixed list of URLs.
type sliceProvider struct{ urls []string }

func (s sliceProvider) N() int          { return len(s.urls) }
func (s sliceProvider) At(i int) string { return s.urls[i%len(s.urls)] }

// fakeRequester records call count and peak concurrency, and returns canned
// results. It blocks briefly so concurrency is observable.
type fakeRequester struct {
	calls    int64
	inFlight int64
	maxSeen  int64
	delay    time.Duration
	seqs     sync.Map // seq -> struct{}, to verify each job processed once
}

func (f *fakeRequester) Do(ctx context.Context, job Job) stats.Sample {
	atomic.AddInt64(&f.calls, 1)
	cur := atomic.AddInt64(&f.inFlight, 1)
	for {
		old := atomic.LoadInt64(&f.maxSeen)
		if cur <= old || atomic.CompareAndSwapInt64(&f.maxSeen, old, cur) {
			break
		}
	}
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	atomic.AddInt64(&f.inFlight, -1)
	f.seqs.Store(job.Seq, struct{}{})
	return stats.Sample{Seq: job.Seq, StatusCode: 200, Total: time.Millisecond}
}

func TestRunProcessesAllJobs(t *testing.T) {
	const n, c = 50, 4
	urls := make([]string, n)
	for i := range urls {
		urls[i] = "http://fake/" + string(rune('a'+i%26))
	}
	f := &fakeRequester{delay: 2 * time.Millisecond}

	collector, wall := Run(context.Background(), sliceProvider{urls}, c, f)

	if got := collector.Len(); got != n {
		t.Errorf("collected %d results, want %d", got, n)
	}
	if got := atomic.LoadInt64(&f.calls); got != n {
		t.Errorf("Do called %d times, want %d", got, n)
	}
	if max := atomic.LoadInt64(&f.maxSeen); max > c {
		t.Errorf("peak concurrency %d exceeded c=%d", max, c)
	}
	if max := atomic.LoadInt64(&f.maxSeen); max < 2 {
		t.Errorf("peak concurrency %d, expected workers to run in parallel (>=2)", max)
	}
	// Every seq 0..n-1 must have been processed exactly once.
	for i := 0; i < n; i++ {
		if _, ok := f.seqs.Load(i); !ok {
			t.Errorf("seq %d was never processed", i)
		}
	}
	if wall <= 0 {
		t.Errorf("wall clock = %v, want > 0", wall)
	}
}

func TestRunSingleWorker(t *testing.T) {
	f := &fakeRequester{}
	collector, _ := Run(context.Background(), sliceProvider{[]string{"http://x"}}, 1, f)
	if collector.Len() != 1 {
		t.Errorf("collected %d, want 1", collector.Len())
	}
	if max := atomic.LoadInt64(&f.maxSeen); max != 1 {
		t.Errorf("peak concurrency %d, want 1 for single worker", max)
	}
}

// --- Integration against an in-process httptest server (no network) ---

func TestIntegration200Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello body"))
	}))
	defer srv.Close()

	const n, c = 50, 4
	p := sliceProvider{repeat(srv.URL, n)}
	req := NewHTTPRequester(http.MethodGet, nil, "", 5*time.Second, c)

	collector, wall := Run(context.Background(), p, c, req)
	s := collector.Summarize(wall)

	if s.Total != n {
		t.Errorf("Total = %d, want %d", s.Total, n)
	}
	if s.Success != n {
		t.Errorf("Success = %d, want %d", s.Success, n)
	}
	if s.Failed != 0 {
		t.Errorf("Failed = %d, want 0", s.Failed)
	}
	if s.StatusCodes["200"] != n {
		t.Errorf("StatusCodes[200] = %d, want %d", s.StatusCodes["200"], n)
	}
	// Timing sanity: TTFB <= TTLB <= Total (max across all).
	if s.TotalStats.Count != n {
		t.Errorf("TotalStats.Count = %d, want %d", s.TotalStats.Count, n)
	}
	if !(s.TTFBStats.Max <= s.TTLBStats.Max+time.Millisecond) {
		t.Errorf("TTFB max %v should be <= TTLB max %v", s.TTFBStats.Max, s.TTLBStats.Max)
	}
}

func TestIntegrationServerDelay(t *testing.T) {
	const delay = 20 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := sliceProvider{repeat(srv.URL, 5)}
	req := NewHTTPRequester(http.MethodGet, nil, "", 5*time.Second, 2)
	collector, wall := Run(context.Background(), p, 2, req)
	s := collector.Summarize(wall)

	if s.TotalStats.Min < delay {
		t.Errorf("min total latency %v, want >= server delay %v", s.TotalStats.Min, delay)
	}
}

func TestIntegration500Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := sliceProvider{repeat(srv.URL, 10)}
	req := NewHTTPRequester(http.MethodGet, nil, "", 5*time.Second, 3)
	collector, wall := Run(context.Background(), p, 3, req)
	s := collector.Summarize(wall)

	if s.Success != 0 {
		t.Errorf("Success = %d, want 0", s.Success)
	}
	if s.Failed != 10 {
		t.Errorf("Failed = %d, want 10", s.Failed)
	}
	if s.StatusCodes["500"] != 10 {
		t.Errorf("StatusCodes[500] = %d, want 10", s.StatusCodes["500"])
	}
}

func TestIntegrationTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	p := sliceProvider{repeat(srv.URL, 4)}
	req := NewHTTPRequester(http.MethodGet, nil, "", 30*time.Millisecond, 2)
	collector, wall := Run(context.Background(), p, 2, req)
	s := collector.Summarize(wall)

	if s.Timeouts != 4 {
		t.Errorf("Timeouts = %d, want 4", s.Timeouts)
	}
	if s.StatusCodes["timeout"] != 4 {
		t.Errorf("StatusCodes[timeout] = %d, want 4", s.StatusCodes["timeout"])
	}
	if s.TotalStats.Count != 0 {
		t.Errorf("timed-out requests must not feed latency stats, got Count=%d", s.TotalStats.Count)
	}
}

func TestIntegrationConnectionRefused(t *testing.T) {
	// Start then immediately close a server to get an unused address.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	p := sliceProvider{repeat(url, 3)}
	req := NewHTTPRequester(http.MethodGet, nil, "", 2*time.Second, 2)
	collector, wall := Run(context.Background(), p, 2, req)
	s := collector.Summarize(wall)

	if s.Failed != 3 {
		t.Errorf("Failed = %d, want 3", s.Failed)
	}
	if s.Errors != 3 {
		t.Errorf("Errors = %d, want 3 (non-timeout transport errors)", s.Errors)
	}
}

func TestIntegrationHeadersAndBody(t *testing.T) {
	var gotAuth, gotBody, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := http.Header{}
	h.Set("Authorization", "Bearer xyz")
	p := sliceProvider{[]string{srv.URL}}
	req := NewHTTPRequester(http.MethodPost, h, "payload-data", 5*time.Second, 1)
	Run(context.Background(), p, 1, req)

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("Authorization = %q, want 'Bearer xyz'", gotAuth)
	}
	if gotBody != "payload-data" {
		t.Errorf("body = %q, want 'payload-data'", gotBody)
	}
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}
