package parser

import "strconv"

// Value is a JSON value node. Pos is the 1-based position where it starts.
type Value interface {
	value()
	Pos() (line, col int)
}

type position struct {
	line int
	col  int
}

func (p position) Pos() (int, int) { return p.line, p.col }

// Object is a JSON object. Members keep source order, and duplicate keys are
// kept as-is since RFC 8259 allows them.
type Object struct {
	position
	Members []Member
}

// Member is one key/value pair of an Object.
type Member struct {
	Key   string
	Value Value
}

// Array is a JSON array.
type Array struct {
	position
	Elements []Value
}

// String holds the decoded string value.
type String struct {
	position
	Value string
}

// Number is a JSON number. Raw is the exact source text; Float can lose
// precision, so treat Raw as the real value.
type Number struct {
	position
	Raw   string
	Float float64
}

// Bool is a JSON true or false.
type Bool struct {
	position
	Value bool
}

// Null is a JSON null.
type Null struct {
	position
}

func (Object) value() {}
func (Array) value()  {}
func (String) value() {}
func (Number) value() {}
func (Bool) value()   {}
func (Null) value()   {}

// ParsedFloat parses Raw as a float64. Out-of-range values become +/-Inf;
// range isn't a syntax error, so the error is dropped.
func (n *Number) ParsedFloat() float64 {
	f, _ := strconv.ParseFloat(n.Raw, 64)
	return f
}
