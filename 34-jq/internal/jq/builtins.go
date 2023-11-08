package jq

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// builtinFunc implements one builtin. args are the (lazy, unevaluated) argument
// filters; in is the current input; emit receives each output value.
type builtinFunc func(args []Node, in Value, emit func(Value) error) error

var builtins map[string]builtinFunc

func builtinExists(name string) bool {
	_, ok := builtins[name]
	return ok
}

// builtinArities lists the argument counts each builtin accepts. Like jq, a
// wrong arity is reported at compile time.
var builtinArities = map[string][]int{
	"empty": {0}, "error": {0, 1}, "not": {0}, "length": {0}, "utf8bytelength": {0},
	"keys": {0}, "keys_unsorted": {0}, "has": {1}, "in": {1}, "contains": {1},
	"inside": {1}, "type": {0}, "add": {0}, "any": {0, 1}, "all": {0, 1},
	"flatten": {0, 1}, "range": {1, 2, 3}, "min": {0}, "max": {0}, "min_by": {1},
	"max_by": {1}, "sort": {0}, "sort_by": {1}, "unique": {0}, "unique_by": {1},
	"group_by": {1}, "reverse": {0}, "to_entries": {0}, "from_entries": {0},
	"with_entries": {1}, "tostring": {0}, "tonumber": {0}, "ascii_downcase": {0},
	"ascii_upcase": {0}, "ltrimstr": {1}, "rtrimstr": {1}, "startswith": {1},
	"endswith": {1}, "split": {1}, "join": {1}, "test": {1}, "map": {1},
	"map_values": {1}, "select": {1}, "recurse": {0, 1}, "paths": {0},
	"getpath": {1}, "setpath": {2}, "del": {1}, "walk": {1}, "tojson": {0},
	"fromjson": {0}, "floor": {0},
	"values": {0}, "nulls": {0}, "booleans": {0}, "numbers": {0}, "strings": {0},
	"arrays": {0}, "objects": {0}, "iterables": {0}, "scalars": {0},
}

// builtinArityOK reports whether name accepts argc arguments.
func builtinArityOK(name string, argc int) bool {
	for _, a := range builtinArities[name] {
		if a == argc {
			return true
		}
	}
	return false
}

// firstOutput evaluates f against in and returns its first output (or null if
// it produces none).
func firstOutput(f Node, in Value) (Value, bool, error) {
	var result Value
	got := false
	err := f.eval(in, func(v Value) error {
		if !got {
			result = v
			got = true
		}
		return errStop
	})
	if err != nil && err != errStop {
		return nil, false, err
	}
	return result, got, nil
}

// errStop stops a stream after the first value. It is not an *EvalError, so
// try and `?` don't swallow it.
type stopSignal struct{}

func (stopSignal) Error() string { return "stop" }

var errStop error = stopSignal{}

func arrayArg(in Value, name string) ([]Value, error) {
	a, ok := in.([]Value)
	if !ok {
		return nil, evalErrorf("%s: input must be an array, got %s", name, TypeName(in))
	}
	return a, nil
}

