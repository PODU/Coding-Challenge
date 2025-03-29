package syscalls

import (
	"fmt"
	"strconv"
	"strings"

	"mystrace/internal/tracer"
)

// lookup returns the table entry for nr, or a placeholder with ok=false.
func lookup(nr int) (Syscall, bool) {
	if s, ok := table[nr]; ok {
		return s, true
	}
	return Syscall{Name: fmt.Sprintf("syscall_0x%x", nr)}, false
}

// Name returns the name of syscall nr and whether it is in the table.
func Name(nr int) (string, bool) {
	s, ok := lookup(nr)
	return s.Name, ok
}

// Decode formats a call's name, arguments and result. Strings longer than
// strSize are cut short.
func Decode(c tracer.RawCall, strSize int) Decoded {
	s, known := lookup(c.Nr)
	var args []string
	if known {
		args = make([]string, 0, len(s.Args))
		for i, at := range s.Args {
			args = append(args, decodeArg(at, c.Args[i], c.Mem, strSize))
		}
	} else {
		// We don't know how many arguments it takes, so show all six registers.
		args = make([]string, 0, len(c.Args))
		for _, v := range c.Args {
			args = append(args, fmt.Sprintf("%#x", v))
		}
	}
	return Decoded{Name: s.Name, Args: args, Ret: renderRet(s, c)}
}

func decodeArg(at ArgType, val uint64, mem tracer.MemReader, strSize int) string {
	switch at {
	case ArgInt, ArgLong:
		return strconv.FormatInt(int64(val), 10)
	case ArgFd:
		return strconv.FormatInt(int64(int32(uint32(val))), 10)
	case ArgPtr:
		if val == 0 {
			return "NULL"
		}
		return fmt.Sprintf("%#x", val)
	case ArgStr:
		if val == 0 {
			return "NULL"
		}
		if mem == nil {
			return fmt.Sprintf("%#x", val)
		}
		// Read one extra byte so we can detect truncation at strSize.
		s, ok := mem.ReadString(val, strSize+1)
		if !ok {
			return fmt.Sprintf("%#x", val)
		}
		return formatString(s, strSize)
	case ArgOpenFl:
		return renderOpenFlags(val)
	case ArgProt:
		return renderProt(val)
	case ArgMap:
		return renderMap(val)
	case ArgMode:
		return fmt.Sprintf("%#o", val)
	case ArgSignal:
		return renderSignal(val)
	case ArgAtFd:
		return renderAtFd(val)
	default:
		return fmt.Sprintf("%#x", val)
	}
}

// formatString quotes s like strace: C escapes for the usual characters, octal
// for other unprintable bytes, and "..." after the quote if it was cut at max.
func formatString(s string, max int) string {
	truncated := false
	if len(s) > max {
		s = s[:max]
		truncated = true
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if c >= 0x20 && c < 0x7f {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, `\%o`, c)
			}
		}
	}
	b.WriteByte('"')
	if truncated {
		b.WriteString("...")
	}
	return b.String()
}

// renderRet formats the result: "?" for calls that never return,
// "-1 ENOENT (message)" for errors, hex for pointers, decimal otherwise.
func renderRet(s Syscall, c tracer.RawCall) string {
	if !c.HasRet {
		return "?"
	}
	if e := ErrnoFromRet(true, c.Ret); e != 0 {
		name := ErrnoName(e)
		if name == "" {
			return fmt.Sprintf("-1 (errno %d)", e)
		}
		return fmt.Sprintf("-1 %s (%s)", name, ErrnoMessage(e))
	}
	if s.Ret == ArgPtr {
		return fmt.Sprintf("%#x", uint64(c.Ret))
	}
	return strconv.FormatInt(c.Ret, 10)
}
