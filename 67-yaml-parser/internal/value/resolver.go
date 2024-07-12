package value

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// floatRe matches plain floats. It requires a '.' or exponent so bare integers
// go to the integer path and bare "inf"/"nan" stay strings (only ".inf"/".nan"
// are special, handled below).
var floatRe = regexp.MustCompile(`^[-+]?(\.[0-9]+([eE][-+]?[0-9]+)?|[0-9]+\.[0-9]*([eE][-+]?[0-9]+)?|[0-9]+[eE][-+]?[0-9]+)$`)

// ResolvePlain applies YAML implicit typing to an unquoted scalar, trying in
// order null, bool, int, float, and finally string. Quoted and block scalars
// must not be passed here; they are always strings.
func ResolvePlain(s string) Value {
	switch s {
	case "", "~", "null", "Null", "NULL":
		return NullValue()
	}

	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return BoolValue(true)
	case "false", "no", "off":
		return BoolValue(false)
	}

	// Base 0 auto-detects 0x/0o/0b and a leading-zero octal, and rejects bad
	// forms like "08", which then fall through to a string.
	if looksLikeInt(s) {
		if n, err := strconv.ParseInt(s, 0, 64); err == nil {
			return IntValue(n)
		}
	}

	if v, ok := specialFloat(s); ok {
		return v
	}
	if floatRe.MatchString(s) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return FloatValue(f)
		}
	}
	return StrValue(s)
}

// looksLikeInt screens out strings ParseInt would accept but YAML wouldn't,
// like underscores or a bare sign.
func looksLikeInt(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '+' || s[0] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	rest := s[i:]
	if strings.HasPrefix(rest, "0x") || strings.HasPrefix(rest, "0X") ||
		strings.HasPrefix(rest, "0o") || strings.HasPrefix(rest, "0O") ||
		strings.HasPrefix(rest, "0b") || strings.HasPrefix(rest, "0B") {
		return true
	}
	for j := i; j < len(s); j++ {
		if s[j] < '0' || s[j] > '9' {
			return false
		}
	}
	return true
}

func specialFloat(s string) (Value, bool) {
	t := strings.ToLower(s)
	sign := 1.0
	switch {
	case strings.HasPrefix(t, "+"):
		t = t[1:]
	case strings.HasPrefix(t, "-"):
		t = t[1:]
		sign = -1.0
	}
	switch t {
	case ".inf":
		return FloatValue(math.Inf(int(sign))), true
	case ".nan":
		return FloatValue(math.NaN()), true
	}
	return Value{}, false
}
