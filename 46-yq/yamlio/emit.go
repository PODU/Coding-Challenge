package yamlio

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Options controls how a result stream is rendered.
type Options struct {
	Format   string // "yaml" or "json"
	Raw      bool   // -r: emit string scalars without quotes
	Compact  bool   // -c: compact JSON (one doc per line)
	NoDocSep bool   // omit '---' between YAML documents
}

// Emit writes the result stream to w according to opts.
func Emit(w io.Writer, nodes []*yaml.Node, opts Options) error {
	switch opts.Format {
	case "json":
		return emitJSON(w, nodes, opts)
	default:
		return emitYAML(w, nodes, opts)
	}
}

// rawScalar returns (value, true) when raw mode should print n as a bare string.
func rawScalar(n *yaml.Node, raw bool) (string, bool) {
	if !raw || n == nil {
		return "", false
	}
	if n.Kind == yaml.ScalarNode && n.Tag != "!!null" {
		return n.Value, true
	}
	return "", false
}

func emitYAML(w io.Writer, nodes []*yaml.Node, opts Options) error {
	for i, n := range nodes {
		if i > 0 && !opts.NoDocSep {
			if _, err := io.WriteString(w, "---\n"); err != nil {
				return err
			}
		}
		if s, ok := rawScalar(n, opts.Raw); ok {
			if _, err := io.WriteString(w, s+"\n"); err != nil {
				return err
			}
			continue
		}
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		if err := enc.Encode(n); err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
		if err := enc.Close(); err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
	}
	return nil
}

func emitJSON(w io.Writer, nodes []*yaml.Node, opts Options) error {
	for _, n := range nodes {
		if s, ok := rawScalar(n, opts.Raw); ok {
			if _, err := io.WriteString(w, s+"\n"); err != nil {
				return err
			}
			continue
		}
		v, err := nodeToInterface(n)
		if err != nil {
			return err
		}
		var b []byte
		if opts.Compact {
			b, err = json.Marshal(v)
		} else {
			b, err = json.MarshalIndent(v, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// nodeToInterface decodes a yaml.Node into a Go value suitable for json.Marshal,
// preserving number/bool/null/string fidelity via the YAML tag resolver.
func nodeToInterface(n *yaml.Node) (any, error) {
	if n == nil {
		return nil, nil
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, fmt.Errorf("converting node for JSON: %w", err)
	}
	return v, nil
}
