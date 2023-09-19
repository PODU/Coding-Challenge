package store

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis returns a Store backed by miniredis (in-process, offline) unless
// REDIS_ADDR is set, in which case it targets that real Redis server. This
// keeps the default test run fully offline and green while allowing an opt-in
// integration run against a real server.
func newTestRedis(t *testing.T) (*RedisStore, func()) {
	t.Helper()
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		client := redis.NewClient(&redis.Options{Addr: addr})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			t.Skipf("REDIS_ADDR=%s unreachable: %v", addr, err)
		}
		// Use a unique prefix per test to avoid cross-test interference.
		s := NewRedisStore(client, "rltest:"+t.Name()+":", time.Minute)
		return s, func() { _ = client.FlushDB(context.Background()); _ = client.Close() }
	}
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := NewRedisStore(client, "rl:", time.Minute)
	return s, func() { _ = client.Close(); mr.Close() }
}

func TestRedisStore_PersistsState(t *testing.T) {
	s, cleanup := newTestRedis(t)
	defer cleanup()

	for i := 1; i <= 3; i++ {
		dec, err := s.Do(context.Background(), "k", countStep)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if dec.Remaining != i {
			t.Fatalf("expected count %d, got %d", i, dec.Remaining)
		}
	}
}

func TestRedisStore_SharedAcrossInstances(t *testing.T) {
	// Two RedisStore values (simulating two server processes) over one backend
	// must observe the same shared state.
	mr := miniredis.RunT(t)
	defer mr.Close()
	c1 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c2 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer c1.Close()
	defer c2.Close()
	a := NewRedisStore(c1, "rl:", time.Minute)
	b := NewRedisStore(c2, "rl:", time.Minute)

	dec, _ := a.Do(context.Background(), "k", countStep) // count -> 1
	if dec.Remaining != 1 {
		t.Fatalf("instance A: expected 1, got %d", dec.Remaining)
	}
	dec, _ = b.Do(context.Background(), "k", countStep) // count -> 2 (shared)
	if dec.Remaining != 2 {
		t.Fatalf("instance B should see shared state, expected 2, got %d", dec.Remaining)
	}
}

func TestRedisStore_ConcurrentAtomicity(t *testing.T) {
	s, cleanup := newTestRedis(t)
	defer cleanup()

	const n = 30
	var wg sync.WaitGroup
	var failures int64
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := s.Do(context.Background(), "k", countStep); err != nil {
				atomic.AddInt64(&failures, 1)
			}
		}()
	}
	wg.Wait()
	if failures != 0 {
		t.Fatalf("%d Do calls failed", failures)
	}

	dec, _ := s.Do(context.Background(), "k", func(st *State) Decision {
		return Decision{Remaining: st.Count}
	})
	// n increments + the final read (which also increments). Read step above
	// does not increment, so expect exactly n.
	if dec.Remaining != n {
		t.Fatalf("expected %d atomic increments, got %d (lost updates)", n, dec.Remaining)
	}
}