func init() {
	builtins = map[string]builtinFunc{
		"empty":          biEmpty,
		"error":          biError,
		"not":            biNot,
		"length":         biLength,
		"utf8bytelength": biUTF8ByteLength,
		"keys":           biKeys,
		"keys_unsorted":  biKeysUnsorted,
		"has":            biHas,
		"in":             biIn,
		"contains":       biContains,
		"inside":         biInside,
		"type":           biType,
		"add":            biAdd,
		"any":            biAny,
		"all":            biAll,
		"flatten":        biFlatten,
		"range":          biRange,
		"min":            biMin,
		"max":            biMax,
		"min_by":         biMinBy,
		"max_by":         biMaxBy,
		"sort":           biSort,
		"sort_by":        biSortBy,
		"unique":         biUnique,
		"unique_by":      biUniqueBy,
		"group_by":       biGroupBy,
		"reverse":        biReverse,
		"to_entries":     biToEntries,
		"from_entries":   biFromEntries,
		"with_entries":   biWithEntries,
		"tostring":       biToString,
		"tonumber":       biToNumber,
		"ascii_downcase": biAsciiDowncase,
		"ascii_upcase":   biAsciiUpcase,
		"ltrimstr":       biLtrimstr,
		"rtrimstr":       biRtrimstr,
		"startswith":     biStartswith,
		"endswith":       biEndswith,
		"split":          biSplit,
		"join":           biJoin,
		"test":           biTest,
		"map":            biMap,
		"map_values":     biMapValues,
		"select":         biSelect,
		"recurse":        biRecurse,
		"paths":          biPaths,
		"getpath":        biGetpath,
		"setpath":        biSetpath,
		"del":            biDel,
		"walk":           biWalk,
		"tojson":         biTojson,
		"fromjson":       biFromjson,
		"floor":          biFloor,
		"values":         typeFilter(func(v Value) bool { return v != nil }),
		"nulls":          typeFilter(func(v Value) bool { return v == nil }),
		"booleans":       typeFilter(func(v Value) bool { _, ok := v.(bool); return ok }),
		"numbers":        typeFilter(func(v Value) bool { _, ok := v.(float64); return ok }),
		"strings":        typeFilter(func(v Value) bool { _, ok := v.(string); return ok }),
		"arrays":         typeFilter(func(v Value) bool { _, ok := v.([]Value); return ok }),
		"objects":        typeFilter(func(v Value) bool { _, ok := v.(*Object); return ok }),
		"iterables":      typeFilter(isIterable),
		"scalars":        typeFilter(func(v Value) bool { return !isIterable(v) }),
	}
}

func isIterable(v Value) bool {
	switch v.(type) {
	case []Value, *Object:
		return true
	}
	return false
}

func wantArity(args []Node, n int, name string) error {
	if len(args) != n {
		return evalErrorf("%s/%d is not defined", name, len(args))
	}
	return nil
}

func biEmpty(args []Node, in Value, emit func(Value) error) error { return nil }

func biError(args []Node, in Value, emit func(Value) error) error {
	if len(args) == 0 {
		return &EvalError{val: in}
	}
	return args[0].eval(in, func(v Value) error {
		return &EvalError{val: v}
	})
}

func biNot(args []Node, in Value, emit func(Value) error) error {
	return emit(!Truthy(in))
}

func biType(args []Node, in Value, emit func(Value) error) error {
	return emit(TypeName(in))
}

func biLength(args []Node, in Value, emit func(Value) error) error {
	switch t := in.(type) {
	case nil:
		return emit(float64(0))
	case bool:
		return evalErrorf("boolean (%v) has no length", t)
	case float64:
		if t < 0 {
			return emit(-t)
		}
		return emit(t)
	case string:
		return emit(float64(utf8.RuneCountInString(t)))
	case []Value:
		return emit(float64(len(t)))
	case *Object:
		return emit(float64(t.Len()))
	}
	return emit(float64(0))
}

func biUTF8ByteLength(args []Node, in Value, emit func(Value) error) error {
	s, ok := in.(string)
	if !ok {
		return evalErrorf("%s (%s) only strings have UTF-8 byte length", TypeName(in), shortValue(in))
	}
	return emit(float64(len(s)))
}

func biKeys(args []Node, in Value, emit func(Value) error) error {
	return keysImpl(in, true, emit)
}
func biKeysUnsorted(args []Node, in Value, emit func(Value) error) error {
	return keysImpl(in, false, emit)
}

func keysImpl(in Value, sortKeys bool, emit func(Value) error) error {
	switch t := in.(type) {
	case *Object:
		var ks []string
		if sortKeys {
			ks = sortedKeys(t)
		} else {
			ks = t.Keys()
		}
		out := make([]Value, len(ks))
		for i, k := range ks {
			out[i] = k
		}
		return emit(out)
	case []Value:
		out := make([]Value, len(t))
		for i := range t {
			out[i] = float64(i)
		}
		return emit(out)
	default:
		return evalErrorf("%s (%s) has no keys", TypeName(in), shortValue(in))
	}
}

