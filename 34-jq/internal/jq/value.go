// Package jq implements a subset of the jq JSON processor.
//
// A filter maps one input to a stream of zero or more outputs. Outputs are
// passed to a callback rather than collected, so filters like limit and
// first can stop early without building slices.
package jq

// Value is a JSON value, one of:
//
//	nil          -> JSON null
//	bool         -> JSON true/false
//	float64      -> JSON number
//	string       -> JSON string
//	[]Value      -> JSON array
//	*Object      -> JSON object (insertion ordered)
//
// jq keeps object keys in insertion order, which a Go map can't do.
type Value = interface{}

// Object is an insertion-ordered JSON object.
type Object struct {
	keys []string
	m    map[string]Value
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{m: map[string]Value{}}
}

// Len returns the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// Keys returns the keys in insertion order. The slice must not be mutated.
func (o *Object) Keys() []string { return o.keys }

// Get returns the value for key and whether it exists.
func (o *Object) Get(key string) (Value, bool) {
	v, ok := o.m[key]
	return v, ok
}

// Set updates key in place, or appends it if new.
func (o *Object) Set(key string, v Value) {
	if _, ok := o.m[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.m[key] = v
}

// Delete removes key if present.
func (o *Object) Delete(key string) {
	if _, ok := o.m[key]; !ok {
		return
	}
	delete(o.m, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Clone returns a shallow copy.
func (o *Object) Clone() *Object {
	n := &Object{keys: make([]string, len(o.keys)), m: make(map[string]Value, len(o.m))}
	copy(n.keys, o.keys)
	for k, v := range o.m {
		n.m[k] = v
	}
	return n
}

// TypeName returns the jq type name of v: one of
// "null", "boolean", "number", "string", "array", "object".
func TypeName(v Value) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []Value:
		return "array"
	case *Object:
		return "object"
	default:
		return "unknown"
	}
}

// Truthy reports jq truthiness: false and null are falsy, everything else truthy.
func Truthy(v Value) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	default:
		return true
	}
}
