package scheduler

import (
	"net/url"
	"sync"
	"testing"

	"github.com/example/lb/internal/backend"
)

func mustBackends(t *testing.T, raws ...string) []*backend.Backend {
	t.Helper()
	out := make([]*backend.Backend, 0, len(raws))
	for _, raw := range raws {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		out = append(out, backend.New(u, nil, nil))
	}
	return out
}

// Backends A,B,C and 6 requests -> A,B,C,A,B,C.
func TestRoundRobinCanonicalOrder(t *testing.T) {
	bs := mustBackends(t, "http://a:8080", "http://b:8080", "http://c:8080")
	rr := NewRoundRobin()

	want := []string{
		"http://a:8080", "http://b:8080", "http://c:8080",
		"http://a:8080", "http://b:8080", "http://c:8080",
	}
	for i, w := range want {
		got := rr.Next(nil, bs).URL.String()
		if got != w {
			t.Fatalf("request %d: got %s, want %s", i+1, got, w)
		}
	}
}

func TestRoundRobinEmptyReturnsNil(t *testing.T) {
	rr := NewRoundRobin()
	if got := rr.Next(nil, nil); got != nil {
		t.Fatalf("expected nil on empty healthy set, got %v", got)
	}
	if got := rr.Next(nil, []*backend.Backend{}); got != nil {
		t.Fatalf("expected nil on empty slice, got %v", got)
	}
}

func TestRoundRobinBalancedOver10k(t *testing.T) {
	bs := mustBackends(t, "http://a:8080", "http://b:8080", "http://c:8080")
	rr := NewRoundRobin()

	counts := map[string]int{}
	const total = 30000
	for i := 0; i < total; i++ {
		counts[rr.Next(nil, bs).URL.String()]++
	}
	for _, b := range bs {
		if got := counts[b.URL.String()]; got != total/len(bs) {
			t.Fatalf("backend %s got %d, want %d", b.URL, got, total/len(bs))
		}
	}
}

// Concurrent Next calls must not race and must distribute all selections.
func TestRoundRobinConcurrent(t *testing.T) {
	bs := mustBackends(t, "http://a:8080", "http://b:8080", "http://c:8080", "http://d:8080")
	rr := NewRoundRobin()

	const goroutines = 50
	const perG = 2000
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[string]int{}

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := map[string]int{}
			for i := 0; i < perG; i++ {
				local[rr.Next(nil, bs).URL.String()]++
			}
			mu.Lock()
			for k, v := range local {
				counts[k] += v
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	total := goroutines * perG
	if total%len(bs) == 0 {
		for _, b := range bs {
			if got := counts[b.URL.String()]; got != total/len(bs) {
				t.Fatalf("backend %s got %d, want %d", b.URL, got, total/len(bs))
			}
		}
	}
}
