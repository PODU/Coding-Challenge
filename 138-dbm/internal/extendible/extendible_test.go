package extendible

import (
	"fmt"
	"testing"
)

func key(i int) []byte { return []byte(fmt.Sprintf("key-%05d", i)) }
func val(i int) []byte { return []byte(fmt.Sprintf("value-%d", i)) }

func TestSplitsAndRetrieval(t *testing.T) {
	tb := New(4)
	for i := 0; i < 200; i++ {
		tb.Put(key(i), val(i))
	}
	if tb.Splits() < 3 {
		t.Fatalf("only %d splits", tb.Splits())
	}
	if tb.GlobalDepth() < 3 || tb.DirectorySize() != 1<<tb.GlobalDepth() {
		t.Fatalf("depth %d dir %d", tb.GlobalDepth(), tb.DirectorySize())
	}
	if tb.Doublings() == 0 {
		t.Fatal("directory never doubled")
	}
	for i := 0; i < 200; i++ {
		v, ok := tb.Get(key(i))
		if !ok || string(v) != string(val(i)) {
			t.Fatalf("key %d -> %q %v", i, v, ok)
		}
	}
	if _, ok := tb.Get([]byte("missing")); ok {
		t.Fatal("phantom key")
	}
	for _, b := range tb.dir {
		if b.Len() > 4 {
			t.Fatalf("bucket over capacity: %d", b.Len())
		}
		if int(b.LocalDepth) > tb.GlobalDepth() {
			t.Fatalf("local depth %d > global %d", b.LocalDepth, tb.GlobalDepth())
		}
	}
}

func TestDirectoryInvariant(t *testing.T) {
	tb := New(2)
	for i := 0; i < 500; i++ {
		tb.Put(key(i), val(i))
	}
	// A bucket with local depth L must be referenced by exactly 2^(G-L)
	// directory slots, and all of its keys must share the low L bits of
	// their hash with the slot index.
	refs := map[*Bucket]int{}
	for i, b := range tb.dir {
		refs[b]++
		for _, e := range b.entries {
			h := tb.hash(e.key)
			l := uint64(1)<<b.LocalDepth - 1
			if h&l != uint64(i)&l {
				t.Fatalf("slot %d holds key with mismatched low bits", i)
			}
		}
	}
	for b, n := range refs {
		if want := 1 << (uint(tb.GlobalDepth()) - uint(b.LocalDepth)); n != want {
			t.Fatalf("bucket depth %d referenced %d times, want %d", b.LocalDepth, n, want)
		}
	}
}

func TestDeleteAndReinsert(t *testing.T) {
	tb := New(3)
	for i := 0; i < 100; i++ {
		tb.Put(key(i), val(i))
	}
	for i := 0; i < 100; i += 3 {
		if !tb.Delete(key(i)) {
			t.Fatalf("delete %d", i)
		}
	}
	if tb.Len() != 66 {
		t.Fatalf("len = %d", tb.Len())
	}
	for i := 0; i < 100; i += 3 {
		if _, ok := tb.Get(key(i)); ok {
			t.Fatalf("key %d survived delete", i)
		}
		if tb.Put(key(i), []byte("again")) {
			t.Fatalf("reinsert %d reported replace", i)
		}
	}
	for i := 0; i < 100; i++ {
		v, ok := tb.Get(key(i))
		if !ok {
			t.Fatalf("lost key %d", i)
		}
		if i%3 == 0 && string(v) != "again" {
			t.Fatalf("reinserted key %d has %q", i, v)
		}
	}
	if tb.Delete([]byte("nope")) {
		t.Fatal("deleted a missing key")
	}
}

func TestReplaceDoesNotGrow(t *testing.T) {
	tb := New(2)
	for i := 0; i < 50; i++ {
		tb.Put([]byte("same"), val(i))
	}
	if tb.Len() != 1 || tb.Splits() != 0 {
		t.Fatalf("len %d splits %d", tb.Len(), tb.Splits())
	}
}

func TestPathologicalHashTerminates(t *testing.T) {
	tb := NewWithHash(2, func([]byte) uint64 { return 0xdeadbeef })
	for i := 0; i < 20; i++ {
		tb.Put(key(i), val(i))
	}
	for i := 0; i < 20; i++ {
		if _, ok := tb.Get(key(i)); !ok {
			t.Fatalf("lost key %d under constant hash", i)
		}
	}
	// the directory must not have grown chasing bits that never differ
	if tb.GlobalDepth() != 0 || tb.Splits() != 0 {
		t.Fatalf("depth %d splits %d: fruitless splits were not prevented", tb.GlobalDepth(), tb.Splits())
	}
	if tb.Len() != 20 {
		t.Fatalf("len = %d", tb.Len())
	}
}

func TestRangeVisitsEachOnce(t *testing.T) {
	tb := New(2)
	for i := 0; i < 64; i++ {
		tb.Put(key(i), val(i))
	}
	seen := map[string]int{}
	tb.Range(func(k, _ []byte) bool { seen[string(k)]++; return true })
	if len(seen) != 64 {
		t.Fatalf("Range saw %d keys", len(seen))
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("%s visited %d times", k, n)
		}
	}
	if tb.Buckets() < 16 {
		t.Fatalf("only %d buckets", tb.Buckets())
	}
}
