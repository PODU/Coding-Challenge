// Package format prints replies the way redis-cli does, e.g. "(integer) 3",
// "(nil)", quoted strings, and numbered lists for arrays.
package format

import (
	"fmt"
	"strconv"
	"strings"

	"rediscli/resp"
)

// Render formats v without a trailing newline.
func Render(v resp.Value) string {
	return render(v, "")
}

// render formats v, indenting every line but the first by indent so nested
// arrays line up under their parent's numbering.
func render(v resp.Value, indent string) string {
	switch v.Type {
	case resp.SimpleString:
		return v.Str
	case resp.Error:
		return "(error) " + v.Str
	case resp.Integer:
		return "(integer) " + strconv.FormatInt(v.Int, 10)
	case resp.BulkString:
		if v.IsNull {
			return "(nil)"
		}
		return quote(v.Str)
	case resp.Array:
		if v.IsNull {
			return "(nil)"
		}
		if len(v.Array) == 0 {
			return "(empty array)"
		}
		return renderArray(v.Array, indent)
	case resp.Null:
		return "(nil)"
	default:
		return ""
	}
}

func renderArray(elems []resp.Value, indent string) string {
	width := len(strconv.Itoa(len(elems)))
	var b strings.Builder
	for i, el := range elems {
		idx := fmt.Sprintf("%*d) ", width, i+1)
		childIndent := indent + strings.Repeat(" ", len(idx))
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString(indent)
		}
		b.WriteString(idx)
		b.WriteString(render(el, childIndent))
	}
	return b.String()
}

// quote quotes s the way redis-cli's sdscatrepr does, escaping control
// characters so binary values print readably.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\a':
			b.WriteString("\\a")
		case '\b':
			b.WriteString("\\b")
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
