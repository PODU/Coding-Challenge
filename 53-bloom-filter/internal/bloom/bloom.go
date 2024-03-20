// Package bloom is a Bloom filter for the spell checker.
//
// The k hash values come from a single 64-bit FNV-1a hash split in two and
// combined Kirsch-Mitzenmacher style, so each lookup hashes the word once
// whatever k is.
package bloom

import (
	"hash/fnv"
	"math"
	"strings"
)

// BloomFilter is not usable as a zero value; create one with New,
// NewWithParams or ReadFrom.
type BloomFilter struct {
	m    uint32 // bits
	k    uint16 // hash functions
	bits []byte
}

// Optimal returns the bit count m and hash count k for n items at
// false-positive rate p:
//
//	m = ceil( -(n * ln p) / (ln 2)^2 )
//	k = round( (m / n) * ln 2 )
//
// n and k are at least 1, so an empty dictionary still gives a working filter.
// p must be in (0, 1); New checks that.
func Optimal(n int, p float64) (m uint32, k uint16) {
	if n < 1 {
		n = 1
	}
	nf := float64(n)
	ln2 := math.Ln2
	mf := math.Ceil(-(nf * math.Log(p)) / (ln2 * ln2))
	if mf < 1 {
		mf = 1
	}
	if mf > math.MaxUint32 {
		mf = math.MaxUint32
	}
	m = uint32(mf)
	kf := math.Round((float64(m) / nf) * ln2)
	if kf < 1 {
		kf = 1
	}
	if kf > math.MaxUint16 {
		kf = math.MaxUint16
	}
	k = uint16(kf)
	return m, k
}

// New returns a filter sized for n items at false-positive rate p.
func New(n int, p float64) (*BloomFilter, error) {
	if p <= 0 || p >= 1 {
		return nil, &ParamError{P: p}
	}
	m, k := Optimal(n, p)
	return NewWithParams(m, k), nil
}

// NewWithParams returns an empty filter with m bits and k hash functions
// (each at least 1).
func NewWithParams(m uint32, k uint16) *BloomFilter {
	if m < 1 {
		m = 1
	}
	if k < 1 {
		k = 1
	}
	return &BloomFilter{
		m:    m,
		k:    k,
		bits: make([]byte, (int(m)+7)/8),
	}
}

// M returns the number of bits.
func (bf *BloomFilter) M() uint32 { return bf.m }

// K returns the number of hash functions.
func (bf *BloomFilter) K() uint16 { return bf.k }

// Bits returns the underlying bit array. Don't modify it.
func (bf *BloomFilter) Bits() []byte { return bf.bits }

// Normalize trims and lowercases a word. Add and Contains both use it, so
// "Hello" and "hello" are the same word.
func Normalize(word string) string {
	return strings.ToLower(strings.TrimSpace(word))
}

func (bf *BloomFilter) indexes(word string) []uint32 {
	h := fnv.New64a()
	h.Write([]byte(Normalize(word)))
	sum := h.Sum64()

	h1 := uint32(sum >> 32)
	// An odd step can't be 0, so the k indices are never all the same bit.
	h2 := uint32(sum) | 1

	idx := make([]uint32, bf.k)
	gi := h1
	for i := 0; i < int(bf.k); i++ {
		idx[i] = gi % bf.m
		gi += h2
	}
	return idx
}

func (bf *BloomFilter) set(i uint32) {
	bf.bits[i>>3] |= 1 << (i & 7)
}

func (bf *BloomFilter) get(i uint32) bool {
	return bf.bits[i>>3]&(1<<(i&7)) != 0
}

// Add inserts word.
func (bf *BloomFilter) Add(word string) {
	for _, i := range bf.indexes(word) {
		bf.set(i)
	}
}

// Contains reports whether word may have been added. False means it
// definitely wasn't; true can be a false positive.
func (bf *BloomFilter) Contains(word string) bool {
	for _, i := range bf.indexes(word) {
		if !bf.get(i) {
			return false
		}
	}
	return true
}
