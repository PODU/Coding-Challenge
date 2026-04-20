package interp

import (
	"fmt"
	"strconv"
	"strings"
)

// sprintfOne formats a single float with a Go format verb (used for CONVFMT).
func sprintfOne(format string, n float64) string {
	return fmt.Sprintf(format, n)
}

// sprintf implements awk's printf/sprintf formatting.
func (p *Interp) sprintf(format string, args []value) string {
	var b strings.Builder
	argi := 0
	nextArg := func() value {
		if argi < len(args) {
			v := args[argi]
			argi++
			return v
		}
		argi++
		return uninit
	}
	i := 0
	for i < len(format) {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			i++
			continue
		}
		// parse a conversion specification
		j := i + 1
		if j < len(format) && format[j] == '%' {
			b.WriteByte('%')
			i = j + 1
			continue
		}
		spec := []byte{'%'}
		// flags
		for j < len(format) && strings.IndexByte("-+ 0#", format[j]) >= 0 {
			spec = append(spec, format[j])
			j++
		}
		// width
		if j < len(format) && format[j] == '*' {
			w := int(nextArg().num())
			spec = append(spec, []byte(strconv.Itoa(w))...)
			j++
		} else {
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				spec = append(spec, format[j])
				j++
			}
		}
		// precision
		if j < len(format) && format[j] == '.' {
			spec = append(spec, '.')
			j++
			if j < len(format) && format[j] == '*' {
				pr := int(nextArg().num())
				spec = append(spec, []byte(strconv.Itoa(pr))...)
				j++
			} else {
				for j < len(format) && format[j] >= '0' && format[j] <= '9' {
					spec = append(spec, format[j])
					j++
				}
			}
		}
		if j >= len(format) {
			// trailing '%...' with no verb: emit literally
			b.WriteString(format[i:])
			i = len(format)
			break
		}
		verb := format[j]
		j++
		switch verb {
		case 'd', 'i':
			spec = append(spec, 'd')
			fmt.Fprintf(&b, string(spec), int64(nextArg().num()))
		case 'o', 'x', 'X':
			spec = append(spec, verb)
			fmt.Fprintf(&b, string(spec), uint64(int64(nextArg().num())))
		case 'u':
			spec = append(spec, 'd')
			fmt.Fprintf(&b, string(spec), uint64(int64(nextArg().num())))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			spec = append(spec, verb)
			fmt.Fprintf(&b, string(spec), nextArg().num())
		case 'c':
			v := nextArg()
			var ch string
			if v.typ == typeStr {
				if len(v.s) > 0 {
					ch = v.s[:1]
				}
			} else {
				code := int(v.num())
				if code == 0 {
					ch = ""
				} else {
					ch = string(rune(code))
				}
			}
			spec = append(spec, 's')
			fmt.Fprintf(&b, string(spec), ch)
		case 's':
			spec = append(spec, 's')
			fmt.Fprintf(&b, string(spec), nextArg().str(p.convfmt))
		default:
			// unknown verb: emit literally
			b.WriteString(format[i:j])
		}
		i = j
	}
	return b.String()
}
