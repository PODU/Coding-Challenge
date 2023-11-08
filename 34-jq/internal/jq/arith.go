package jq

import (
	"encoding/base64"
	"strings"
)

// applyBinary computes `l op r` with jq type semantics.
func applyBinary(op string, l, r Value) (Value, error) {
	switch op {
	case "+":
		return addValues(l, r)
	case "-":
		return subValues(l, r)
	case "*":
		return mulValues(l, r)
	case "/":
		return divValues(l, r)
	case "%":
		return modValues(l, r)
	case "==":
		return Equal(l, r), nil
	case "!=":
		return !Equal(l, r), nil
	case "<":
		return Compare(l, r) < 0, nil
	case "<=":
		return Compare(l, r) <= 0, nil
	case ">":
		return Compare(l, r) > 0, nil
	case ">=":
		return Compare(l, r) >= 0, nil
	case "and":
		if !Truthy(l) {
			return false, nil
		}
		return Truthy(r), nil
	case "or":
		if Truthy(l) {
			return true, nil
		}
		return Truthy(r), nil
	}
	return nil, evalErrorf("unknown operator %q", op)
}

func addValues(l, r Value) (Value, error) {
	if l == nil {
		return r, nil
	}
	if r == nil {
		return l, nil
	}
	switch lv := l.(type) {
	case float64:
		if rv, ok := r.(float64); ok {
			return lv + rv, nil
		}
	case string:
		if rv, ok := r.(string); ok {
			return lv + rv, nil
		}
	case []Value:
		if rv, ok := r.([]Value); ok {
			out := make([]Value, 0, len(lv)+len(rv))
			out = append(out, lv...)
			out = append(out, rv...)
			return out, nil
		}
	case *Object:
		if rv, ok := r.(*Object); ok {
			out := lv.Clone()
			for _, k := range rv.keys {
				v, _ := rv.Get(k)
				out.Set(k, v)
			}
			return out, nil
		}
	}
	return nil, typeBinErr(l, r, "added")
}

func subValues(l, r Value) (Value, error) {
	switch lv := l.(type) {
	case float64:
		if rv, ok := r.(float64); ok {
			return lv - rv, nil
		}
	case []Value:
		if rv, ok := r.([]Value); ok {
			out := []Value{}
			for _, el := range lv {
				if !containsValue(rv, el) {
					out = append(out, el)
				}
			}
			return out, nil
		}
	}
	return nil, typeBinErr(l, r, "subtracted")
}

func mulValues(l, r Value) (Value, error) {
	switch lv := l.(type) {
	case float64:
		if rv, ok := r.(float64); ok {
			return lv * rv, nil
		}
	case *Object:
		if rv, ok := r.(*Object); ok {
			return deepMerge(lv, rv), nil
		}
	}
	return nil, typeBinErr(l, r, "multiplied")
}

func divValues(l, r Value) (Value, error) {
	switch lv := l.(type) {
	case float64:
		if rv, ok := r.(float64); ok {
			if rv == 0 {
				return nil, evalErrorf("%s (%s) and %s (%s) cannot be divided because the divisor is zero", TypeName(l), shortValue(l), TypeName(r), shortValue(r))
			}
			return lv / rv, nil
		}
	case string:
		if rv, ok := r.(string); ok {
			parts := splitString(lv, rv)
			out := make([]Value, len(parts))
			for i, s := range parts {
				out[i] = s
			}
			return out, nil
		}
	}
	return nil, typeBinErr(l, r, "divided")
}

func modValues(l, r Value) (Value, error) {
	lv, lok := l.(float64)
	rv, rok := r.(float64)
	if lok && rok {
		ri := int64(rv)
		if ri == 0 {
			return nil, evalErrorf("%s and %s cannot be divided because the divisor is zero", shortValue(l), shortValue(r))
		}
		li := int64(lv)
		m := li % abs64(ri)
		return float64(m), nil
	}
	return nil, typeBinErr(l, r, "divided")
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func typeBinErr(l, r Value, verb string) *EvalError {
	return evalErrorf("%s (%s) and %s (%s) cannot be %s",
		TypeName(l), shortValue(l), TypeName(r), shortValue(r), verb)
}

func containsValue(arr []Value, v Value) bool {
	for _, el := range arr {
		if Equal(el, v) {
			return true
		}
	}
	return false
}

func deepMerge(a, b *Object) *Object {
	out := a.Clone()
	for _, k := range b.keys {
		bv, _ := b.Get(k)
		if av, ok := out.Get(k); ok {
			ao, aok := av.(*Object)
			bo, bok := bv.(*Object)
			if aok && bok {
				out.Set(k, deepMerge(ao, bo))
				continue
			}
		}
		out.Set(k, bv)
	}
	return out
}

func splitString(s, sep string) []string {
	if sep == "" {
		// jq splits into characters on empty separator
		rs := []rune(s)
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = string(r)
		}
		return out
	}
	return strings.Split(s, sep)
}

// applyFormat applies an @-format to a value interpolated into a string. An
// empty name means plain interpolation.
func applyFormat(format string, v Value) (string, error) {
	if format == "" {
		return interpToString(v), nil
	}
	out, err := applyFormatValue(format, v)
	if err != nil {
		return "", err
	}
	return out, nil
}

// interpToString mirrors jq: strings interpolate as-is, everything else as JSON.
func interpToString(v Value) string {
	if s, ok := v.(string); ok {
		return s
	}
	return Encode(v, EncodeOptions{Compact: true})
}

// applyFormatValue implements the @-format builtins, returning a string.
func applyFormatValue(name string, v Value) (string, error) {
	switch name {
	case "text":
		return interpToString(v), nil
	case "json":
		return Encode(v, EncodeOptions{Compact: true}), nil
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(interpToString(v))), nil
	case "base64d":
		s, ok := v.(string)
		if !ok {
			return "", evalErrorf("@base64d: input must be a string")
		}
		dec, err := base64.StdEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			// Some encoders drop the padding and some don't; accept both.
			dec, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				return "", evalErrorf("@base64d: invalid base64: %v", err)
			}
		}
		return string(dec), nil
	case "csv", "tsv":
		arr, ok := v.([]Value)
		if !ok {
			return "", evalErrorf("@%s: input must be an array", name)
		}
		sep := ","
		if name == "tsv" {
			sep = "\t"
		}
		return formatDelimited(arr, sep, name == "csv")
	default:
		return "", evalErrorf("@%s is not a supported format", name)
	}
}

func formatDelimited(arr []Value, sep string, csv bool) (string, error) {
	var b strings.Builder
	for i, el := range arr {
		if i > 0 {
			b.WriteString(sep)
		}
		switch t := el.(type) {
		case nil:
			// empty
		case float64:
			b.WriteString(FormatNumber(t))
		case bool:
			if t {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
		case string:
			if csv {
				b.WriteByte('"')
				b.WriteString(strings.ReplaceAll(t, `"`, `""`))
				b.WriteByte('"')
			} else {
				s := t
				s = strings.ReplaceAll(s, `\`, `\\`)
				s = strings.ReplaceAll(s, "\t", `\t`)
				s = strings.ReplaceAll(s, "\n", `\n`)
				s = strings.ReplaceAll(s, "\r", `\r`)
				b.WriteString(s)
			}
		default:
			return "", evalErrorf("@%s: arrays must contain scalars", map[bool]string{true: "csv", false: "tsv"}[csv])
		}
	}
	return b.String(), nil
}
