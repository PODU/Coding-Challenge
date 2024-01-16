package cache

import (
	"sync"
	"testing"
	"time"

	"dns-forwarder/internal/dnsmsg"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// answer builds a raw A response with the given TTL for example.com.
func answer(ttl uint32) []byte {
	m := &dnsmsg.Message{
		Header:    dnsmsg.Header{ID: 1, QR: true, RA: true, RCode: dnsmsg.RCodeNoError},
		Questions: []dnsmsg.Question{{Name: "example.com", Type: dnsmsg.TypeA, Class: dnsmsg.ClassIN}},
		Answers: []dnsmsg.ResourceRecord{
			{Name: "example.com", Type: dnsmsg.TypeA, Class: dnsmsg.ClassIN, TTL: ttl, RData: []byte{1, 2, 3, 4}},
		},
	}
	b, err := dnsmsg.Encode(m)
	if err != nil {
		panic(err)
	}
	return b
}

func servedTTL(t *testing.T, raw []byte) uint32 {
	t.Helper()
	m, err := dnsmsg.Decode(raw)
	if err != nil {
		t.Fatalf("decode served: %v", err)
	}
	if len(m.Answers) == 0 {
		t.Fatalf("served message has no answers")
	}
	return m.Answers[0].TTL
}

var key = NormalizeKey("example.com", dnsmsg.TypeA, dnsmsg.ClassIN)

func TestPutGetHit(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk})
	c.Put(key, answer(100), 100)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit")
	}
	if ttl := servedTTL(t, got); ttl != 100 {
		t.Errorf("served TTL = %d, want 100", ttl)
	}
}

func TestGetRemainingTTL(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk})
	c.Put(key, answer(100), 100)

	clk.Advance(30 * time.Second)
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit mid-window")
	}
	if ttl := servedTTL(t, got); ttl != 70 {
		t.Errorf("remaining TTL = %d, want 70", ttl)
	}
}

func TestGetExpired(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk})
	c.Put(key, answer(100), 100)

	clk.Advance(101 * time.Second)
	if _, ok := c.Get(key); ok {
		t.Fatal("expected miss after expiry")
	}
	if c.Len() != 0 {
		t.Errorf("expired entry not evicted, len=%d", c.Len())
	}
}

func TestZeroTTLNotCached(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk})
	c.Put(key, answer(0), 0)
	if _, ok := c.Get(key); ok {
		t.Fatal("TTL 0 should not be cached")
	}
}

func TestMaxTTLCap(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk, MaxTTL: 60})
	c.Put(key, answer(1000), 1000)
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit")
	}
	if ttl := servedTTL(t, got); ttl != 60 {
		t.Errorf("capped TTL = %d, want 60", ttl)
	}
}

func TestMinTTLFloor(t *testing.T) {
	clk := newFakeClock()
	c := NewWithOptions(Options{Clock: clk, MinTTL: 30})
	c.Put(key, answer(5), 5)
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit")
	}
	if ttl := servedTTL(t, got); ttl != 30 {
		t.Errorf("floored TTL = %d, want 30", ttl)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New() // wall clock
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Put(key, answer(300), 300)
				c.Get(key)
			}
		}()
	}
	wg.Wait()
}