func biHas(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "has"); err != nil {
		return err
	}
	return args[0].eval(in, func(key Value) error {
		ok, err := hasKey(in, key)
		if err != nil {
			return err
		}
		return emit(ok)
	})
}

func hasKey(in, key Value) (bool, error) {
	switch t := in.(type) {
	case *Object:
		ks, ok := key.(string)
		if !ok {
			return false, evalErrorf("Cannot check whether object has a key using %s", TypeName(key))
		}
		_, present := t.Get(ks)
		return present, nil
	case []Value:
		ki, ok := key.(float64)
		if !ok {
			return false, evalErrorf("Cannot check whether array has a key using %s", TypeName(key))
		}
		i := int(ki)
		return i >= 0 && i < len(t), nil
	default:
		return false, evalErrorf("Cannot check whether %s has a key", TypeName(in))
	}
}

func biIn(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "in"); err != nil {
		return err
	}
	return args[0].eval(in, func(container Value) error {
		ok, err := hasKey(container, in)
		if err != nil {
			return err
		}
		return emit(ok)
	})
}

func biContains(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "contains"); err != nil {
		return err
	}
	return args[0].eval(in, func(x Value) error {
		return emit(valueContains(in, x))
	})
}

func biInside(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "inside"); err != nil {
		return err
	}
	return args[0].eval(in, func(x Value) error {
		return emit(valueContains(x, in))
	})
}

