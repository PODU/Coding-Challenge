// Package bruteforce lists every string over a character set with length
// between min and max, counting like an odometer in base len(charset).
package bruteforce

import (
	"context"
	"errors"
)

// Count returns how many candidates Generate will emit for the given charset
// size and length bounds: Σ_{k=min..max} n^k. It returns 0 for empty ranges.
func Count(n, min, max int) int {
	if n <= 0 || min > max || max < 0 {
		return 0
	}
	if min < 1 {
		min = 1
	}
	total := 0
	pow := 1
	for k := 1; k <= max; k++ {
		pow *= n
		if k >= min {
			total += pow
		}
	}
	return total
}

// Validate checks the brute-force parameters.
func Validate(charset string, min, max int) error {
	if charset == "" {
		return errors.New("charset must not be empty")
	}
	if min < 1 {
		return errors.New("min length must be >= 1")
	}
	if max < min {
		return errors.New("max length must be >= min length")
	}
	if hasDup(charset) {
		return errors.New("charset must not contain duplicate characters")
	}
	return nil
}

func hasDup(s string) bool {
	var seen [256]bool
	b := []byte(s)
	for _, c := range b {
		if seen[c] {
			return true
		}
		seen[c] = true
	}
	return false
}

// Generate emits all candidates over charset with length in [min, max] in
// odometer order (shortest first; within a length, charset order). The channel
// closes when the keyspace is exhausted or ctx is cancelled.
func Generate(ctx context.Context, charset string, min, max int) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		if Validate(charset, min, max) != nil {
			return
		}
		set := []byte(charset)
		n := len(set)
		for length := min; length <= max; length++ {
			idx := make([]int, length)
			buf := make([]byte, length)
			for i := range buf {
				buf[i] = set[0]
			}
			for {
				select {
				case out <- string(buf):
				case <-ctx.Done():
					return
				}
				pos := length - 1
				for pos >= 0 {
					idx[pos]++
					if idx[pos] < n {
						buf[pos] = set[idx[pos]]
						break
					}
					idx[pos] = 0
					buf[pos] = set[0]
					pos--
				}
				if pos < 0 {
					break // every combination of this length done
				}
			}
		}
	}()
	return out
}
