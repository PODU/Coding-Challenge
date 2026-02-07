package validator

import (
	"encoding/json"
	"math/big"
)

// toRat returns v as an exact rational, or false if v isn't a number.
// float64 and int are accepted too, for schemas built by hand in tests.
func toRat(v interface{}) (*big.Rat, bool) {
	switch n := v.(type) {
	case json.Number:
		r := new(big.Rat)
		if _, ok := r.SetString(n.String()); ok {
			return r, true
		}
		return nil, false
	case float64:
		r := new(big.Rat)
		if r.SetFloat64(n) == nil {
			return nil, false
		}
		return r, true
	case int:
		return new(big.Rat).SetInt64(int64(n)), true
	case int64:
		return new(big.Rat).SetInt64(n), true
	}
	return nil, false
}

func isNumber(v interface{}) bool {
	_, ok := toRat(v)
	return ok
}

// isInteger reports whether v is a number with no fractional part, so 1.0
// counts but 1.5 doesn't.
func isInteger(v interface{}) bool {
	r, ok := toRat(v)
	return ok && r.IsInt()
}
