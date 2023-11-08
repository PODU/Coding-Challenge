package jq

import (
	"fmt"
	"strings"
)

// EvalError is a recoverable jq runtime error. try/catch and the `?` operator
// catch values of this type; other Go errors propagate as fatal.
type EvalError struct {
	val Value
}

func (e *EvalError) Error() string {
	if s, ok := e.val.(string); ok {
		return s
	}
	if e.val == nil {
		return "null (null)"
	}
	return TypeName(e.val) + " (" + Encode(e.val, EncodeOptions{Compact: true}) + ") not a string"
}

// Val is the value a catch handler receives.
func (e *EvalError) Val() Value { return e.val }

func evalErrorf(format string, args ...interface{}) *EvalError {
	return &EvalError{val: fmt.Sprintf(format, args...)}
}

// emitError wraps an error returned by a downstream consumer so that try/`?`
// can tell it apart from an error produced by the filter itself.
type emitError struct{ err error }

func (e *emitError) Error() string { return e.err.Error() }

func guard(emit func(Value) error) func(Value) error {
	return func(v Value) error {
		if err := emit(v); err != nil {
			return &emitError{err}
		}
		return nil
	}
}

// Run evaluates prog against input, feeding each output to emit.
func Run(prog Node, input Value, emit func(Value) error) error {
	return prog.eval(input, emit)
}

func (identity) eval(in Value, emit func(Value) error) error { return emit(in) }

