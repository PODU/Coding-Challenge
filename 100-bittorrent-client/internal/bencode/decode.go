// Package bencode decodes and encodes bencode (BEP 3), the format used by
// .torrent files and tracker responses.
//
// Decoded values use these Go types:
//
//	bencode string     -> string (raw bytes, may be non-UTF-8)
//	bencode integer    -> int64
//	bencode list       -> []interface{}
//	bencode dictionary -> map[string]interface{}
package bencode

import (
	"errors"
	"fmt"
)

// maxDepth stops deeply nested input from blowing the stack.
const maxDepth = 1000

var (
	ErrEmpty       = errors.New("bencode: empty input")
	ErrTrailing    = errors.New("bencode: trailing data after top-level value")
	ErrEOF         = errors.New("bencode: unexpected end of input")
	ErrLeadingZero = errors.New("bencode: integer has leading zero")
	ErrNegZero     = errors.New("bencode: integer is negative zero")
	ErrMaxDepth    = errors.New("bencode: maximum nesting depth exceeded")
)

// Decode parses one bencode value. Trailing bytes after it are an error.
func Decode(data []byte) (interface{}, error) {
	d := &decoder{data: data}
	v, err := d.parse(0)
	if err != nil {
		return nil, err
	}
	if d.pos != len(d.data) {
		return nil, ErrTrailing
	}
	return v, nil
}

// DecodeWithRaw is like Decode, but when the value is a dictionary it also
// returns the raw bytes of each top-level value. The info hash has to be taken
// over the original "info" bytes; re-encoding them is not guaranteed to give
// the same result.
func DecodeWithRaw(data []byte) (interface{}, map[string][]byte, error) {
	d := &decoder{data: data, capture: true}
	v, err := d.parse(0)
	if err != nil {
		return nil, nil, err
	}
	if d.pos != len(d.data) {
		return nil, nil, ErrTrailing
	}
	return v, d.topRaw, nil
}

type decoder struct {
	data    []byte
	pos     int
	capture bool
	topRaw  map[string][]byte
}

func (d *decoder) parse(depth int) (interface{}, error) {
	if depth > maxDepth {
		return nil, ErrMaxDepth
	}
	if d.pos >= len(d.data) {
		if d.pos == 0 {
			return nil, ErrEmpty
		}
		return nil, ErrEOF
	}
	switch c := d.data[d.pos]; {
	case c == 'i':
		return d.parseInt()
	case c == 'l':
		return d.parseList(depth)
	case c == 'd':
		return d.parseDict(depth)
	case c >= '0' && c <= '9':
		return d.parseString()
	default:
		return nil, fmt.Errorf("bencode: invalid type byte %q at offset %d", c, d.pos)
	}
}

func (d *decoder) parseInt() (int64, error) {
	start := d.pos + 1
	end := start
	for end < len(d.data) && d.data[end] != 'e' {
		end++
	}
	if end >= len(d.data) {
		return 0, ErrEOF
	}
	tok := d.data[start:end]
	if len(tok) == 0 {
		return 0, errors.New("bencode: empty integer")
	}
	body := tok
	neg := body[0] == '-'
	if neg {
		body = body[1:]
		if len(body) == 0 {
			return 0, errors.New("bencode: lone minus sign in integer")
		}
	}
	if len(body) > 1 && body[0] == '0' {
		return 0, ErrLeadingZero
	}
	var n int64
	for _, ch := range body {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("bencode: invalid integer digit %q", ch)
		}
		n = n*10 + int64(ch-'0')
	}
	if neg {
		if n == 0 {
			return 0, ErrNegZero
		}
		n = -n
	}
	d.pos = end + 1
	return n, nil
}

func (d *decoder) parseString() (string, error) {
	colon := d.pos
	for colon < len(d.data) && d.data[colon] != ':' {
		c := d.data[colon]
		if c < '0' || c > '9' {
			return "", fmt.Errorf("bencode: invalid string length digit %q", c)
		}
		colon++
	}
	if colon >= len(d.data) {
		return "", ErrEOF
	}
	lenTok := d.data[d.pos:colon]
	if len(lenTok) == 0 {
		return "", errors.New("bencode: missing string length")
	}
	if len(lenTok) > 1 && lenTok[0] == '0' {
		return "", errors.New("bencode: string length has leading zero")
	}
	var n int
	for _, ch := range lenTok {
		n = n*10 + int(ch-'0')
		if n < 0 || n > len(d.data) {
			return "", errors.New("bencode: string length out of range")
		}
	}
	start := colon + 1
	if start+n > len(d.data) {
		return "", ErrEOF
	}
	s := string(d.data[start : start+n])
	d.pos = start + n
	return s, nil
}

func (d *decoder) parseList(depth int) ([]interface{}, error) {
	d.pos++ // consume 'l'
	list := []interface{}{}
	for {
		if d.pos >= len(d.data) {
			return nil, ErrEOF
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			return list, nil
		}
		v, err := d.parse(depth + 1)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
}

func (d *decoder) parseDict(depth int) (map[string]interface{}, error) {
	top := depth == 0 && d.capture
	d.pos++ // consume 'd'
	m := map[string]interface{}{}
	for {
		if d.pos >= len(d.data) {
			return nil, ErrEOF
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			return m, nil
		}
		if d.data[d.pos] < '0' || d.data[d.pos] > '9' {
			return nil, errors.New("bencode: dictionary key is not a string")
		}
		key, err := d.parseString()
		if err != nil {
			return nil, err
		}
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("bencode: duplicate dictionary key %q", key)
		}

		valStart := d.pos
		v, err := d.parse(depth + 1)
		if err != nil {
			return nil, err
		}
		if top {
			if d.topRaw == nil {
				d.topRaw = map[string][]byte{}
			}
			d.topRaw[key] = d.data[valStart:d.pos]
		}
		m[key] = v
	}
}
