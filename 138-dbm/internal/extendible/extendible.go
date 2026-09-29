// Package extendible is an in-memory extendible hash table. A directory of
// 2^globalDepth pointers indexes fixed-capacity buckets by the low bits of
// the key hash. A full bucket splits in two, and if it was already as deep
// as the directory, the directory doubles first. The same scheme, with pages in
// place of pointers, drives the on-disk store in package dbm.
package extendible

import (
	"bytes"

	"ccdbm/internal/hashfn"
)

// Splitting is only worthwhile when it actually separates keys. If the
// bucket's entries and the incoming key agree on the next splitWindow hash
// bits, up to splitWindow splits would each double the directory and move
// nothing; the insert instead lets the bucket exceed its capacity
// ("overflow") until a later key disagrees on one of those bits. maxDepth
// is a hard ceiling on directory size regardless.
const (
	splitWindow = 4
	maxDepth    = 32
)

type kv struct{ key, val []byte }

// Bucket holds up to the table's capacity of entries whose hashes share
// the low LocalDepth bits.
type Bucket struct {
	LocalDepth uint8
	entries    []kv
}

// Len returns the number of entries in the bucket.
func (b *Bucket) Len() int { return len(b.entries) }

// Table is an extendible hash table. It is not safe for concurrent use.
type Table struct {
	dir         []*Bucket
	globalDepth uint8
	capacity    int
	hash        hashfn.Func
	size        int
	splits      int
	doublings   int
}

// New returns a table whose buckets hold capacity entries, hashed with
// FNV-1a.
func New(capacity int) *Table { return NewWithHash(capacity, hashfn.FNV1a64) }

// NewWithHash returns a table using the given hash function.
func NewWithHash(capacity int, h hashfn.Func) *Table {
	if capacity < 1 {
		capacity = 1
	}
	return &Table{dir: []*Bucket{{}}, capacity: capacity, hash: h}
}

func (t *Table) mask() uint64 { return uint64(len(t.dir)) - 1 }

func (t *Table) bucketFor(h uint64) *Bucket { return t.dir[h&t.mask()] }

// Put stores value under key and reports whether it replaced an existing
// value.
func (t *Table) Put(key, value []byte) bool {
	h := t.hash(key)
	b := t.bucketFor(h)
	for i := range b.entries {
		if bytes.Equal(b.entries[i].key, key) {
			b.entries[i].val = clone(value)
			return true
		}
	}
	for len(b.entries) >= t.capacity && t.splittable(b, h) {
		t.split(b, h)
		b = t.bucketFor(h)
	}
	b.entries = append(b.entries, kv{clone(key), clone(value)})
	t.size++
	return false
}

// Get returns a copy of the value stored under key.
func (t *Table) Get(key []byte) ([]byte, bool) {
	b := t.bucketFor(t.hash(key))
	for i := range b.entries {
		if bytes.Equal(b.entries[i].key, key) {
			return clone(b.entries[i].val), true
		}
	}
	return nil, false
}

// Delete removes key and reports whether it was present. Buckets are not
// merged on delete; the freed slot is reused by the next insert.
func (t *Table) Delete(key []byte) bool {
	b := t.bucketFor(t.hash(key))
	for i := range b.entries {
		if bytes.Equal(b.entries[i].key, key) {
			b.entries = append(b.entries[:i], b.entries[i+1:]...)
			t.size--
			return true
		}
	}
	return false
}

// splittable reports whether splitting b would separate its entries from
// the key hashing to h within splitWindow more bits.
func (t *Table) splittable(b *Bucket, h uint64) bool {
	if b.LocalDepth >= maxDepth {
		return false
	}
	window := uint64(1)<<splitWindow - 1
	ref := (h >> b.LocalDepth) & window
	for _, e := range b.entries {
		if (t.hash(e.key)>>b.LocalDepth)&window != ref {
			return true
		}
	}
	return false
}

// split divides bucket b, which is reached by hash h, into two buckets of
// local depth b.LocalDepth+1, doubling the directory first if b is already
// as deep as the directory.
func (t *Table) split(b *Bucket, h uint64) {
	if b.LocalDepth == t.globalDepth {
		t.doubleDirectory()
	}
	bit := uint64(1) << b.LocalDepth
	b2 := &Bucket{LocalDepth: b.LocalDepth + 1}
	b.LocalDepth++
	kept := b.entries[:0]
	for _, e := range b.entries {
		if t.hash(e.key)&bit != 0 {
			b2.entries = append(b2.entries, e)
		} else {
			kept = append(kept, e)
		}
	}
	b.entries = kept
	// Every directory slot that pointed at b has index canon + k*bit
	// (canon = the low bits shared by all of b's keys before the split);
	// the odd k are the ones whose new bit is set and now belong to b2.
	canon := h & (bit - 1)
	for i := canon + bit; i < uint64(len(t.dir)); i += 2 * bit {
		t.dir[i] = b2
	}
	t.splits++
}

func (t *Table) doubleDirectory() {
	t.dir = append(t.dir, t.dir...)
	t.globalDepth++
	t.doublings++
}

// Len returns the number of stored keys.
func (t *Table) Len() int { return t.size }

// GlobalDepth is the number of hash bits the directory indexes on.
func (t *Table) GlobalDepth() int { return int(t.globalDepth) }

// DirectorySize is the number of directory slots (2^GlobalDepth).
func (t *Table) DirectorySize() int { return len(t.dir) }

// Buckets counts distinct buckets.
func (t *Table) Buckets() int {
	seen := map[*Bucket]bool{}
	for _, b := range t.dir {
		seen[b] = true
	}
	return len(seen)
}

// Splits counts bucket splits performed so far.
func (t *Table) Splits() int { return t.splits }

// Doublings counts directory doublings performed so far.
func (t *Table) Doublings() int { return t.doublings }

// Range calls fn for every entry, visiting each bucket once, until fn
// returns false.
func (t *Table) Range(fn func(key, value []byte) bool) {
	seen := map[*Bucket]bool{}
	for _, b := range t.dir {
		if seen[b] {
			continue
		}
		seen[b] = true
		for _, e := range b.entries {
			if !fn(e.key, e.val) {
				return
			}
		}
	}
}

func clone(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return append([]byte(nil), b...)
}
