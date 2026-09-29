package hashtable

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPutGetDelete(t *testing.T) {
	tb := New()
	if tb.Put([]byte("greeting"), []byte("Hello, world!")) {
		t.Fatal("first put reported replace")
	}
	if v, ok := tb.Get([]byte("greeting")); !ok || string(v) != "Hello, world!" {
		t.Fatalf("get = %q, %v", v, ok)
	}
	if !tb.Put([]byte("greeting"), []byte("Hi")) {
		t.Fatal("second put should replace")
	}
	if v, _ := tb.Get([]byte("greeting")); string(v) != "Hi" {
		t.Fatalf("after replace: %q", v)
	}
	if tb.Len() != 1 {
		t.Fatalf("len = %d", tb.Len())
	}
	if !tb.Delete([]byte("greeting")) || tb.Delete([]byte("greeting")) {
		t.Fatal("delete semantics wrong")
	}
	if _, ok := tb.Get([]byte("greeting")); ok {
		t.Fatal("deleted key still present")
	}
	if tb.Len() != 0 {
		t.Fatalf("len after delete = %d", tb.Len())
	}
}

func TestBinaryKeysAndValues(t *testing.T) {
	tb := New()
	k := []byte{0, 1, 2, 0xff, 0}
	v := []byte{0, 0, 0}
	tb.Put(k, v)
	tb.Put([]byte{}, []byte("empty key"))
	if got, ok := tb.Get([]byte{0, 1, 2, 0xff, 0}); !ok || !bytes.Equal(got, v) {
		t.Fatalf("binary get = %v %v", got, ok)
	}
	if got, ok := tb.Get(nil); !ok || string(got) != "empty key" {
		t.Fatalf("empty-key get = %q %v", got, ok)
	}
	// stored copies are independent of the caller's buffers
	k[0] = 9
	if _, ok := tb.Get([]byte{0, 1, 2, 0xff, 0}); !ok {
		t.Fatal("table aliased the caller's key buffer")
	}
}

func TestGrowthKeepsEverything(t *testing.T) {
	tb := New()
	const n = 10000
	for i := 0; i < n; i++ {
		tb.Put([]byte(fmt.Sprintf("key%d", i)), []byte(fmt.Sprintf("val%d", i)))
	}
	if tb.Len() != n {
		t.Fatalf("len = %d", tb.Len())
	}
	if tb.Buckets() <= initialBuckets {
		t.Fatalf("table never grew: %d buckets", tb.Buckets())
	}
	for i := 0; i < n; i++ {
		v, ok := tb.Get([]byte(fmt.Sprintf("key%d", i)))
		if !ok || string(v) != fmt.Sprintf("val%d", i) {
			t.Fatalf("key%d -> %q %v", i, v, ok)
		}
	}
	for i := 0; i < n; i += 2 {
		if !tb.Delete([]byte(fmt.Sprintf("key%d", i))) {
			t.Fatalf("delete key%d failed", i)
		}
	}
	if len(tb.Keys()) != n/2 || tb.Len() != n/2 {
		t.Fatalf("after deletes: %d keys", tb.Len())
	}
}

func TestCollisionsChain(t *testing.T) {
	tb := NewWithHash(func([]byte) uint64 { return 42 }) // everything collides
	for i := 0; i < 100; i++ {
		tb.Put([]byte{byte(i)}, []byte{byte(i * 2)})
	}
	for i := 0; i < 100; i++ {
		if v, ok := tb.Get([]byte{byte(i)}); !ok || v[0] != byte(i*2) {
			t.Fatalf("collision chain lost key %d", i)
		}
	}
	tb.Delete([]byte{50})
	if _, ok := tb.Get([]byte{50}); ok {
		t.Fatal("delete in chain failed")
	}
	if _, ok := tb.Get([]byte{51}); !ok {
		t.Fatal("delete in chain broke a neighbour")
	}
}

func TestRangeStopsEarly(t *testing.T) {
	tb := New()
	for i := 0; i < 10; i++ {
		tb.Put([]byte{byte(i)}, nil)
	}
	n := 0
	tb.Range(func(_, _ []byte) bool { n++; return n < 3 })
	if n != 3 {
		t.Fatalf("Range visited %d, want 3", n)
	}
}