func (recurseDefault) eval(in Value, emit func(Value) error) error {
	if err := emit(in); err != nil {
		return err
	}
	switch t := in.(type) {
	case []Value:
		for _, el := range t {
			if err := (recurseDefault{}).eval(el, emit); err != nil {
				return err
			}
		}
	case *Object:
		for _, k := range t.keys {
			v, _ := t.Get(k)
			if err := (recurseDefault{}).eval(v, emit); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f field) eval(in Value, emit func(Value) error) error {
	switch t := in.(type) {
	case nil:
		return emit(nil)
	case *Object:
		v, _ := t.Get(f.name)
		return emit(v)
	default:
		return cannotIndexStr(in, f.name)
	}
}

func cannotIndexStr(in Value, key string) *EvalError {
	return evalErrorf("Cannot index %s with string %q", TypeName(in), key)
}

func (n indexExpr) eval(in Value, emit func(Value) error) error {
	return n.idx.eval(in, func(idx Value) error {
		return indexInto(in, idx, emit)
	})
}

func indexInto(in, idx Value, emit func(Value) error) error {
	switch iv := idx.(type) {
	case string:
		switch t := in.(type) {
		case nil:
			return emit(nil)
		case *Object:
			v, _ := t.Get(iv)
			return emit(v)
		default:
			return cannotIndexStr(in, iv)
		}
	case float64:
		switch t := in.(type) {
		case nil:
			return emit(nil)
		case []Value:
			i := int(iv)
			if i < 0 {
				i += len(t)
			}
			if i < 0 || i >= len(t) {
				return emit(nil)
			}
			return emit(t[i])
		default:
			return evalErrorf("Cannot index %s with number", TypeName(in))
		}
	case nil:
		return evalErrorf("Cannot index %s with null", TypeName(in))
	default:
		return evalErrorf("Cannot index %s with %s", TypeName(in), TypeName(idx))
	}
}

func (n sliceExpr) eval(in Value, emit func(Value) error) error {
	los, err := collect(n.lo, in)
	if err != nil {
		return err
	}
	his, err := collect(n.hi, in)
	if err != nil {
		return err
	}
	for _, hi := range his {
		for _, lo := range los {
			if err := sliceInto(in, lo, hi, emit); err != nil {
				return err
			}
		}
	}
	return nil
}

// collect returns all outputs of n against in, or {nil} if n is nil (open end).
func collect(n Node, in Value) ([]Value, error) {
	if n == nil {
		return []Value{nil}, nil
	}
	var out []Value
	err := n.eval(in, func(v Value) error { out = append(out, v); return nil })
	return out, err
}

func sliceInto(in, lo, hi Value, emit func(Value) error) error {
	clamp := func(v Value, def, length int) (int, error) {
		if v == nil {
			return def, nil
		}
		f, ok := v.(float64)
		if !ok {
			return 0, evalErrorf("Start and end indices of an %s slice must be numbers", TypeName(in))
		}
		i := int(f)
		if i < 0 {
			i += length
		}
		if i < 0 {
			i = 0
		}
		if i > length {
			i = length
		}
		return i, nil
	}
	switch t := in.(type) {
	case nil:
		return emit(nil)
	case []Value:
		l, err := clamp(lo, 0, len(t))
		if err != nil {
			return err
		}
		h, err := clamp(hi, len(t), len(t))
		if err != nil {
			return err
		}
		if l > h {
			l = h
		}
		out := make([]Value, h-l)
		copy(out, t[l:h])
		return emit(out)
	case string:
		runes := []rune(t)
		l, err := clamp(lo, 0, len(runes))
		if err != nil {
			return err
		}
		h, err := clamp(hi, len(runes), len(runes))
		if err != nil {
			return err
		}
		if l > h {
			l = h
		}
		return emit(string(runes[l:h]))
	default:
		return evalErrorf("Cannot index %s with object", TypeName(in))
	}
}

func (iterate) eval(in Value, emit func(Value) error) error {
	switch t := in.(type) {
	case []Value:
		for _, el := range t {
			if err := emit(el); err != nil {
				return err
			}
		}
		return nil
	case *Object:
		for _, k := range t.keys {
			v, _ := t.Get(k)
			if err := emit(v); err != nil {
				return err
			}
		}
		return nil
	default:
		return evalErrorf("Cannot iterate over %s (%s)", TypeName(in), shortValue(in))
	}
}

func (n optional) eval(in Value, emit func(Value) error) error {
	err := n.inner.eval(in, guard(emit))
	if err == nil {
		return nil
	}
	if ee, ok := err.(*emitError); ok {
		return ee.err
	}
	if _, ok := err.(*EvalError); ok {
		return nil
	}
	return err
}

func (n pipe) eval(in Value, emit func(Value) error) error {
	return n.l.eval(in, func(v Value) error {
		return n.r.eval(v, emit)
	})
}

func (n comma) eval(in Value, emit func(Value) error) error {
	if err := n.l.eval(in, emit); err != nil {
		return err
	}
	return n.r.eval(in, emit)
}

func (n arrayConstruct) eval(in Value, emit func(Value) error) error {
	arr := []Value{}
	if n.inner != nil {
		err := n.inner.eval(in, func(v Value) error { arr = append(arr, v); return nil })
		if err != nil {
			return err
		}
	}
	return emit(arr)
}

func (n objectConstruct) eval(in Value, emit func(Value) error) error {
	return buildObject(n.entries, 0, in, NewObject(), emit)
}

func buildObject(entries []objEntry, i int, in Value, acc *Object, emit func(Value) error) error {
	if i == len(entries) {
		return emit(acc.Clone())
	}
	e := entries[i]
	return e.key.eval(in, func(k Value) error {
		ks, ok := k.(string)
		if !ok {
			return evalErrorf("Object keys must be strings, not %s", TypeName(k))
		}
		return e.val.eval(in, func(v Value) error {
			acc.Set(ks, v)
			return buildObject(entries, i+1, in, acc, emit)
		})
	})
}

func (n literal) eval(in Value, emit func(Value) error) error { return emit(n.val) }

func (n unaryNeg) eval(in Value, emit func(Value) error) error {
	return n.x.eval(in, func(v Value) error {
		f, ok := v.(float64)
		if !ok {
			return evalErrorf("%s (%s) cannot be negated", TypeName(v), shortValue(v))
		}
		return emit(-f)
	})
}

func (n binOp) eval(in Value, emit func(Value) error) error {
	return n.r.eval(in, func(rv Value) error {
		return n.l.eval(in, func(lv Value) error {
			res, err := applyBinary(n.op, lv, rv)
			if err != nil {
				return err
			}
			return emit(res)
		})
	})
}

func (n alternative) eval(in Value, emit func(Value) error) error {
	found := false
	gemit := guard(emit)
	err := n.l.eval(in, func(v Value) error {
		if Truthy(v) {
			found = true
			return gemit(v)
		}
		return nil
	})
	if err != nil {
		if ee, ok := err.(*emitError); ok {
			return ee.err
		}
		if _, ok := err.(*EvalError); !ok {
			return err
		}
		// production error on the left: fall through to the right
	}
	if found {
		return nil
	}
	return n.r.eval(in, emit)
}

func (n ifExpr) eval(in Value, emit func(Value) error) error {
	return n.evalFrom(0, in, emit)
}

func (n ifExpr) evalFrom(i int, in Value, emit func(Value) error) error {
	if i >= len(n.conds) {
		if n.els == nil {
			return emit(in)
		}
		return n.els.eval(in, emit)
	}
	return n.conds[i].eval(in, func(c Value) error {
		if Truthy(c) {
			return n.thens[i].eval(in, emit)
		}
		return n.evalFrom(i+1, in, emit)
	})
}

func (n tryCatch) eval(in Value, emit func(Value) error) error {
	err := n.body.eval(in, guard(emit))
	if err == nil {
		return nil
	}
	if ee, ok := err.(*emitError); ok {
		return ee.err
	}
	if ev, ok := err.(*EvalError); ok {
		if n.handler == nil {
			return nil
		}
		return n.handler.eval(ev.val, emit)
	}
	return err
}

func (n strInterp) eval(in Value, emit func(Value) error) error {
	return interpFrom(n.parts, n.format, 0, in, "", emit)
}

func interpFrom(parts []interpPart, format string, i int, in Value, acc string, emit func(Value) error) error {
	if i == len(parts) {
		return emit(acc)
	}
	p := parts[i]
	if !p.isExpr {
		return interpFrom(parts, format, i+1, in, acc+p.text, emit)
	}
	return p.expr.eval(in, func(v Value) error {
		s, err := applyFormat(format, v)
		if err != nil {
			return err
		}
		return interpFrom(parts, format, i+1, in, acc+s, emit)
	})
}

func (n formatExpr) eval(in Value, emit func(Value) error) error {
	s, err := applyFormatValue(n.name, in)
	if err != nil {
		return err
	}
	return emit(s)
}

func (n funcCall) eval(in Value, emit func(Value) error) error {
	fn, ok := builtins[n.name]
	if !ok {
		return evalErrorf("%s/%d is not defined", n.name, len(n.args))
	}
	return fn(n.args, in, emit)
}

// shortValue renders a value compactly for error messages, truncating long ones.
func shortValue(v Value) string {
	s := Encode(v, EncodeOptions{Compact: true})
	if len(s) > 11 {
		s = s[:10] + "..."
	}
	return strings.ReplaceAll(s, "\n", "")
}
