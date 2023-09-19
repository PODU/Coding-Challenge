package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// TestDistributed_SharedLimitAcrossInstances: two server instances share one Redis with a 60/min limit. All 60 requests go
// to instance A; instance B must then reject request 61 for the same key.
// Backed by miniredis so it runs offline; set REDIS_ADDR to use a real server.
func TestDistributed_SharedLimitAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	clk := clock.NewFakeClock(time.Unix(60, 0)) // aligned to a window boundary.

	newInstance := func() Limiter {
		c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = c.Close() })
		st := store.NewRedisStore(c, "rl:", time.Minute)
		l, err := New(FixedWindowAlgo, Config{Limit: 60, Window: time.Minute}, st, clk)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return l
	}

	instanceA := newInstance()
	instanceB := newInstance()

	for i := 0; i < 60; i++ {
		dec, err := instanceA.Allow(context.Background(), "client-ip")
		if err != nil {
			t.Fatalf("A request %d: %v", i+1, err)
		}
		if !dec.Allowed {
			t.Fatalf("A request %d: expected allowed", i+1)
		}
	}

	dec, err := instanceB.Allow(context.Background(), "client-ip")
	if err != nil {
		t.Fatalf("B request: %v", err)
	}
	if dec.Allowed {
		t.Fatal("instance B should reject request 61 for the shared key")
	}
}
