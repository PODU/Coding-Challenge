// Package hashfn provides the hash functions a ccdbm database can be built
// on. The function is chosen when a database is created and its identifier
// is recorded in the file header, so every later open uses the same one.
package hashfn

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

// Func maps a key to a 64-bit hash. Databases index buckets by the low bits
// of the result, so a function must spread entropy into the low bits.
type Func func(key []byte) uint64

// ID identifies a hash function in a database header.
type ID uint32

// Registered hash functions.
const (
	FNV1a   ID = 1 // 64-bit FNV-1a (default)
	DJB2    ID = 2 // Bernstein's djb2 with a 64-bit accumulator
	Murmur3 ID = 3 // MurmurHash3 x86_32, seed 0
)

var registry = map[ID]struct {
	name string
	fn   Func
}{
	FNV1a:   {"fnv1a", FNV1a64},
	DJB2:    {"djb2", DJB2Hash},
	Murmur3: {"murmur3", func(b []byte) uint64 { return uint64(Murmur3x86_32(b, 0)) }},
}

// String returns the registered name of the function.
func (id ID) String() string {
	if r, ok := registry[id]; ok {
		return r.name
	}
	return fmt.Sprintf("unknown(%d)", uint32(id))
}

// Valid reports whether id names a registered function.
func (id ID) Valid() bool {
	_, ok := registry[id]
	return ok
}

// Func returns the hash function, or nil for an unknown ID.
func (id ID) Func() Func {
	if r, ok := registry[id]; ok {
		return r.fn
	}
	return nil
}

// Parse maps a name such as "fnv1a" to its ID (case-insensitive).
func Parse(name string) (ID, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	for id, r := range registry {
		if r.name == n {
			return id, nil
		}
	}
	return 0, fmt.Errorf("unknown hash function %q (choose one of %s)", name, strings.Join(Names(), ", "))
}

// Names lists the registered function names in sorted order.
func Names() []string {
	out := make([]string, 0, len(registry))
	for _, r := range registry {
		out = append(out, r.name)
	}
	sort.Strings(out)
	return out
}

// FNV1a64 is the 64-bit Fowler–Noll–Vo 1a hash.
func FNV1a64(b []byte) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime
	}
	return h
}

// DJB2Hash is Daniel Bernstein's h = h*33 + c hash, run in 64 bits.
func DJB2Hash(b []byte) uint64 {
	h := uint64(5381)
	for _, c := range b {
		h = h*33 + uint64(c)
	}
	return h
}

// Murmur3x86_32 is Austin Appleby's MurmurHash3 for the x86_32 variant.
func Murmur3x86_32(b []byte, seed uint32) uint32 {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
	)
	h := seed
	n := len(b)
	for len(b) >= 4 {
		k := binary.LittleEndian.Uint32(b)
		b = b[4:]
		k *= c1
		k = bits.RotateLeft32(k, 15)
		k *= c2
		h ^= k
		h = bits.RotateLeft32(h, 13)
		h = h*5 + 0xe6546b64
	}
	var k uint32
	switch len(b) {
	case 3:
		k ^= uint32(b[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(b[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(b[0])
		k *= c1
		k = bits.RotateLeft32(k, 15)
		k *= c2
		h ^= k
	}
	h ^= uint32(n)
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}
