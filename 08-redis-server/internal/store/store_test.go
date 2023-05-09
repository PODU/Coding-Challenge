package store

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// newTestStore returns a store with a manually-advanced clock.
func newTestStore() (*Store, *int64) {
	s := New()
	var clock int64
	s.SetClock(func() int64 { return atomic.LoadInt64(&clock) })
	return s, &clock
}

func TestSetGet(t *testing.T) {
	s := New()
	if _, _, ok, err := s.Set("k", "v", SetOptions{}, false); err != nil || !ok {
		t.Fatalf("Set failed: ok=%v err=%v", ok, err)
	}
	got, ok, err := s.Get("k")
	if err != nil || !ok || got != "v" {
		t.Fatalf("Get = %q, ok=%v, err=%v; want v", got, ok, err)
	}

	// Miss.
	if _, ok, _ := s.Get("missing"); ok {
		t.Errorf("expected miss for absent key")
	}

	// Overwrite.
	s.Set("k", "v2", SetOptions{}, false)
	if got, _, _ := s.Get("k"); got != "v2" {
		t.Errorf("overwrite: got %q, want v2", got)
	}
}

func TestExistsDel(t *testing.T) {
	s := New()
	s.Set("a", "1", SetOptions{}, false)
	s.Set("b", "2", SetOptions{}, false)

	if n := s.Exists("a", "b", "c"); n != 2 {
		t.Errorf("Exists = %d, want 2", n)
	}
	if n := s.Exists("a", "a"); n != 2 {
		t.Errorf("Exists counts duplicates: got %d, want 2", n)
	}
	if n := s.Del("a", "c"); n != 1 {
		t.Errorf("Del = %d, want 1", n)
	}
	if s.Exists("a") != 0 {
		t.Errorf("a should be gone")
	}
}

func TestExpiryRelative(t *testing.T) {
	s, clock := newTestStore()
	atomic.StoreInt64(clock, 1000)
	// Expire at t=2000.
	s.Set("k", "v", SetOptions{ExpiresAt: 2000}, false)

	if _, ok, _ := s.Get("k"); !ok {
		t.Fatalf("key should be live before expiry")
	}
	atomic.StoreInt64(clock, 2000) // now == expiresAt -> gone
	if _, ok, _ := s.Get("k"); ok {
		t.Errorf("key should be expired at now==expiresAt")
	}
	if s.Exists("k") != 0 {
		t.Errorf("Exists should not count expired key")
	}
}

func TestExpiryAbsolutePast(t *testing.T) {
	s, clock := newTestStore()
	atomic.StoreInt64(clock, 5000)
	// EXAT/PXAT in the past -> immediately expired.
	s.Set("k", "v", SetOptions{ExpiresAt: 1000}, false)
	if _, ok, _ := s.Get("k"); ok {
		t.Errorf("past absolute expiry should be immediately gone")
	}
}

func TestKeepTTL(t *testing.T) {
	s, clock := newTestStore()
	atomic.StoreInt64(clock, 1000)
	s.Set("k", "v", SetOptions{ExpiresAt: 2000}, false)
	// Overwrite with KEEPTTL must retain expiry.
	s.Set("k", "v2", SetOptions{KeepTTL: true}, false)
	atomic.StoreInt64(clock, 2000)
	if _, ok, _ := s.Get("k"); ok {
		t.Errorf("KEEPTTL should have retained the original expiry")
	}
}

func TestNXXX(t *testing.T) {
	s := New()
	// NX on absent: succeeds.
	if _, _, ok, _ := s.Set("k", "v", SetOptions{NX: true}, false); !ok {
		t.Errorf("NX on absent key should set")
	}
	// NX on present: suppressed.
	if _, _, ok, _ := s.Set("k", "v2", SetOptions{NX: true}, false); ok {
		t.Errorf("NX on existing key should not set")
	}
	if got, _, _ := s.Get("k"); got != "v" {
		t.Errorf("value should be unchanged after failed NX, got %q", got)
	}
	// XX on present: succeeds.
	if _, _, ok, _ := s.Set("k", "v3", SetOptions{XX: true}, false); !ok {
		t.Errorf("XX on existing key should set")
	}
	// XX on absent: suppressed.
	if _, _, ok, _ := s.Set("new", "v", SetOptions{XX: true}, false); ok {
		t.Errorf("XX on absent key should not set")
	}
}

