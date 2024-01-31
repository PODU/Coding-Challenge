package query

import "gopkg.in/yaml.v3"

// Filter is the common interface for every node in a compiled query.
//
// Following jq's model, a filter maps a single input value to a *stream* of
// zero or more output values. Values are represented as *yaml.Node so that
// comments, key order, tags and scalar styles survive end-to-end.
type Filter interface {
	// Eval evaluates the filter against one input node and returns the
	// resulting stream. A nil error with an empty slice means "no output".
	Eval(in *yaml.Node) ([]*yaml.Node, error)
	// String renders the filter back to source-like form (debugging/tests).
	String() string
}

// Identity is the `.` filter: it returns its input unchanged.
type Identity struct{}

// Field is object access `.name` / `.["name"]`: it selects a mapping value by key.
type Field struct {
	Name     string
	Optional bool // set by a trailing '?'
}

// Index is array access `.[n]` with negative indices and out-of-range -> null.
type Index struct {
	N        int
	Optional bool
}

// Iterate is `.[]`: it streams every element of a sequence or value of a mapping.
type Iterate struct {
	Optional bool
}

// Pipe is `A | B`: it threads each output of A as input to B (flatMap).
// It is also used internally to compose consecutive path segments.
type Pipe struct {
	Left, Right Filter
}

// Collect is `[ E ]`: it gathers the whole stream produced by E into one
// sequence node.
type Collect struct {
	Inner Filter
}
