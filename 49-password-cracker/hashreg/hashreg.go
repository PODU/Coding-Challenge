// Package hashreg is a small registry of hash algorithms behind a common Algo
// interface, plus Detect, which guesses one from a digest's hex length.
package hashreg

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Algo is a hash algorithm. Implementations must be safe for concurrent use.
type Algo interface {
	Name() string
	Sum(data []byte) []byte
	HexLen() int
}

func HexSum(a Algo, data []byte) string {
	return hex.EncodeToString(a.Sum(data))
}

type algo struct {
	name   string
	hexLen int
	sum    func(data []byte) []byte
}

func (a algo) Name() string        { return a.name }
func (a algo) HexLen() int         { return a.hexLen }
func (a algo) Sum(d []byte) []byte { return a.sum(d) }

var registry = map[string]Algo{
	"md5":    algo{"md5", 32, func(d []byte) []byte { s := md5.Sum(d); return s[:] }},
	"sha1":   algo{"sha1", 40, func(d []byte) []byte { s := sha1.Sum(d); return s[:] }},
	"sha256": algo{"sha256", 64, func(d []byte) []byte { s := sha256.Sum256(d); return s[:] }},
	"sha512": algo{"sha512", 128, func(d []byte) []byte { s := sha512.Sum512(d); return s[:] }},
}

// Get returns the algorithm registered under name (case-insensitive).
func Get(name string) (Algo, error) {
	a, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, fmt.Errorf("unknown algorithm %q (known: %s)", name, strings.Join(Names(), ", "))
	}
	return a, nil
}

// Names returns the sorted list of registered algorithm names.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Detect picks the algorithm whose digest is hexLen characters long, erroring
// if the length is shared by more than one.
func Detect(hexLen int) (Algo, error) {
	var matches []Algo
	for _, a := range registry {
		if a.HexLen() == hexLen {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no known algorithm produces a %d-char hex digest", hexLen)
	case 1:
		return matches[0], nil
	default:
		sort.Slice(matches, func(i, j int) bool { return matches[i].Name() < matches[j].Name() })
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name()
		}
		return nil, fmt.Errorf("ambiguous digest length %d (candidates: %s); specify -a explicitly", hexLen, strings.Join(names, ", "))
	}
}
