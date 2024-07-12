// Package yaml parses a subset of YAML into a typed value tree, with no
// third-party library.
package yaml

import (
	"fmt"

	"yamlcc/internal/parser"
	"yamlcc/internal/value"
)

// Value is the parsed-node type, re-exported so callers need only this package.
type Value = value.Value

// Parse parses a YAML stream into one Value per document.
func Parse(src []byte) ([]Value, error) {
	return parser.Parse(src, false)
}

// ParseStrict is Parse, but with strict set it rejects normally tolerated
// constructs such as directives.
func ParseStrict(src []byte, strict bool) ([]Value, error) {
	return parser.Parse(src, strict)
}

// ParseOne parses a stream of at most one document. Empty input gives null;
// more than one document is an error.
func ParseOne(src []byte) (Value, error) {
	docs, err := parser.Parse(src, false)
	if err != nil {
		return Value{}, err
	}
	switch len(docs) {
	case 0:
		return value.NullValue(), nil
	case 1:
		return docs[0], nil
	default:
		return Value{}, fmt.Errorf("expected a single document, got %d", len(docs))
	}
}
