// Package hashtable is an in-memory hash table keyed by byte strings, using
// separate chaining and growing when the load factor gets too high.
package hashtable

import (
	"bytes"

	"ccdbm/internal/hashfn"
)

const (
	initialBuckets = 16
	maxLoad        = 0.75
)

type entry struct {
	key, val []byte
	next     *entry
}

// Table is a chained hash table. It is not safe for concurrent use.
type Table struct {
	buckets []*entry
	size    int
	hash    hashfn.Func
}

// New returns an empty table hashed with FNV-1a.
func New() *Table { return NewWithHash(hashfn.FNV1a64) }

// NewWithHash returns an empty table using the given hash function.
func NewWithHash(h hashfn.Func) *Table {
	return &Table{buckets: make([]*entry, initialBuckets), hash: h}
}

func (t *Table) index(key []byte, n int) int {
	return int(t.hash(key) & uint64(n-1)) // n is always a power of two
}

// Put stores value under key, copying both. It reports whether an existing
// value was replaced.
func (t *Table) Put(key, value []byte) bool {
	i := t.index(key, len(t.buckets))
	for e := t.buckets[i]; e != nil; e = e.next {
		if bytes.Equal(e.key, key) {
			e.val = clone(value)
			return true
		}
	}
	t.buckets[i] = &entry{key: clone(key), val: clone(value), next: t.buckets[i]}
	t.size++
	if float64(t.size) > maxLoad*float64(len(t.buckets)) {
		t.grow()
	}
	return false
}

// Get returns a copy of the value stored under key.
func (t *Table) Get(key []byte) ([]byte, bool) {
	for e := t.buckets[t.index(key, len(t.buckets))]; e != nil; e = e.next {
		if bytes.Equal(e.key, key) {
			return clone(e.val), true
		}
	}
	return nil, false
}

// Delete removes key and reports whether it was present.
func (t *Table) Delete(key []byte) bool {
	i := t.index(key, len(t.buckets))
	var prev *entry
	for e := t.buckets[i]; e != nil; prev, e = e, e.next {
		if bytes.Equal(e.key, key) {
			if prev == nil {
				t.buckets[i] = e.next
			} else {
				prev.next = e.next
			}
			t.size--
			return true
		}
	}
	return false
}

// Len returns the number of stored keys.
func (t *Table) Len() int { return t.size }

// Buckets returns the current number of slots (for tests and stats).
func (t *Table) Buckets() int { return len(t.buckets) }

// Range calls fn for every key/value pair until fn returns false. The
// slices passed to fn are the table's own and must not be modified.
func (t *Table) Range(fn func(key, value []byte) bool) {
	for _, head := range t.buckets {
		for e := head; e != nil; e = e.next {
			if !fn(e.key, e.val) {
				return
			}
		}
	}
}

// Keys returns copies of all keys in bucket order.
func (t *Table) Keys() [][]byte {
	out := make([][]byte, 0, t.size)
	t.Range(func(k, _ []byte) bool {
		out = append(out, clone(k))
		return true
	})
	return out
}

func (t *Table) grow() {
	nb := make([]*entry, len(t.buckets)*2)
	for _, head := range t.buckets {
		for e := head; e != nil; {
			next := e.next
			i := t.index(e.key, len(nb))
			e.next = nb[i]
			nb[i] = e
			e = next
		}
	}
	t.buckets = nb
}

func clone(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return append([]byte(nil), b...)
}
