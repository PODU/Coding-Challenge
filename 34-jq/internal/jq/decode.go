package jq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Decoder reads a stream of JSON values from an io.Reader, preserving object
// key insertion order.
type Decoder struct {
	dec *json.Decoder
}

// NewDecoder returns a decoder for r, skipping a leading UTF-8 BOM.
func NewDecoder(r io.Reader) *Decoder {
	d := json.NewDecoder(stripBOM(r))
	d.UseNumber()
	return &Decoder{dec: d}
}

// Decode reads the next top-level value, or returns io.EOF at the end.
func (d *Decoder) Decode() (Value, error) {
	if !d.dec.More() {
		// Read a token to get io.EOF or the syntax error.
		_, err := d.dec.Token()
		if err == io.EOF || err == nil {
			return nil, io.EOF
		}
		return nil, err
	}
	return d.decodeValue()
}

func (d *Decoder) decodeValue() (Value, error) {
	tok, err := d.dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := NewObject()
			for d.dec.More() {
				keyTok, err := d.dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string")
				}
				val, err := d.decodeValue()
				if err != nil {
					return nil, err
				}
				obj.Set(key, val) // last value wins on duplicate keys
			}
			if _, err := d.dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []Value{}
			for d.dec.More() {
				val, err := d.decodeValue()
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := d.dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", t)
		}
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", t.String(), err)
		}
		return f, nil
	case string:
		return t, nil
	case bool:
		return t, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected token %v", tok)
	}
}

// ParseValue decodes exactly one JSON value from src (used by fromjson).
func ParseValue(src []byte) (Value, error) {
	d := NewDecoder(bytes.NewReader(src))
	v, err := d.Decode()
	if err != nil {
		return nil, err
	}
	// Reject trailing garbage.
	if _, err := d.Decode(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing data after JSON value")
		}
		return nil, err
	}
	return v, nil
}

// stripBOM wraps r so that a single leading UTF-8 BOM is dropped.
func stripBOM(r io.Reader) io.Reader {
	br := &bomReader{r: r}
	return br
}

type bomReader struct {
	r       io.Reader
	checked bool
	buf     []byte
}

func (b *bomReader) Read(p []byte) (int, error) {
	if !b.checked {
		b.checked = true
		var head [3]byte
		n, err := io.ReadFull(b.r, head[:])
		if n == 3 && head[0] == 0xEF && head[1] == 0xBB && head[2] == 0xBF {
		} else {
			b.buf = append(b.buf, head[:n]...)
		}
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return 0, err
		}
	}
	if len(b.buf) > 0 {
		n := copy(p, b.buf)
		b.buf = b.buf[n:]
		return n, nil
	}
	return b.r.Read(p)
}
