package ustar

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// errBase256 is returned for GNU base-256 numeric fields, which we don't support.
var errBase256 = errors.New("ustar: base-256 numeric encoding is not supported")

// encodeOctal formats v as width-1 zero-padded octal digits followed by a NUL.
func encodeOctal(v int64, width int) ([]byte, error) {
	if v < 0 {
		return nil, fmt.Errorf("ustar: cannot encode negative value %d as octal", v)
	}
	digits := width - 1
	s := strconv.FormatInt(v, 8)
	if len(s) > digits {
		return nil, fmt.Errorf("ustar: value %d does not fit in %d octal digits", v, digits)
	}
	out := make([]byte, width) // last byte stays NUL
	pad := digits - len(s)
	for i := 0; i < pad; i++ {
		out[i] = '0'
	}
	copy(out[pad:digits], s)
	return out, nil
}

// parseOctal decodes an octal field padded with NULs or spaces. A blank field
// is 0. GNU base-256 fields (high bit set on the first byte) are rejected.
func parseOctal(field []byte) (int64, error) {
	if len(field) > 0 && field[0]&0x80 != 0 {
		return 0, errBase256
	}
	s := strings.Trim(string(field), "\x00 ")
	if s == "" {
		return 0, nil
	}
	// Some encoders embed stray NULs; cut at the first one within.
	if i := strings.IndexByte(s, '\x00'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 8, 64)
	if err != nil {
		return 0, fmt.Errorf("ustar: invalid octal field %q: %w", s, err)
	}
	return n, nil
}