func TestSetGetOld(t *testing.T) {
	s := New()
	old, hadOld, _, _ := s.Set("k", "v1", SetOptions{}, true)
	if hadOld {
		t.Errorf("expected no old value on first set, got %q", old)
	}
	old, hadOld, _, _ = s.Set("k", "v2", SetOptions{}, true)
	if !hadOld || old != "v1" {
		t.Errorf("GET option: got old=%q hadOld=%v, want v1/true", old, hadOld)
	}
}

func TestIncrDecr(t *testing.T) {
	s := New()
	if n, _ := s.IncrBy("c", 1); n != 1 {
		t.Errorf("INCR from absent = %d, want 1", n)
	}
	if n, _ := s.IncrBy("c", 1); n != 2 {
		t.Errorf("INCR = %d, want 2", n)
	}
	if n, _ := s.IncrBy("c", -1); n != 1 {
		t.Errorf("DECR = %d, want 1", n)
	}

	// Non-integer value.
	s.Set("s", "hello", SetOptions{}, false)
	if _, err := s.IncrBy("s", 1); !errors.Is(err, ErrNotInteger) {
		t.Errorf("INCR non-integer: got %v, want ErrNotInteger", err)
	}

	// Overflow.
	s.Set("big", "9223372036854775807", SetOptions{}, false)
	if _, err := s.IncrBy("big", 1); !errors.Is(err, ErrOverflow) {
		t.Errorf("INCR overflow: got %v, want ErrOverflow", err)
	}
}

func TestPushAndRange(t *testing.T) {
	s := New()
	if n, _ := s.Push("l", []string{"a", "b", "c"}, false); n != 3 { // RPUSH
		t.Errorf("RPUSH len = %d, want 3", n)
	}
	if n, _ := s.Push("l", []string{"x", "y"}, true); n != 5 { // LPUSH -> y x a b c
		t.Errorf("LPUSH len = %d, want 5", n)
	}
	got, _ := s.LRange("l", 0, -1)
	want := []string{"y", "x", "a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("LRANGE = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LRANGE = %v, want %v", got, want)
		}
	}

	// Sub-range and WRONGTYPE.
	sub, _ := s.LRange("l", 1, 2)
	if len(sub) != 2 || sub[0] != "x" || sub[1] != "a" {
		t.Errorf("LRANGE 1 2 = %v, want [x a]", sub)
	}

	s.Set("str", "v", SetOptions{}, false)
	if _, err := s.Push("str", []string{"z"}, false); !errors.Is(err, ErrWrongType) {
		t.Errorf("PUSH on string: got %v, want ErrWrongType", err)
	}
}

func TestWrongTypeGet(t *testing.T) {
	s := New()
	s.Push("l", []string{"a"}, false)
	if _, _, err := s.Get("l"); !errors.Is(err, ErrWrongType) {
		t.Errorf("GET on list: got %v, want ErrWrongType", err)
	}
}

func TestPurgeExpired(t *testing.T) {
	s, clock := newTestStore()
	atomic.StoreInt64(clock, 1000)
	s.Set("a", "1", SetOptions{ExpiresAt: 1500}, false)
	s.Set("b", "2", SetOptions{}, false)
	atomic.StoreInt64(clock, 2000)
	if n := s.PurgeExpired(); n != 1 {
		t.Errorf("PurgeExpired = %d, want 1", n)
	}
	if s.Exists("b") != 1 {
		t.Errorf("non-expiring key b should survive")
	}
}

// TestConcurrentIncr runs N goroutines incrementing the same key M times and
// asserts no updates are lost. Run with -race to catch data races.
func TestConcurrentIncr(t *testing.T) {
	s := New()
	const (
		goroutines = 50
		perG       = 1000
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if _, err := s.IncrBy("counter", 1); err != nil {
					t.Errorf("IncrBy error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, _, _ := s.Get("counter")
	want := "50000"
	if got != want {
		t.Errorf("final counter = %q, want %q", got, want)
	}
}
