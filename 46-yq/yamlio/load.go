// Package yamlio wraps gopkg.in/yaml.v3 and encoding/json to load input
// documents into a format-agnostic *yaml.Node model and emit result nodes as
// YAML or JSON.
package yamlio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// LoadYAML returns the root content node of each document in a YAML stream.
func LoadYAML(data []byte) ([]*yaml.Node, error) {
	var docs []*yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
		docs = append(docs, unwrap(&doc))
	}
	return docs, nil
}

// unwrap returns the content node of a DocumentNode, or the node itself.
func unwrap(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 1 {
			return n.Content[0]
		}
		// Empty document -> null.
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	return n
}

// LoadJSON converts a stream of whitespace-separated JSON values to yaml.Nodes.
func LoadJSON(data []byte) ([]*yaml.Node, error) {
	var docs []*yaml.Node
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	for {
		var v any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing JSON: %w", err)
		}
		docs = append(docs, jsonToNode(v))
	}
	return docs, nil
}

// jsonToNode converts a decoded JSON value to a yaml.Node. Object keys come
// out sorted, since the decoded map has no order.
func jsonToNode(v any) *yaml.Node {
	switch t := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case bool:
		val := "false"
		if t {
			val = "true"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val}
	case json.Number:
		tag := "!!float"
		if _, err := t.Int64(); err == nil {
			tag = "!!int"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: t.String()}
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}
	case []any:
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, e := range t {
			seq.Content = append(seq.Content, jsonToNode(e))
		}
		return seq
	case map[string]any:
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			m.Content = append(m.Content, key, jsonToNode(t[k]))
		}
		return m
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%v", t)}
	}
}
