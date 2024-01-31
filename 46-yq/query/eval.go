package query

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// EvalError is a runtime evaluation error (jq-like), distinct from parse errors.
type EvalError struct{ msg string }

func (e *EvalError) Error() string { return e.msg }

func evalErrf(format string, a ...any) error {
	return &EvalError{fmt.Sprintf(format, a...)}
}

// resolve follows aliases to the anchored node.
func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// deepResolve replaces aliases at any depth with copies of their targets, so a
// subtree pulled out of its document still makes sense on its own. A node with
// no aliases inside is returned as is, keeping its comments and styles.
func deepResolve(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode {
		r := deepResolve(n.Alias)
		if r != nil && r.Anchor != "" {
			cp := *r
			cp.Anchor = ""
			return &cp
		}
		return r
	}
	if len(n.Content) == 0 {
		return n
	}
	resolved := make([]*yaml.Node, len(n.Content))
	changed := false
	for i, c := range n.Content {
		resolved[i] = deepResolve(c)
		if resolved[i] != c {
			changed = true
		}
	}
	if !changed {
		return n
	}
	cp := *n
	cp.Anchor = ""
	cp.Content = resolved
	return &cp
}

func nullNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
}

func typeName(n *yaml.Node) string {
	if n == nil {
		return "null"
	}
	switch n.Kind {
	case yaml.MappingNode:
		return "object"
	case yaml.SequenceNode:
		return "array"
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!null":
			return "null"
		case "!!bool":
			return "boolean"
		case "!!int", "!!float":
			return "number"
		default:
			return "string"
		}
	case yaml.AliasNode:
		return "alias"
	default:
		return "value"
	}
}

func (Identity) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	return []*yaml.Node{in}, nil
}

func (f Field) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	n := resolve(in)
	if n == nil || n.Kind != yaml.MappingNode {
		if f.Optional {
			return nil, nil
		}
		return nil, evalErrf("cannot index %s with %q", typeName(n), f.Name)
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := resolve(n.Content[i])
		if key.Kind == yaml.ScalarNode && key.Value == f.Name {
			return []*yaml.Node{deepResolve(n.Content[i+1])}, nil
		}
	}
	if f.Optional {
		return nil, nil
	}
	return nil, evalErrf("object has no key %q", f.Name)
}

func (f Index) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	n := resolve(in)
	if n == nil || n.Kind != yaml.SequenceNode {
		if f.Optional {
			return nil, nil
		}
		return nil, evalErrf("cannot index %s with number", typeName(n))
	}
	idx := f.N
	if idx < 0 {
		idx += len(n.Content)
	}
	if idx < 0 || idx >= len(n.Content) {
		return []*yaml.Node{nullNode()}, nil
	}
	return []*yaml.Node{deepResolve(n.Content[idx])}, nil
}

func (f Iterate) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	n := resolve(in)
	switch {
	case n != nil && n.Kind == yaml.SequenceNode:
		out := make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			out[i] = deepResolve(c)
		}
		return out, nil
	case n != nil && n.Kind == yaml.MappingNode:
		out := make([]*yaml.Node, 0, len(n.Content)/2)
		for i := 1; i < len(n.Content); i += 2 {
			out = append(out, deepResolve(n.Content[i]))
		}
		return out, nil
	default:
		if f.Optional {
			return nil, nil
		}
		return nil, evalErrf("cannot iterate over %s", typeName(n))
	}
}

func (f Pipe) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	left, err := f.Left.Eval(in)
	if err != nil {
		return nil, err
	}
	var out []*yaml.Node
	for _, n := range left {
		r, err := f.Right.Eval(n)
		if err != nil {
			return nil, err
		}
		out = append(out, r...)
	}
	return out, nil
}

func (f Collect) Eval(in *yaml.Node) ([]*yaml.Node, error) {
	items, err := f.Inner.Eval(in)
	if err != nil {
		return nil, err
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seq.Content = append(seq.Content, items...)
	return []*yaml.Node{seq}, nil
}

func (Identity) String() string { return "." }

func (f Field) String() string {
	s := "." + f.Name
	if f.Optional {
		s += "?"
	}
	return s
}

func (f Index) String() string {
	s := ".[" + strconv.Itoa(f.N) + "]"
	if f.Optional {
		s += "?"
	}
	return s
}

func (f Iterate) String() string {
	if f.Optional {
		return ".[]?"
	}
	return ".[]"
}

func (f Pipe) String() string { return f.Left.String() + " | " + f.Right.String() }

func (f Collect) String() string { return "[" + f.Inner.String() + "]" }
