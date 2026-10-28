// Package protocol implements the Barrier wire protocol (version 1.6): the
// writef/readf field codec, the 4-byte length framing, and typed messages.
package protocol

import (
	"errors"
	"fmt"
)

// Limits copied from Barrier (PROTOCOL_MAX_STRING_LENGTH and
// PROTOCOL_MAX_LIST_LENGTH). A peer that claims more is treated as hostile.
const (
	MaxStringLength = 4 * 1024 * 1024
	MaxListLength   = 1024 * 1024
)

// ErrShort is returned when a payload ends before its format does.
var ErrShort = errors.New("protocol: message too short")

// Pack encodes args according to a Barrier format string:
//
//	%1i %2i %4i   big-endian unsigned integer of that many bytes
//	%s            4-byte length, then the bytes (string or []byte)
//	%1I %2I %4I   4-byte count, then that many integers of the given width
//	%%            a literal '%'
//
// Every other byte is copied literally, which is how the 4-character message
// codes ("DKDN", "CINN", ...) end up at the front of each payload.
func Pack(format string, args ...any) ([]byte, error) {
	out := make([]byte, 0, len(format)+8*len(args))
	ai := 0
	next := func() (any, error) {
		if ai >= len(args) {
			return nil, fmt.Errorf("protocol: format %q needs more arguments", format)
		}
		ai++
		return args[ai-1], nil
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			out = append(out, c)
			continue
		}
		i++
		if i >= len(format) {
			return nil, fmt.Errorf("protocol: dangling %% in %q", format)
		}
		if format[i] == '%' {
			out = append(out, '%')
			continue
		}
		width := 0
		if d := format[i]; d == '1' || d == '2' || d == '4' {
			width = int(d - '0')
			i++
			if i >= len(format) {
				return nil, fmt.Errorf("protocol: bad specifier in %q", format)
			}
		}
		a, err := next()
		if err != nil {
			return nil, err
		}
		switch format[i] {
		case 'i':
			v, err := toUint(a)
			if err != nil {
				return nil, err
			}
			out = appendUint(out, v, width)
		case 's':
			var b []byte
			switch s := a.(type) {
			case string:
				b = []byte(s)
			case []byte:
				b = s
			default:
				return nil, fmt.Errorf("protocol: %%s wants string, got %T", a)
			}
			out = appendUint(out, uint64(len(b)), 4)
			out = append(out, b...)
		case 'I':
			list, ok := a.([]uint32)
			if !ok {
				return nil, fmt.Errorf("protocol: %%I wants []uint32, got %T", a)
			}
			out = appendUint(out, uint64(len(list)), 4)
			for _, v := range list {
				out = appendUint(out, uint64(v), width)
			}
		default:
			return nil, fmt.Errorf("protocol: unknown specifier %%%c in %q", format[i], format)
		}
	}
	if ai != len(args) {
		return nil, fmt.Errorf("protocol: format %q got %d extra arguments", format, len(args)-ai)
	}
	return out, nil
}

// MustPack is Pack for formats and arguments fixed at compile time.
func MustPack(format string, args ...any) []byte {
	b, err := Pack(format, args...)
	if err != nil {
		panic(err)
	}
	return b
}

// Unpack decodes data according to format. Integers come back as uint32,
// %s as string and %I as []uint32. Literal bytes must match exactly. Trailing
// bytes after the format are ignored, as Barrier ignores them: newer protocol
// minors are allowed to append fields.
func Unpack(data []byte, format string) ([]any, error) {
	var out []any
	p := 0
	take := func(n int) ([]byte, error) {
		if n < 0 || len(data)-p < n {
			return nil, ErrShort
		}
		b := data[p : p+n]
		p += n
		return b, nil
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || (i+1 < len(format) && format[i+1] == '%') {
			if c == '%' {
				i++
			}
			b, err := take(1)
			if err != nil {
				return nil, err
			}
			if b[0] != c {
				return nil, fmt.Errorf("protocol: expected %q at offset %d, got %q", c, p-1, b[0])
			}
			continue
		}
		i++
		width := 0
		if i < len(format) {
			if d := format[i]; d == '1' || d == '2' || d == '4' {
				width = int(d - '0')
				i++
			}
		}
		if i >= len(format) {
			return nil, fmt.Errorf("protocol: bad specifier in %q", format)
		}
		switch format[i] {
		case 'i':
			b, err := take(width)
			if err != nil {
				return nil, err
			}
			out = append(out, uint32(readUint(b)))
		case 's':
			b, err := take(4)
			if err != nil {
				return nil, err
			}
			n := readUint(b)
			if n > MaxStringLength {
				return nil, fmt.Errorf("protocol: string length %d exceeds limit", n)
			}
			s, err := take(int(n))
			if err != nil {
				return nil, err
			}
			out = append(out, string(s))
		case 'I':
			b, err := take(4)
			if err != nil {
				return nil, err
			}
			n := readUint(b)
			if n > MaxListLength {
				return nil, fmt.Errorf("protocol: list length %d exceeds limit", n)
			}
			list := make([]uint32, 0, min(int(n), (len(data)-p)/max(width, 1)))
			for range n {
				e, err := take(width)
				if err != nil {
					return nil, err
				}
				list = append(list, uint32(readUint(e)))
			}
			out = append(out, list)
		default:
			return nil, fmt.Errorf("protocol: unknown specifier %%%c in %q", format[i], format)
		}
	}
	return out, nil
}

func appendUint(b []byte, v uint64, width int) []byte {
	for s := (width - 1) * 8; s >= 0; s -= 8 {
		b = append(b, byte(v>>uint(s)))
	}
	return b
}

func readUint(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

// toUint accepts any integer type. Negative values are encoded as two's
// complement, which is how Barrier puts signed coordinates in %2i fields.
func toUint(a any) (uint64, error) {
	switch v := a.(type) {
	case int:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("protocol: %%i wants an integer, got %T", a)
}