// valueContains implements jq's contains: substrings for strings, and
// recursive containment for arrays and objects.
func valueContains(a, b Value) bool {
	switch bt := b.(type) {
	case *Object:
		at, ok := a.(*Object)
		if !ok {
			return false
		}
		for _, k := range bt.keys {
			bv, _ := bt.Get(k)
			av, present := at.Get(k)
			if !present || !valueContains(av, bv) {
				return false
			}
		}
		return true
	case []Value:
		at, ok := a.([]Value)
		if !ok {
			return false
		}
		for _, bel := range bt {
			found := false
			for _, ael := range at {
				if valueContains(ael, bel) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case string:
		as, ok := a.(string)
		if !ok {
			return false
		}
		return strings.Contains(as, bt)
	default:
		return Equal(a, b)
	}
}

func biAdd(args []Node, in Value, emit func(Value) error) error {
	var elems []Value
	switch t := in.(type) {
	case []Value:
		elems = t
	case *Object:
		for _, k := range t.keys {
			v, _ := t.Get(k)
			elems = append(elems, v)
		}
	case nil:
		return emit(nil)
	default:
		return evalErrorf("Cannot iterate over %s (%s)", TypeName(in), shortValue(in))
	}
	if len(elems) == 0 {
		return emit(nil)
	}
	var acc Value
	for i, el := range elems {
		if i == 0 {
			acc = el
			continue
		}
		var err error
		acc, err = addValues(acc, el)
		if err != nil {
			return err
		}
	}
	return emit(acc)
}

func biAny(args []Node, in Value, emit func(Value) error) error {
	return anyAll(args, in, emit, false)
}
func biAll(args []Node, in Value, emit func(Value) error) error {
	return anyAll(args, in, emit, true)
}

func anyAll(args []Node, in Value, emit func(Value) error, all bool) error {
	arr, err := arrayArg(in, "any/all")
	if err != nil {
		return err
	}
	var cond Node = identity{}
	if len(args) == 1 {
		cond = args[0]
	} else if len(args) > 1 {
		return evalErrorf("any/all with %d args not supported", len(args))
	}
	for _, el := range arr {
		v, _, err := firstOutput(cond, el)
		if err != nil {
			return err
		}
		t := Truthy(v)
		if all && !t {
			return emit(false)
		}
		if !all && t {
			return emit(true)
		}
	}
	return emit(all)
}

func biFlatten(args []Node, in Value, emit func(Value) error) error {
	arr, err := arrayArg(in, "flatten")
	if err != nil {
		return err
	}
	depth := 1 << 30
	if len(args) == 1 {
		v, _, err := firstOutput(args[0], in)
		if err != nil {
			return err
		}
		d, ok := v.(float64)
		if !ok {
			return evalErrorf("flatten depth must be a number")
		}
		depth = int(d)
	}
	out := []Value{}
	flattenInto(arr, depth, &out)
	return emit(out)
}

func flattenInto(arr []Value, depth int, out *[]Value) {
	for _, el := range arr {
		if sub, ok := el.([]Value); ok && depth > 0 {
			flattenInto(sub, depth-1, out)
		} else {
			*out = append(*out, el)
		}
	}
}

func biRange(args []Node, in Value, emit func(Value) error) error {
	switch len(args) {
	case 1:
		return args[0].eval(in, func(n Value) error {
			to, err := asNumber(n, "range")
			if err != nil {
				return err
			}
			return rangeEmit(0, to, 1, emit)
		})
	case 2:
		return args[0].eval(in, func(a Value) error {
			from, err := asNumber(a, "range")
			if err != nil {
				return err
			}
			return args[1].eval(in, func(b Value) error {
				to, err := asNumber(b, "range")
				if err != nil {
					return err
				}
				return rangeEmit(from, to, 1, emit)
			})
		})
	case 3:
		return args[0].eval(in, func(a Value) error {
			from, err := asNumber(a, "range")
			if err != nil {
				return err
			}
			return args[1].eval(in, func(b Value) error {
				to, err := asNumber(b, "range")
				if err != nil {
					return err
				}
				return args[2].eval(in, func(c Value) error {
					step, err := asNumber(c, "range")
					if err != nil {
						return err
					}
					return rangeEmit(from, to, step, emit)
				})
			})
		})
	default:
		return evalErrorf("range/%d is not defined", len(args))
	}
}

func rangeEmit(from, to, step float64, emit func(Value) error) error {
	if step == 0 {
		return nil
	}
	if step > 0 {
		for x := from; x < to; x += step {
			if err := emit(x); err != nil {
				return err
			}
		}
	} else {
		for x := from; x > to; x += step {
			if err := emit(x); err != nil {
				return err
			}
		}
	}
	return nil
}

func asNumber(v Value, who string) (float64, error) {
	f, ok := v.(float64)
	if !ok {
		return 0, evalErrorf("%s: expected number, got %s", who, TypeName(v))
	}
	return f, nil
}

func biMin(args []Node, in Value, emit func(Value) error) error {
	return minMax(in, emit, false)
}
func biMax(args []Node, in Value, emit func(Value) error) error {
	return minMax(in, emit, true)
}
func minMax(in Value, emit func(Value) error, max bool) error {
	arr, err := arrayArg(in, "min/max")
	if err != nil {
		return err
	}
	if len(arr) == 0 {
		return emit(nil)
	}
	best := arr[0]
	for _, el := range arr[1:] {
		c := Compare(el, best)
		if (max && c > 0) || (!max && c < 0) {
			best = el
		}
	}
	return emit(best)
}

func biMinBy(args []Node, in Value, emit func(Value) error) error {
	return minMaxBy(args, in, emit, false)
}
func biMaxBy(args []Node, in Value, emit func(Value) error) error {
	return minMaxBy(args, in, emit, true)
}
func minMaxBy(args []Node, in Value, emit func(Value) error, max bool) error {
	if err := wantArity(args, 1, "min_by/max_by"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "min_by/max_by")
	if err != nil {
		return err
	}
	if len(arr) == 0 {
		return emit(nil)
	}
	bestKey, _, err := firstOutput(args[0], arr[0])
	if err != nil {
		return err
	}
	best := arr[0]
	for _, el := range arr[1:] {
		k, _, err := firstOutput(args[0], el)
		if err != nil {
			return err
		}
		c := Compare(k, bestKey)
		if (max && c > 0) || (!max && c < 0) {
			best = el
			bestKey = k
		}
	}
	return emit(best)
}

func biSort(args []Node, in Value, emit func(Value) error) error {
	arr, err := arrayArg(in, "sort")
	if err != nil {
		return err
	}
	out := append([]Value{}, arr...)
	sort.SliceStable(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
	return emit(out)
}

func biSortBy(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "sort_by"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "sort_by")
	if err != nil {
		return err
	}
	keys, err := computeKeys(args[0], arr)
	if err != nil {
		return err
	}
	idx := indexSlice(len(arr))
	sort.SliceStable(idx, func(i, j int) bool { return Compare(keys[idx[i]], keys[idx[j]]) < 0 })
	out := reorder(arr, idx)
	return emit(out)
}

func biGroupBy(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "group_by"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "group_by")
	if err != nil {
		return err
	}
	keys, err := computeKeys(args[0], arr)
	if err != nil {
		return err
	}
	idx := indexSlice(len(arr))
	sort.SliceStable(idx, func(i, j int) bool { return Compare(keys[idx[i]], keys[idx[j]]) < 0 })
	out := []Value{}
	var cur []Value
	var curKey Value
	for n, i := range idx {
		if n == 0 || Compare(keys[i], curKey) != 0 {
			if cur != nil {
				out = append(out, cur)
			}
			cur = []Value{}
			curKey = keys[i]
		}
		cur = append(cur, arr[i])
	}
	if cur != nil {
		out = append(out, cur)
	}
	return emit(out)
}

func biUnique(args []Node, in Value, emit func(Value) error) error {
	arr, err := arrayArg(in, "unique")
	if err != nil {
		return err
	}
	out := append([]Value{}, arr...)
	sort.SliceStable(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
	deduped := []Value{}
	for i, el := range out {
		if i == 0 || Compare(el, out[i-1]) != 0 {
			deduped = append(deduped, el)
		}
	}
	return emit(deduped)
}

func biUniqueBy(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "unique_by"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "unique_by")
	if err != nil {
		return err
	}
	keys, err := computeKeys(args[0], arr)
	if err != nil {
		return err
	}
	idx := indexSlice(len(arr))
	sort.SliceStable(idx, func(i, j int) bool { return Compare(keys[idx[i]], keys[idx[j]]) < 0 })
	out := []Value{}
	for n, i := range idx {
		if n == 0 || Compare(keys[i], keys[idx[n-1]]) != 0 {
			out = append(out, arr[i])
		}
	}
	return emit(out)
}

func computeKeys(f Node, arr []Value) ([]Value, error) {
	keys := make([]Value, len(arr))
	for i, el := range arr {
		k, _, err := firstOutput(f, el)
		if err != nil {
			return nil, err
		}
		keys[i] = k
	}
	return keys, nil
}

func indexSlice(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}

func reorder(arr []Value, idx []int) []Value {
	out := make([]Value, len(idx))
	for i, j := range idx {
		out[i] = arr[j]
	}
	return out
}

func biReverse(args []Node, in Value, emit func(Value) error) error {
	switch t := in.(type) {
	case []Value:
		out := make([]Value, len(t))
		for i, el := range t {
			out[len(t)-1-i] = el
		}
		return emit(out)
	case string:
		rs := []rune(t)
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		return emit(string(rs))
	case nil:
		return emit(nil)
	default:
		return evalErrorf("Cannot reverse %s", TypeName(in))
	}
}

func biToEntries(args []Node, in Value, emit func(Value) error) error {
	obj, ok := in.(*Object)
	if !ok {
		return evalErrorf("to_entries: input must be an object, got %s", TypeName(in))
	}
	out := make([]Value, 0, obj.Len())
	for _, k := range obj.keys {
		v, _ := obj.Get(k)
		e := NewObject()
		e.Set("key", k)
		e.Set("value", v)
		out = append(out, e)
	}
	return emit(out)
}

func biFromEntries(args []Node, in Value, emit func(Value) error) error {
	arr, err := arrayArg(in, "from_entries")
	if err != nil {
		return err
	}
	out := NewObject()
	for _, el := range arr {
		o, ok := el.(*Object)
		if !ok {
			return evalErrorf("from_entries: array elements must be objects")
		}
		key := firstKeyOf(o, "key", "k", "name", "Name", "Key", "K")
		val, hasVal := lookupAny(o, "value", "v", "Value", "V")
		if !hasVal {
			val = nil
		}
		ks, err := entryKeyToString(key)
		if err != nil {
			return err
		}
		out.Set(ks, val)
	}
	return emit(out)
}

func firstKeyOf(o *Object, names ...string) Value {
	for _, n := range names {
		if v, ok := o.Get(n); ok {
			return v
		}
	}
	return nil
}

func lookupAny(o *Object, names ...string) (Value, bool) {
	for _, n := range names {
		if v, ok := o.Get(n); ok {
			return v, true
		}
	}
	return nil, false
}

func entryKeyToString(key Value) (string, error) {
	switch k := key.(type) {
	case string:
		return k, nil
	case float64:
		return FormatNumber(k), nil
	case nil:
		return "null", nil
	case bool:
		if k {
			return "true", nil
		}
		return "false", nil
	default:
		return "", evalErrorf("Cannot use %s as object key", TypeName(key))
	}
}

func biWithEntries(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "with_entries"); err != nil {
		return err
	}
	// with_entries(f) == to_entries | map(f) | from_entries
	return biToEntries(nil, in, func(entries Value) error {
		arr := entries.([]Value)
		mapped := make([]Value, 0, len(arr))
		for _, e := range arr {
			v, got, err := firstOutput(args[0], e)
			if err != nil {
				return err
			}
			if got {
				mapped = append(mapped, v)
			}
		}
		return biFromEntries(nil, mapped, emit)
	})
}

