package xxd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reverse reads a hex dump from r and writes the original bytes to w.
//
// Normal dumps are read line by line: the offset before ':' says where the
// bytes go (gaps are zero-filled) and the ASCII column is ignored. With
// cfg.Plain every hex digit in the input is decoded as one stream.
func Reverse(r io.Reader, w io.Writer, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	if err := reverse(r, bw, cfg); err != nil {
		return err
	}
	return bw.Flush()
}

func reverse(r io.Reader, w *bufio.Writer, cfg Config) error {
	sc := bufio.NewScanner(r)
	// Large -c values make for long lines.
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	if cfg.Plain {
		return reversePlain(sc, w)
	}

	var pos int64
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		offset, hexPart, ok := splitDumpLine(line)
		if !ok {
			continue
		}

		data, err := decodeHex(hexPart)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			continue
		}

		if offset > pos {
			if err := writeZeros(w, offset-pos); err != nil {
				return err
			}
			pos = offset
		}
		// An offset that goes backwards can't be honoured on a plain writer,
		// so the bytes are just appended (xxd does the same into a pipe).
		if _, err := w.Write(data); err != nil {
			return err
		}
		pos += int64(len(data))
	}
	return sc.Err()
}

func reversePlain(sc *bufio.Scanner, w *bufio.Writer) error {
	var nibbles strings.Builder
	for sc.Scan() {
		for _, c := range sc.Text() {
			if isHexDigit(byte(c)) {
				nibbles.WriteByte(byte(c))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	data, err := decodeHex(nibbles.String())
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// splitDumpLine returns the offset and hex column of a dump line. The hex
// column ends at the first double space: groups are separated by single
// spaces, so that is where the ASCII column starts.
func splitDumpLine(line string) (offset int64, hexPart string, ok bool) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return 0, "", false
	}
	off, err := strconv.ParseInt(strings.TrimSpace(line[:colon]), 16, 64)
	if err != nil {
		return 0, "", false
	}
	rest := line[colon+1:]
	if i := strings.Index(rest, "  "); i >= 0 {
		rest = rest[:i]
	}
	return off, rest, true
}

// decodeHex decodes the hex digits in s, skipping blanks.
func decodeHex(s string) ([]byte, error) {
	out := make([]byte, 0, len(s)/2)
	hi := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		v, ok := hexVal(c)
		if !ok {
			return nil, fmt.Errorf("invalid hex character %q", c)
		}
		if hi < 0 {
			hi = v
		} else {
			out = append(out, byte(hi<<4|v))
			hi = -1
		}
	}
	if hi >= 0 {
		return nil, fmt.Errorf("odd number of hex digits in %q", s)
	}
	return out, nil
}

func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	default:
		return 0, false
	}
}

func isHexDigit(c byte) bool {
	_, ok := hexVal(c)
	return ok
}

func writeZeros(w *bufio.Writer, n int64) error {
	var zeros [4096]byte
	for n > 0 {
		chunk := int64(len(zeros))
		if chunk > n {
			chunk = n
		}
		if _, err := w.Write(zeros[:chunk]); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}
