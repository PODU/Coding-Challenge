package yamlio

import (
	"bytes"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadYAMLMultiDoc(t *testing.T) {
	data := []byte("a: 1\n---\nb: 2\n---\nc: 3\n")
	docs, err := LoadYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("got %d docs, want 3", len(docs))
	}
	for _, d := range docs {
		if d.Kind != yaml.MappingNode {
			t.Errorf("doc kind = %v, want mapping", d.Kind)
		}
	}
}

func TestLoadJSONTypeFidelity(t *testing.T) {
	docs, err := LoadJSON([]byte(`{"i":42,"f":1.5,"s":"42","b":true,"n":null,"a":[1,2]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs", len(docs))
	}
	want := map[string]string{
		"i": "!!int", "f": "!!float", "s": "!!str",
		"b": "!!bool", "n": "!!null", "a": "!!seq",
	}
	m := docs[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		v := m.Content[i+1]
		if want[k] != "" && v.Tag != want[k] {
			t.Errorf("key %q tag = %q, want %q", k, v.Tag, want[k])
		}
	}
}

func TestEmitJSONFromYAML(t *testing.T) {
	docs, _ := LoadYAML([]byte("name: x\nnums:\n  - 1\n  - 2\n"))
	var buf bytes.Buffer
	if err := Emit(&buf, docs, Options{Format: "json", Compact: true}); err != nil {
		t.Fatal(err)
	}
	want := `{"name":"x","nums":[1,2]}` + "\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// YAML -> JSON -> YAML must preserve the data (compared semantically).
func TestRoundTrip(t *testing.T) {
	src := []byte("name: x\nage: 30\ntags:\n  - a\n  - b\nactive: true\n")
	yamlDocs, _ := LoadYAML(src)

	var jsonBuf bytes.Buffer
	if err := Emit(&jsonBuf, yamlDocs, Options{Format: "json"}); err != nil {
		t.Fatal(err)
	}
	jsonDocs, err := LoadJSON(jsonBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var yamlBuf bytes.Buffer
	if err := Emit(&yamlBuf, jsonDocs, Options{Format: "yaml"}); err != nil {
		t.Fatal(err)
	}

	var a, b any
	if err := yaml.Unmarshal(src, &a); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(yamlBuf.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("round-trip mismatch:\n in: %#v\nout: %#v", a, b)
	}
}

func TestEmitYAMLDocSep(t *testing.T) {
	docs, _ := LoadYAML([]byte("a: 1\n---\nb: 2\n"))

	var withSep bytes.Buffer
	Emit(&withSep, docs, Options{Format: "yaml"})
	if !bytes.Contains(withSep.Bytes(), []byte("---")) {
		t.Errorf("expected '---' separator, got %q", withSep.String())
	}

	var noSep bytes.Buffer
	Emit(&noSep, docs, Options{Format: "yaml", NoDocSep: true})
	if bytes.Contains(noSep.Bytes(), []byte("---")) {
		t.Errorf("did not expect '---', got %q", noSep.String())
	}
}

func TestRawScalar(t *testing.T) {
	docs, _ := LoadYAML([]byte(`greeting: "hello world"`))
	val := docs[0].Content[1] // the scalar value node
	var buf bytes.Buffer
	if err := Emit(&buf, []*yaml.Node{val}, Options{Format: "yaml", Raw: true}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello world\n" {
		t.Errorf("raw output = %q, want %q", buf.String(), "hello world\n")
	}
}
