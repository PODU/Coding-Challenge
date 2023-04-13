// Package cut parses cut's LIST syntax and applies it to input lines.
package cut

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Range is an inclusive, 1-indexed span of positions. Hi == 0 means the range
// runs to the end of the line ("N-").
type Range struct {
	Lo, Hi int
}

func (r Range) open() bool { return r.Hi == 0 }

// Selector decides which 1-indexed positions are printed.
type Selector struct {
	ranges     []Range // sorted by Lo, non-overlapping
	complement bool
}

// ParseList parses a LIST such as "1,3-5,7-" or "1 2". Elements may be
// separated by commas, whitespace, or a mix of both.
func ParseList(spec string, complement bool) (Selector, error) {
	fields := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
	})
	if len(fields) == 0 {
		return Selector{}, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}

	ranges := make([]Range, 0, len(fields))
	for _, f := range fields {
		r, err := parseRange(f)
		if err != nil {
			return Selector{}, err
		}
		ranges = append(ranges, r)
	}

	return Selector{ranges: mergeRanges(ranges), complement: complement}, nil
}

func parseRange(s string) (Range, error) {
	dash := strings.IndexByte(s, '-')

	switch {
	case dash < 0: // N
		n, err := parsePos(s)
		if err != nil {
			return Range{}, err
		}
		return Range{Lo: n, Hi: n}, nil

	case dash == 0: // -M
		m, err := parsePos(s[1:])
		if err != nil {
			return Range{}, err
		}
		return Range{Lo: 1, Hi: m}, nil

	case dash == len(s)-1: // N-
		n, err := parsePos(s[:dash])
		if err != nil {
			return Range{}, err
		}
		return Range{Lo: n, Hi: 0}, nil

	default: // N-M
		lo, err := parsePos(s[:dash])
		if err != nil {
			return Range{}, err
		}
		hi, err := parsePos(s[dash+1:])
		if err != nil {
			return Range{}, err
		}
		if hi < lo {
			return Range{}, fmt.Errorf("invalid decreasing range")
		}
		return Range{Lo: lo, Hi: hi}, nil
	}
}

func parsePos(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid range with no endpoint: -")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid field value %q", s)
	}
	if n == 0 {
		return 0, fmt.Errorf("fields are numbered from 1")
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid field value %q", s)
	}
	return n, nil
}

// mergeRanges sorts and merges overlapping or adjacent ranges. GNU cut never
// repeats or reorders output, so "3,1-2" prints positions 1-3 once each.
func mergeRanges(in []Range) []Range {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Lo != in[j].Lo {
			return in[i].Lo < in[j].Lo
		}
		// Open ranges (Hi==0) sort after closed ones with equal Lo so the merge
		// below absorbs the closed ones into the open tail.
		return !in[i].open() && in[j].open()
	})

	merged := []Range{in[0]}
	for _, r := range in[1:] {
		last := &merged[len(merged)-1]
		if last.open() {
			// Everything after this is already covered.
			continue
		}
		if r.Lo <= last.Hi+1 {
			if r.open() {
				last.Hi = 0
			} else if r.Hi > last.Hi {
				last.Hi = r.Hi
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// Selected reports whether position pos (1-indexed) should be printed.
func (s Selector) Selected(pos int) bool {
	in := false
	for _, r := range s.ranges {
		if pos < r.Lo {
			break
		}
		if r.open() || pos <= r.Hi {
			in = true
			break
		}
	}
	if s.complement {
		return !in
	}
	return in
}

func (s Selector) Empty() bool { return len(s.ranges) == 0 }
