package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseSize parses a byte count with an optional k, M, G or T suffix (powers of
// 1024) and an optional trailing B.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	mult := int64(1)
	last := s[len(s)-1]
	if last == 'B' || last == 'b' {
		s = s[:len(s)-1]
		if s == "" {
			return 0, fmt.Errorf("invalid size")
		}
		last = s[len(s)-1]
	}
	switch last {
	case 'k', 'K':
		mult = 1 << 10
	case 'm', 'M':
		mult = 1 << 20
	case 'g', 'G':
		mult = 1 << 30
	case 't', 'T':
		mult = 1 << 40
	}
	if mult != 1 {
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("size must not be negative")
	}
	return n * mult, nil
}
