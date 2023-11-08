package jq

// getPath navigates in along path (a slice of string/number keys), returning
// null for any missing component.
func getPath(in Value, path []Value) (Value, error) {
	cur := in
	for _, key := range path {
		switch k := key.(type) {
		case string:
			switch t := cur.(type) {
			case nil:
				cur = nil
			case *Object:
				v, _ := t.Get(k)
				cur = v
			default:
				return nil, cannotIndexStr(cur, k)
			}
		case float64:
			switch t := cur.(type) {
			case nil:
				cur = nil
			case []Value:
				i := int(k)
				if i < 0 {
					i += len(t)
				}
				if i < 0 || i >= len(t) {
					cur = nil
				} else {
					cur = t[i]
				}
			default:
				return nil, evalErrorf("Cannot index %s with number", TypeName(cur))
			}
		default:
			return nil, evalErrorf("Invalid path component %s", TypeName(key))
		}
	}
	return cur, nil
}

// setPath returns a copy of in with the value at path replaced by v.
func setPath(in Value, path []Value, v Value) (Value, error) {
	if len(path) == 0 {
		return v, nil
	}
	key := path[0]
	switch k := key.(type) {
	case string:
		var obj *Object
		switch t := in.(type) {
		case nil:
			obj = NewObject()
		case *Object:
			obj = t.Clone()
		default:
			return nil, evalErrorf("Cannot index %s with %q", TypeName(in), k)
		}
		child, _ := obj.Get(k)
		nv, err := setPath(child, path[1:], v)
		if err != nil {
			return nil, err
		}
		obj.Set(k, nv)
		return obj, nil
	case float64:
		var arr []Value
		switch t := in.(type) {
		case nil:
			arr = []Value{}
		case []Value:
			arr = append([]Value{}, t...)
		default:
			return nil, evalErrorf("Cannot index %s with number", TypeName(in))
		}
		i := int(k)
		if i < 0 {
			i += len(arr)
			if i < 0 {
				return nil, evalErrorf("Out of bounds negative array index")
			}
		}
		for len(arr) <= i {
			arr = append(arr, nil)
		}
		nv, err := setPath(arr[i], path[1:], v)
		if err != nil {
			return nil, err
		}
		arr[i] = nv
		return arr, nil
	default:
		return nil, evalErrorf("Invalid path component %s", TypeName(key))
	}
}

// delPath removes the value at a single path from in, returning a copy.
func delPath(in Value, path []Value) (Value, error) {
	if len(path) == 0 {
		return nil, nil
	}
	key := path[0]
	if len(path) == 1 {
		switch k := key.(type) {
		case string:
			switch t := in.(type) {
			case nil:
				return nil, nil
			case *Object:
				o := t.Clone()
				o.Delete(k)
				return o, nil
			default:
				return nil, evalErrorf("Cannot delete field of %s", TypeName(in))
			}
		case float64:
			switch t := in.(type) {
			case nil:
				return nil, nil
			case []Value:
				i := int(k)
				if i < 0 {
					i += len(t)
				}
				if i < 0 || i >= len(t) {
					return append([]Value{}, t...), nil
				}
				out := make([]Value, 0, len(t)-1)
				out = append(out, t[:i]...)
				out = append(out, t[i+1:]...)
				return out, nil
			default:
				return nil, evalErrorf("Cannot delete element of %s", TypeName(in))
			}
		default:
			return nil, evalErrorf("Invalid path component %s", TypeName(key))
		}
	}
	child, err := getPath(in, path[:1])
	if err != nil {
		return nil, err
	}
	nv, err := delPath(child, path[1:])
	if err != nil {
		return nil, err
	}
	return setPath(in, path[:1], nv)
}

// evalPaths returns the paths selected by a path expression such as .a[0] or
// .[]? when evaluated against in.
func evalPaths(n Node, in Value) ([][]Value, error) {
	switch t := n.(type) {
	case identity:
		return [][]Value{{}}, nil
	case field:
		return [][]Value{{t.name}}, nil
	case indexExpr:
		var out [][]Value
		var idxs []Value
		err := t.idx.eval(in, func(v Value) error { idxs = append(idxs, v); return nil })
		if err != nil {
			return nil, err
		}
		for _, iv := range idxs {
			switch iv.(type) {
			case string, float64:
				out = append(out, []Value{iv})
			default:
				return nil, evalErrorf("Invalid path expression with result %s", TypeName(iv))
			}
		}
		return out, nil
	case iterate:
		switch v := in.(type) {
		case []Value:
			out := make([][]Value, len(v))
			for i := range v {
				out[i] = []Value{float64(i)}
			}
			return out, nil
		case *Object:
			out := make([][]Value, 0, v.Len())
			for _, k := range v.keys {
				out = append(out, []Value{k})
			}
			return out, nil
		case nil:
			return nil, evalErrorf("Cannot iterate over null")
		default:
			return nil, evalErrorf("Cannot iterate over %s", TypeName(in))
		}
	case pipe:
		lpaths, err := evalPaths(t.l, in)
		if err != nil {
			return nil, err
		}
		var out [][]Value
		for _, lp := range lpaths {
			sub, err := getPath(in, lp)
			if err != nil {
				return nil, err
			}
			rpaths, err := evalPaths(t.r, sub)
			if err != nil {
				return nil, err
			}
			for _, rp := range rpaths {
				combined := append(append([]Value{}, lp...), rp...)
				out = append(out, combined)
			}
		}
		return out, nil
	case optional:
		paths, err := evalPaths(t.inner, in)
		if err != nil {
			if _, ok := err.(*EvalError); ok {
				return nil, nil
			}
			return nil, err
		}
		return paths, nil
	case literal:
		// e.g. del(empty) style; a non-path literal is invalid
		return nil, evalErrorf("Invalid path expression")
	default:
		return nil, evalErrorf("Invalid path expression")
	}
}

// allPaths returns every path to every (sub)value in v, excluding the root,
// in document order (used by the paths builtin).
func allPaths(v Value, prefix []Value, out *[][]Value) {
	switch t := v.(type) {
	case []Value:
		for i, el := range t {
			p := append(append([]Value{}, prefix...), float64(i))
			*out = append(*out, p)
			allPaths(el, p, out)
		}
	case *Object:
		for _, k := range t.keys {
			el, _ := t.Get(k)
			p := append(append([]Value{}, prefix...), k)
			*out = append(*out, p)
			allPaths(el, p, out)
		}
	}
}
