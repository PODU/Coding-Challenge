package store

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// clock is a controllable time source for deterministic expiry tests.
type clock struct{ t atomic.Int64 }

func (c *clock) now() int64  { return c.t.Load() }
func (c *clock) set(v int64) { c.t.Store(v) }

func newTestStore(maxBytes int64) (*Store, *clock) {
	c := &clock{}
	c.set(1000)
	return NewWithClock(maxBytes, c.now), c
}

func mustGet(t *testing.T, s *Store, key string) ItemView {
	t.Helper()
	v := s.Get([]string{key})
	if len(v) != 1 {
		t.Fatalf("expected hit for %q, got %d", key, len(v))
	}
	return v[0]
}

func TestSetGetRoundTrip(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Set("foo", 7, 0, []byte("bar")); r != ResultStored {
		t.Fatalf("set: %v", r)
	}
	v := mustGet(t, s, "foo")
	if string(v.Value) != "bar" || v.Flags != 7 {
		t.Fatalf("bad value: %+v", v)
	}
}

func TestEmptyValue(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Set("e", 0, 0, []byte{}); r != ResultStored {
		t.Fatalf("set empty: %v", r)
	}
	v := mustGet(t, s, "e")
	if len(v.Value) != 0 {
		t.Fatalf("expected empty value")
	}
}

func TestBinarySafe(t *testing.T) {
	s, _ := newTestStore(0)
	val := []byte{0, '\r', '\n', 0xff, 'x'}
	s.Set("b", 0, 0, val)
	v := mustGet(t, s, "b")
	if string(v.Value) != string(val) {
		t.Fatalf("binary value mangled")
	}
}

func TestAdd(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Add("k", 0, 0, []byte("v1")); r != ResultStored {
		t.Fatalf("add new: %v", r)
	}
	if r := s.Add("k", 0, 0, []byte("v2")); r != ResultNotStored {
		t.Fatalf("add existing: %v", r)
	}
	if string(mustGet(t, s, "k").Value) != "v1" {
		t.Fatalf("add overwrote existing")
	}
}

func TestReplace(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Replace("k", 0, 0, []byte("v")); r != ResultNotStored {
		t.Fatalf("replace missing: %v", r)
	}
	s.Set("k", 0, 0, []byte("v1"))
	if r := s.Replace("k", 0, 0, []byte("v2")); r != ResultStored {
		t.Fatalf("replace existing: %v", r)
	}
	if string(mustGet(t, s, "k").Value) != "v2" {
		t.Fatalf("replace did not update")
	}
}

func TestAppendPrepend(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Append("k", []byte("x")); r != ResultNotStored {
		t.Fatalf("append missing: %v", r)
	}
	s.Set("k", 5, 0, []byte("bar"))
	if r := s.Append("k", []byte("baz")); r != ResultStored {
		t.Fatalf("append: %v", r)
	}
	if string(mustGet(t, s, "k").Value) != "barbaz" {
		t.Fatalf("append wrong: %s", mustGet(t, s, "k").Value)
	}
	if r := s.Prepend("k", []byte("foo")); r != ResultStored {
		t.Fatalf("prepend: %v", r)
	}
	v := mustGet(t, s, "k")
	if string(v.Value) != "foobarbaz" {
		t.Fatalf("prepend wrong: %s", v.Value)
	}
	if v.Flags != 5 {
		t.Fatalf("append/prepend should preserve flags, got %d", v.Flags)
	}
}

func TestDelete(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Delete("k"); r != ResultNotFound {
		t.Fatalf("delete missing: %v", r)
	}
	s.Set("k", 0, 0, []byte("v"))
	if r := s.Delete("k"); r != ResultStored {
		t.Fatalf("delete hit: %v", r)
	}
	if len(s.Get([]string{"k"})) != 0 {
		t.Fatalf("key still present after delete")
	}
}

func TestExpiryRelative(t *testing.T) {
	s, c := newTestStore(0)
	s.Set("k", 0, 10, []byte("v")) // expires at now+10 = 1010
	c.set(1009)
	if len(s.Get([]string{"k"})) != 1 {
		t.Fatalf("should still be live at 1009")
	}
	c.set(1010)
	if len(s.Get([]string{"k"})) != 0 {
		t.Fatalf("should be expired at 1010")
	}
}

func TestExpiryAbsolute(t *testing.T) {
	s, c := newTestStore(0)
	abs := int64(2592000 + 5000) // above 30d threshold => absolute ts
	s.Set("k", 0, abs, []byte("v"))
	c.set(abs - 1)
	if len(s.Get([]string{"k"})) != 1 {
		t.Fatalf("should be live before absolute ts")
	}
	c.set(abs)
	if len(s.Get([]string{"k"})) != 0 {
		t.Fatalf("should be expired at absolute ts")
	}
}

func TestExpiryNegativeImmediate(t *testing.T) {
	s, _ := newTestStore(0)
	s.Set("k", 0, -1, []byte("v"))
	if len(s.Get([]string{"k"})) != 0 {
		t.Fatalf("negative exptime should be immediately expired")
	}
}