func biToString(args []Node, in Value, emit func(Value) error) error {
	if s, ok := in.(string); ok {
		return emit(s)
	}
	return emit(Encode(in, EncodeOptions{Compact: true}))
}

func biToNumber(args []Node, in Value, emit func(Value) error) error {
	switch t := in.(type) {
	case float64:
		return emit(t)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return evalErrorf("Cannot parse '%s' as number", t)
		}
		return emit(f)
	default:
		return evalErrorf("Cannot parse %s as number", TypeName(in))
	}
}

func biAsciiDowncase(args []Node, in Value, emit func(Value) error) error {
	s, err := stringArg(in, "ascii_downcase")
	if err != nil {
		return err
	}
	return emit(asciiMap(s, 'A', 'Z', 32))
}
func biAsciiUpcase(args []Node, in Value, emit func(Value) error) error {
	s, err := stringArg(in, "ascii_upcase")
	if err != nil {
		return err
	}
	return emit(asciiMap(s, 'a', 'z', -32))
}

func asciiMap(s string, lo, hi byte, delta int) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= lo && b[i] <= hi {
			b[i] = byte(int(b[i]) + delta)
		}
	}
	return string(b)
}

func stringArg(in Value, who string) (string, error) {
	s, ok := in.(string)
	if !ok {
		return "", evalErrorf("%s (%s) cannot be %s; input must be a string", TypeName(in), shortValue(in), who)
	}
	return s, nil
}

