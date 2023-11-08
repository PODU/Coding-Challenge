package jq

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// EncodeOptions controls how values are serialised.
type EncodeOptions struct {
	Compact  bool   // -c : single line, no insignificant whitespace
	Raw      bool   // -r : top-level strings printed without quotes
	SortKeys bool   // -S : sort object keys
	ASCII    bool   // -a : escape non-ASCII as \uXXXX
	Indent   string // pretty indent unit (default two spaces)
}

// Encode serialises v according to opts and returns the string (no trailing
// newline). The Raw option only affects a top-level string value.
func Encode(v Value, opts EncodeOptions) string {
	if opts.Raw {
		if s, ok := v.(string); ok {
			return s
		}
	}
	var b strings.Builder
	indent := opts.Indent
	if indent == "" {
		indent = "  "
	}
	enc := &encoder{opts: opts, indent: indent}
	enc.encode(&b, v, 0)
	return b.String()
}

type encoder struct {
	opts   EncodeOptions
	indent string
}

func (e *encoder) newline(b *strings.Builder, depth int) {
	if e.opts.Compact {
		return
	}
	b.WriteByte('\n')
	for i := 0; i < depth; i++ {
		b.WriteString(e.indent)
	}
}

func (e *encoder) encode(b *strings.Builder, v Value, depth int) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64:
		b.WriteString(FormatNumber(t))
	case string:
		e.encodeString(b, t)
	case []Value:
		e.encodeArray(b, t, depth)
	case *Object:
		e.encodeObject(b, t, depth)
	default:
		b.WriteString("null")
	}
}

func (e *encoder) encodeArray(b *strings.Builder, arr []Value, depth int) {
	if len(arr) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteByte('[')
	for i, el := range arr {
		if i > 0 {
			b.WriteByte(',')
		}
		e.newline(b, depth+1)
		e.encode(b, el, depth+1)
	}
	e.newline(b, depth)
	b.WriteByte(']')
}

func (e *encoder) encodeObject(b *strings.Builder, o *Object, depth int) {
	if o.Len() == 0 {
		b.WriteString("{}")
		return
	}
	keys := o.Keys()
	if e.opts.SortKeys {
		keys = sortedKeys(o)
	}
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		e.newline(b, depth+1)
		e.encodeString(b, k)
		b.WriteByte(':')
		if !e.opts.Compact {
			b.WriteByte(' ')
		}
		val, _ := o.Get(k)
		e.encode(b, val, depth+1)
	}
	e.newline(b, depth)
	b.WriteByte('}')
}

func (e *encoder) encodeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				writeUnicodeEscape(b, r)
			case r == utf8.RuneError:
				b.WriteString(`�`)
			case e.opts.ASCII && r > 0x7f:
				if r > 0xffff {
					r1, r2 := utf16Pair(r)
					writeUnicodeEscape(b, r1)
					writeUnicodeEscape(b, r2)
				} else {
					writeUnicodeEscape(b, r)
				}
			default:
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func writeUnicodeEscape(b *strings.Builder, r rune) {
	const hex = "0123456789abcdef"
	b.WriteString(`\u`)
	b.WriteByte(hex[(r>>12)&0xf])
	b.WriteByte(hex[(r>>8)&0xf])
	b.WriteByte(hex[(r>>4)&0xf])
	b.WriteByte(hex[r&0xf])
}

func utf16Pair(r rune) (rune, rune) {
	r -= 0x10000
	return 0xd800 + (r>>10)&0x3ff, 0xdc00 + r&0x3ff
}

// FormatNumber renders a float64 the way jq does: integers without a decimal
// point, everything else as the shortest round-trippable representation.
func FormatNumber(f float64) string {
	if math.IsNaN(f) {
		return "null"
	}
	if math.IsInf(f, 1) {
		return "1.7976931348623157e+308"
	}
	if math.IsInf(f, -1) {
		return "-1.7976931348623157e+308"
	}
	if f == 0 {
		return "0"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e17 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
