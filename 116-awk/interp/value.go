package interp

import (
	"math"
	"strconv"
	"strings"
)

type valtyp int

const (
	typeUninit valtyp = iota // uninitialised: "" / 0
	typeNum
	typeStr
	typeStrnum // a string from input that looks numeric
)

// value is an awk scalar.
type value struct {
	typ valtyp
	n   float64
	s   string
}

func num(n float64) value { return value{typ: typeNum, n: n} }
func str(s string) value  { return value{typ: typeStr, s: s} }
func boolVal(b bool) value {
	if b {
		return num(1)
	}
	return num(0)
}

var uninit = value{typ: typeUninit}

// strnum makes a value from input text: it becomes a numeric string if it
// looks like a number, otherwise a plain string.
func strnum(s string) value {
	if looksNumeric(s) {
		return value{typ: typeStrnum, s: s, n: parseNum(s)}
	}
	return value{typ: typeStr, s: s}
}

// numericComparable reports whether the value participates in numeric
// comparison.
func (v value) numericComparable() bool {
	switch v.typ {
	case typeNum, typeStrnum, typeUninit:
		return true
	}
	return false
}

func (v value) num() float64 {
	switch v.typ {
	case typeNum, typeStrnum:
		return v.n
	case typeUninit:
		return 0
	default:
		return parseNum(v.s)
	}
}

// str renders the value as a string using the given format for non-integral
// numbers (CONVFMT or OFMT).
func (v value) str(format string) string {
	switch v.typ {
	case typeStr, typeStrnum:
		return v.s
	case typeUninit:
		return ""
	default:
		return formatNum(v.n, format)
	}
}

func (v value) boolean() bool {
	switch v.typ {
	case typeNum, typeStrnum:
		return v.n != 0
	case typeUninit:
		return false
	default:
		return v.s != ""
	}
}

func formatNum(n float64, format string) string {
	if math.IsNaN(n) {
		return "nan"
	}
	if math.IsInf(n, 1) {
		return "inf"
	}
	if math.IsInf(n, -1) {
		return "-inf"
	}
	if n == math.Trunc(n) {
		if n == 0 {
			return "0"
		}
		if math.Abs(n) < 1e15 {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return sprintfOne(format, n)
}

// compare returns -1, 0, or 1 comparing a and b per awk rules.
func compareValues(a, b value, convfmt string) int {
	if a.numericComparable() && b.numericComparable() {
		x, y := a.num(), b.num()
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(a.str(convfmt), b.str(convfmt))
}

// looksNumeric reports whether s (ignoring surrounding blanks) is a valid awk
// numeric string.
func looksNumeric(s string) bool {
	i := 0
	n := len(s)
	for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	j := n
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n') {
		j--
	}
	if i >= j {
		return false
	}
	body := s[i:j]
	k := 0
	if body[k] == '+' || body[k] == '-' {
		k++
	}
	digits := false
	for k < len(body) && body[k] >= '0' && body[k] <= '9' {
		k++
		digits = true
	}
	if k < len(body) && body[k] == '.' {
		k++
		for k < len(body) && body[k] >= '0' && body[k] <= '9' {
			k++
			digits = true
		}
	}
	if !digits {
		return false
	}
	if k < len(body) && (body[k] == 'e' || body[k] == 'E') {
		k++
		if k < len(body) && (body[k] == '+' || body[k] == '-') {
			k++
		}
		expDigits := false
		for k < len(body) && body[k] >= '0' && body[k] <= '9' {
			k++
			expDigits = true
		}
		if !expDigits {
			return false
		}
	}
	return k == len(body)
}

// parseNum parses the leading numeric prefix of s (awk's strtod semantics).
func parseNum(s string) float64 {
	i := 0
	n := len(s)
	for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	start := i
	if i < n && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := false
	for i < n && s[i] >= '0' && s[i] <= '9' {
		i++
		digits = true
	}
	if i < n && s[i] == '.' {
		i++
		for i < n && s[i] >= '0' && s[i] <= '9' {
			i++
			digits = true
		}
	}
	if !digits {
		return 0
	}
	if i < n && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < n && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < n && s[j] >= '0' && s[j] <= '9' {
			for j < n && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	f, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0
	}
	return f
}