func biLtrimstr(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "ltrimstr"); err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		s, sok := in.(string)
		ps, pok := p.(string)
		if sok && pok && strings.HasPrefix(s, ps) {
			return emit(s[len(ps):])
		}
		return emit(in)
	})
}

func biRtrimstr(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "rtrimstr"); err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		s, sok := in.(string)
		ps, pok := p.(string)
		if sok && pok && strings.HasSuffix(s, ps) {
			return emit(s[:len(s)-len(ps)])
		}
		return emit(in)
	})
}

func biStartswith(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "startswith"); err != nil {
		return err
	}
	s, err := stringArg(in, "startswith")
	if err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		ps, ok := p.(string)
		if !ok {
			return evalErrorf("startswith() requires string inputs")
		}
		return emit(strings.HasPrefix(s, ps))
	})
}

func biEndswith(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "endswith"); err != nil {
		return err
	}
	s, err := stringArg(in, "endswith")
	if err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		ps, ok := p.(string)
		if !ok {
			return evalErrorf("endswith() requires string inputs")
		}
		return emit(strings.HasSuffix(s, ps))
	})
}

func biSplit(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "split"); err != nil {
		return err
	}
	s, err := stringArg(in, "split")
	if err != nil {
		return err
	}
	return args[0].eval(in, func(sep Value) error {
		ss, ok := sep.(string)
		if !ok {
			return evalErrorf("split input and separator must be strings")
		}
		parts := splitString(s, ss)
		out := make([]Value, len(parts))
		for i, p := range parts {
			out[i] = p
		}
		return emit(out)
	})
}