func TestExpiryZeroNever(t *testing.T) {
	s, c := newTestStore(0)
	s.Set("k", 0, 0, []byte("v"))
	c.set(1 << 40)
	if len(s.Get([]string{"k"})) != 1 {
		t.Fatalf("exptime 0 should never expire")
	}
}

func TestCasSuccessExistsNotFound(t *testing.T) {
	s, _ := newTestStore(0)
	if r := s.Cas("k", 0, 0, []byte("v"), 1); r != ResultNotFound {
		t.Fatalf("cas missing: %v", r)
	}
	s.Set("k", 0, 0, []byte("v1"))
	tok := mustGet(t, s, "k").CAS
	if r := s.Cas("k", 0, 0, []byte("v2"), tok+999); r != ResultExists {
		t.Fatalf("cas stale token should be EXISTS: %v", r)
	}
	if r := s.Cas("k", 0, 0, []byte("v2"), tok); r != ResultStored {
		t.Fatalf("cas valid token: %v", r)
	}
	// Old token now stale.
	if r := s.Cas("k", 0, 0, []byte("v3"), tok); r != ResultExists {
		t.Fatalf("cas with old token should be EXISTS: %v", r)
	}
}

func TestCasMonotonic(t *testing.T) {
	s, _ := newTestStore(0)
	s.Set("a", 0, 0, []byte("1"))
	c1 := mustGet(t, s, "a").CAS
	s.Set("b", 0, 0, []byte("1"))
	c2 := mustGet(t, s, "b").CAS
	if c2 <= c1 {
		t.Fatalf("cas not monotonic: %d then %d", c1, c2)
	}
}

func TestReapExpired(t *testing.T) {
	s, c := newTestStore(0)
	s.Set("k", 0, 10, []byte("v"))
	s.Set("p", 0, 0, []byte("v"))
	c.set(2000)
	if n := s.ReapExpired(); n != 1 {
		t.Fatalf("expected 1 reaped, got %d", n)
	}
	if s.Len() != 1 {
		t.Fatalf("expected 1 item left, got %d", s.Len())
	}
}

func TestTooLargeItem(t *testing.T) {
	s, _ := newTestStore(0)
	big := make([]byte, DefaultMaxItemBytes+1)
	if r := s.Set("k", 0, 0, big); r != ResultTooLarge {
		t.Fatalf("expected too large, got %v", r)
	}
}

func TestLRUEviction(t *testing.T) {
	// Budget tight enough to hold ~3 items of this size.
	perItem := itemSize("keyN", []byte("0123456789"))
	s, _ := newTestStore(perItem * 3)

	s.Set("a", 0, 0, []byte("0123456789"))
	s.Set("b", 0, 0, []byte("0123456789"))
	s.Set("c", 0, 0, []byte("0123456789"))

	// Touch "a" so it becomes most-recently-used; "b" is now LRU.
	mustGet(t, s, "a")

	// Insert "d": should evict "b" (least recently used).
	s.Set("d", 0, 0, []byte("0123456789"))

	if len(s.Get([]string{"b"})) != 0 {
		t.Fatalf("expected b to be evicted")
	}
	for _, k := range []string{"a", "c", "d"} {
		if len(s.Get([]string{k})) != 1 {
			t.Fatalf("expected %s to survive", k)
		}
	}
}

func TestEvictUntilFits(t *testing.T) {
	perItem := itemSize("k", []byte("xxxxx"))
	s, _ := newTestStore(perItem * 2)
	for i := 0; i < 10; i++ {
		s.Set(fmt.Sprintf("k%02d", i), 0, 0, []byte("xxxxx"))
	}
	if s.Bytes() > perItem*2+8 { // small slack for key length variance
		t.Fatalf("memory budget exceeded: %d", s.Bytes())
	}
	if s.Len() > 2 {
		t.Fatalf("expected at most 2 items, got %d", s.Len())
	}
}

func TestMemoryAccountingOnOverwriteAndDelete(t *testing.T) {
	s, _ := newTestStore(0)
	s.Set("k", 0, 0, []byte("aaaa"))
	b1 := s.Bytes()
	s.Set("k", 0, 0, []byte("aaaaaaaa")) // larger overwrite
	if s.Bytes() <= b1 {
		t.Fatalf("expected accounting to grow on larger overwrite")
	}
	s.Delete("k")
	if s.Bytes() != 0 {
		t.Fatalf("expected 0 bytes after delete, got %d", s.Bytes())
	}
}

func TestConcurrency(t *testing.T) {
	s, _ := newTestStore(0)
	const workers = 50
	const iters = 500
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", id)
			for i := 0; i < iters; i++ {
				s.Set(key, 0, 0, []byte("v"))
				s.Get([]string{key})
				s.Append(key, []byte("x"))
				s.Delete(key)
			}
		}(w)
	}
	wg.Wait()
}
