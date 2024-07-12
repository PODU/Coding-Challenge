// Package value holds the parsed data model and resolves plain scalars to
// typed values.
package value

// Kind is the type a Value holds.
type Kind int

const (
	Null Kind = iota
	Bool
	Int
	Float
	Str
	Seq
	Map
)

// KV is one mapping entry. Keys are strings so they map onto JSON keys.
type KV struct {
	Key string
	Val Value
}

// Value is a single YAML node; which fields are set depends on Kind.
type Value struct {
	Kind  Kind
	Bool  bool
	Int   int64
	Float float64
	Str   string
	Seq   []Value
	Items []KV // mapping entries, in source order
}

func NullValue() Value           { return Value{Kind: Null} }
func BoolValue(b bool) Value     { return Value{Kind: Bool, Bool: b} }
func IntValue(n int64) Value     { return Value{Kind: Int, Int: n} }
func FloatValue(f float64) Value { return Value{Kind: Float, Float: f} }
func StrValue(s string) Value    { return Value{Kind: Str, Str: s} }

// SeqValue returns a sequence value, never with a nil slice.
func SeqValue(items []Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{Kind: Seq, Seq: items}
}

// MapValue returns a mapping value, never with a nil slice.
func MapValue(items []KV) Value {
	if items == nil {
		items = []KV{}
	}
	return Value{Kind: Map, Items: items}
}