func biJoin(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "join"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "join")
	if err != nil {
		return err
	}
	return args[0].eval(in, func(sep Value) error {
		ss, ok := sep.(string)
		if !ok {
			return evalErrorf("join separator must be a string")
		}
		var b strings.Builder
		for i, el := range arr {
			if i > 0 {
				b.WriteString(ss)
			}
			switch t := el.(type) {
			case nil:
				// jq treats null as empty
			case string:
				b.WriteString(t)
			case float64:
				b.WriteString(FormatNumber(t))
			case bool:
				if t {
					b.WriteString("true")
				} else {
					b.WriteString("false")
				}
			default:
				return evalErrorf("Cannot join with %s", TypeName(el))
			}
		}
		return emit(b.String())
	})
}

func biTest(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "test"); err != nil {
		return err
	}
	s, err := stringArg(in, "test")
	if err != nil {
		return err
	}
	return args[0].eval(in, func(re Value) error {
		pat, ok := re.(string)
		if !ok {
			return evalErrorf("test() requires a string regex")
		}
		ok2, err := regexTest(s, pat)
		if err != nil {
			return err
		}
		return emit(ok2)
	})
}

func biMap(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "map"); err != nil {
		return err
	}
	arr, err := arrayArg(in, "map")
	if err != nil {
		return err
	}
	out := []Value{}
	for _, el := range arr {
		if err := args[0].eval(el, func(v Value) error {
			out = append(out, v)
			return nil
		}); err != nil {
			return err
		}
	}
	return emit(out)
}

func biMapValues(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "map_values"); err != nil {
		return err
	}
	switch t := in.(type) {
	case []Value:
		out := []Value{}
		for _, el := range t {
			v, got, err := firstOutput(args[0], el)
			if err != nil {
				return err
			}
			if got {
				out = append(out, v)
			}
		}
		return emit(out)
	case *Object:
		out := NewObject()
		for _, k := range t.keys {
			val, _ := t.Get(k)
			v, got, err := firstOutput(args[0], val)
			if err != nil {
				return err
			}
			if got {
				out.Set(k, v)
			}
		}
		return emit(out)
	default:
		return evalErrorf("Cannot iterate over %s", TypeName(in))
	}
}

func biSelect(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "select"); err != nil {
		return err
	}
	return args[0].eval(in, func(c Value) error {
		if Truthy(c) {
			return emit(in)
		}
		return nil
	})
}

func biRecurse(args []Node, in Value, emit func(Value) error) error {
	if len(args) == 0 {
		return (recurseDefault{}).eval(in, emit)
	}
	if len(args) != 1 {
		return evalErrorf("recurse/%d is not defined", len(args))
	}
	return recurseF(args[0], in, emit)
}

func recurseF(f Node, in Value, emit func(Value) error) error {
	if err := emit(in); err != nil {
		return err
	}
	return f.eval(in, func(v Value) error {
		return recurseF(f, v, emit)
	})
}

