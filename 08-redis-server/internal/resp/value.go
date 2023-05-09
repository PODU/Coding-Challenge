// Package resp encodes and decodes RESP2, the Redis wire protocol. It knows
// nothing about commands or storage.
package resp

// Kind identifies a RESP2 wire type by its prefix byte.
type Kind byte

const (
	// Simple is a "+OK\r\n" style simple string.
	Simple Kind = '+'
	// Error is a "-Error message\r\n" style error string.
	Error Kind = '-'
	// Integer is a ":1000\r\n" style 64-bit signed integer.
	Integer Kind = ':'
	// Bulk is a "$5\r\nhello\r\n" length-prefixed, binary-safe string.
	Bulk Kind = '$'
	// Array is a "*1\r\n...\r\n" sequence of nested values.
	Array Kind = '*'
)

// Value holds any of the five RESP types; Kind says which field is in use.
type Value struct {
	Kind   Kind
	Str    string  // simple string / error / bulk string payload
	Int    int64   // integer payload
	Arr    []Value // array elements
	IsNull bool    // null bulk string ($-1) or null array (*-1)
}

// SimpleString builds a "+s\r\n" reply.
func SimpleString(s string) Value { return Value{Kind: Simple, Str: s} }

// ErrorString builds a "-s\r\n" reply.
func ErrorString(s string) Value { return Value{Kind: Error, Str: s} }

// Int builds a ":n\r\n" reply.
func Int(n int64) Value { return Value{Kind: Integer, Int: n} }

// BulkString builds a "$len\r\ns\r\n" reply (binary-safe).
func BulkString(s string) Value { return Value{Kind: Bulk, Str: s} }

// NullBulk builds the "$-1\r\n" null bulk string (e.g. GET on a miss).
func NullBulk() Value { return Value{Kind: Bulk, IsNull: true} }

// ArrayOf builds a "*len\r\n..." reply from the given elements.
func ArrayOf(elems ...Value) Value { return Value{Kind: Array, Arr: elems} }

// ArrayFrom builds an array reply from a slice of elements.
func ArrayFrom(elems []Value) Value { return Value{Kind: Array, Arr: elems} }

// NullArray builds the "*-1\r\n" null array.
func NullArray() Value { return Value{Kind: Array, IsNull: true} }

// BulkArray wraps items as an array of bulk strings.
func BulkArray(items []string) Value {
	elems := make([]Value, len(items))
	for i, s := range items {
		elems[i] = BulkString(s)
	}
	return Value{Kind: Array, Arr: elems}
}

// Equal reports whether two values are structurally identical.
func (v Value) Equal(o Value) bool {
	if v.Kind != o.Kind || v.IsNull != o.IsNull {
		return false
	}
	switch v.Kind {
	case Simple, Error, Bulk:
		return v.Str == o.Str
	case Integer:
		return v.Int == o.Int
	case Array:
		if len(v.Arr) != len(o.Arr) {
			return false
		}
		for i := range v.Arr {
			if !v.Arr[i].Equal(o.Arr[i]) {
				return false
			}
		}
		return true
	}
	return false
}
