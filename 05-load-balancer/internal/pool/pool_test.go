package pool

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

func TestHealthyExcludesDownBackends(t *testing.T) {
	bs := mustBackends(t, "http://a", "http://b", "http://c")
	p := New(bs)

	if got := len(p.Healthy()); got != 3 {
		t.Fatalf("all backends start alive: got %d healthy, want 3", got)
	}

	bs[1].SetAlive(false)
	healthy := p.Healthy()
	if len(healthy) != 2 {
		t.Fatalf("got %d healthy after marking one down, want 2", len(healthy))
	}
	for _, b := range healthy {
		if b.URL.String() == "http://b" {
			t.Fatal("down backend b appeared in healthy snapshot")
		}
	}

	bs[1].SetAlive(true)
	if got := len(p.Healthy()); got != 3 {
		t.Fatalf("recovery: got %d healthy, want 3", got)
	}
}

func TestHealthySnapshotIsCopy(t *testing.T) {
	bs := mustBackends(t, "http://a", "http://b")
	p := New(bs)
	s1 := p.Healthy()
	bs[0].SetAlive(false)
	// s1 was taken before the change; mutating the pool must not affect it.
	if len(s1) != 2 {
		t.Fatalf("earlier snapshot mutated: got %d, want 2", len(s1))
	}
}

// Concurrent SetAlive + Healthy must be race-free (run with -race).
func TestConcurrentSetAliveAndHealthy(t *testing.T) {
	bs := mustBackends(t, "http://a", "http://b", "http://c", "http://d")
	p := New(bs)

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				bs[i%len(bs)].SetAlive(i%2 == 0)
				i++
			}
		}
	}()

	var readers sync.WaitGroup
	for r := 0; r < 4; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for i := 0; i < 10000; i++ {
				_ = p.Healthy()
				_ = p.All()
			}
		}()
	}

	readers.Wait() // readers finish
	close(stop)    // then signal the writer to stop
	writer.Wait()
}
