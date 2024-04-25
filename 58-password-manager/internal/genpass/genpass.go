// Package genpass generates random passwords using crypto/rand.
package genpass

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
)

const (
	lowerSet  = "abcdefghijklmnopqrstuvwxyz"
	upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet  = "0123456789"
	symbolSet = "!@#$%^&*()-_=+[]{};:,.<>?"
)

type Options struct {
	Length  int
	Lower   bool
	Upper   bool
	Digits  bool
	Symbols bool
}

// DefaultOptions is 20 characters from every class.
func DefaultOptions() Options {
	return Options{Length: 20, Lower: true, Upper: true, Digits: true, Symbols: true}
}

func (o Options) classes() []string {
	var c []string
	if o.Lower {
		c = append(c, lowerSet)
	}
	if o.Upper {
		c = append(c, upperSet)
	}
	if o.Digits {
		c = append(c, digitSet)
	}
	if o.Symbols {
		c = append(c, symbolSet)
	}
	return c
}

// Generate returns a password with at least one character from each enabled
// class.
func Generate(o Options) (string, error) {
	classes := o.classes()
	if len(classes) == 0 {
		return "", errors.New("at least one character class must be enabled")
	}
	if o.Length <= 0 {
		return "", errors.New("length must be greater than zero")
	}
	if o.Length < len(classes) {
		return "", errors.New("length is too short to include every enabled character class")
	}

	var alphabet []byte
	for _, c := range classes {
		alphabet = append(alphabet, c...)
	}

	// One character from each class first, then fill up and shuffle so the
	// guaranteed ones aren't always at the front.
	out := make([]byte, 0, o.Length)
	for _, c := range classes {
		b, err := pick(c)
		if err != nil {
			return "", err
		}
		out = append(out, b)
	}
	for len(out) < o.Length {
		b, err := pick(string(alphabet))
		if err != nil {
			return "", err
		}
		out = append(out, b)
	}
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

// pick returns a uniformly random byte of set. rand.Int avoids modulo bias.
func pick(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, err
	}
	return set[n.Int64()], nil
}

// shuffle is a Fisher-Yates shuffle driven by crypto/rand.
func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		jn, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := jn.Int64()
		b[i], b[j] = b[j], b[i]
	}
	return nil
}

// Entropy returns length * log2(alphabet size), in bits.
func Entropy(o Options) float64 {
	size := 0
	for _, c := range o.classes() {
		size += len(c)
	}
	if size == 0 || o.Length <= 0 {
		return 0
	}
	return float64(o.Length) * math.Log2(float64(size))
}