func biPaths(args []Node, in Value, emit func(Value) error) error {
	var out [][]Value
	allPaths(in, nil, &out)
	for _, p := range out {
		arr := make([]Value, len(p))
		copy(arr, p)
		if err := emit(arr); err != nil {
			return err
		}
	}
	return nil
}

func biGetpath(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "getpath"); err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		path, err := toPath(p)
		if err != nil {
			return err
		}
		v, err := getPath(in, path)
		if err != nil {
			return err
		}
		return emit(v)
	})
}

func biSetpath(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 2, "setpath"); err != nil {
		return err
	}
	return args[0].eval(in, func(p Value) error {
		path, err := toPath(p)
		if err != nil {
			return err
		}
		return args[1].eval(in, func(val Value) error {
			nv, err := setPath(in, path, val)
			if err != nil {
				return err
			}
			return emit(nv)
		})
	})
}

func biDel(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "del"); err != nil {
		return err
	}
	paths, err := evalPaths(args[0], in)
	if err != nil {
		return err
	}
	// Delete deeper / higher indices first so earlier deletions don't shift
	// the remaining paths.
	sort.SliceStable(paths, func(i, j int) bool {
		return comparePathsForDelete(paths[i], paths[j]) > 0
	})
	cur := in
	for _, p := range paths {
		cur, err = delPath(cur, p)
		if err != nil {
			return err
		}
	}
	return emit(cur)
}

func comparePathsForDelete(a, b []Value) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		c := Compare(a[i], b[i])
		if c != 0 {
			return c
		}
	}
	return intCmp(len(a), len(b))
}

func toPath(p Value) ([]Value, error) {
	arr, ok := p.([]Value)
	if !ok {
		return nil, evalErrorf("Path must be specified as an array, got %s", TypeName(p))
	}
	for _, el := range arr {
		switch el.(type) {
		case string, float64:
		default:
			return nil, evalErrorf("Path components must be strings or numbers, got %s", TypeName(el))
		}
	}
	return arr, nil
}

func biWalk(args []Node, in Value, emit func(Value) error) error {
	if err := wantArity(args, 1, "walk"); err != nil {
		return err
	}
	v, err := walkValue(args[0], in)
	if err != nil {
		return err
	}
	return emit(v)
}

func walkValue(f Node, in Value) (Value, error) {
	var transformed Value
	switch t := in.(type) {
	case []Value:
		out := make([]Value, len(t))
		for i, el := range t {
			nv, err := walkValue(f, el)
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		transformed = out
	case *Object:
		out := NewObject()
		for _, k := range t.keys {
			el, _ := t.Get(k)
			nv, err := walkValue(f, el)
			if err != nil {
				return nil, err
			}
			out.Set(k, nv)
		}
		transformed = out
	default:
		transformed = in
	}
	res, _, err := firstOutput(f, transformed)
	return res, err
}

func biTojson(args []Node, in Value, emit func(Value) error) error {
	return emit(Encode(in, EncodeOptions{Compact: true}))
}

func biFromjson(args []Node, in Value, emit func(Value) error) error {
	s, ok := in.(string)
	if !ok {
		return evalErrorf("fromjson: input must be a string, got %s", TypeName(in))
	}
	v, err := ParseValue([]byte(s))
	if err != nil {
		return evalErrorf("fromjson: %v", err)
	}
	return emit(v)
}

func biFloor(args []Node, in Value, emit func(Value) error) error {
	f, ok := in.(float64)
	if !ok {
		return evalErrorf("floor: input must be a number, got %s", TypeName(in))
	}
	out := float64(int64(f))
	if f < 0 && out != f {
		out--
	}
	return emit(out)
}

// typeFilter builds builtins like numbers and strings that pass their input
// through only when pred holds.
func typeFilter(pred func(Value) bool) builtinFunc {
	return func(args []Node, in Value, emit func(Value) error) error {
		if pred(in) {
			return emit(in)
		}
		return nil
	}
}
