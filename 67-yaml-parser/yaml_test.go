package yaml_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "yamlcc"
	"yamlcc/internal/emit"
	"yamlcc/internal/value"
)

var update = flag.Bool("update", false, "regenerate golden .json files")

// renderDocs mirrors the CLI's JSON document-combination rule: a single
// document is emitted bare, multiple documents as a JSON array.
func renderDocs(docs []value.Value) []byte {
	switch len(docs) {
	case 0:
		return []byte("null")
	case 1:
		return emit.Marshal(docs[0], 2)
	default:
		return emit.Marshal(value.SeqValue(docs), 2)
	}
}

func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/valid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no valid fixtures found")
	}
	for _, f := range files {
		f := f
		name := strings.TrimSuffix(filepath.Base(f), ".yaml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			docs, err := yaml.Parse(src)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			got := renderDocs(docs)

			goldenPath := strings.TrimSuffix(f, ".yaml") + ".json"
			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			if strings.TrimRight(string(got), "\n") != strings.TrimRight(string(want), "\n") {
				t.Errorf("JSON mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}

func TestInvalid(t *testing.T) {
	wantSubstr := map[string]string{
		"tab_indent":          "tab",
		"bad_indent":          "indentation",
		"unterminated_dquote": "unterminated",
		"unterminated_squote": "unterminated",
		"unclosed_flow_seq":   "flow",
		"unclosed_flow_map":   "flow",
		"bad_escape":          "escape",
		"unsupported_anchor":  "unsupported",
	}
	files, err := filepath.Glob("testdata/invalid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		f := f
		name := strings.TrimSuffix(filepath.Base(f), ".yaml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = yaml.Parse(src)
			if err == nil {
				t.Fatalf("expected error, got none")
			}
			sub, ok := wantSubstr[name]
			if !ok {
				t.Fatalf("no expected substring registered for %s", name)
			}
			if !strings.Contains(err.Error(), sub) {
				t.Errorf("error %q does not contain %q", err.Error(), sub)
			}
		})
	}
}

func TestParseOne(t *testing.T) {
	v, err := yaml.ParseOne([]byte("name: test\nvalue: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != value.Map || len(v.Items) != 2 {
		t.Fatalf("unexpected value: %+v", v)
	}

	// Empty input yields a null value, not an error.
	v, err = yaml.ParseOne(nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != value.Null {
		t.Fatalf("expected null for empty input, got %+v", v)
	}

	// Multiple documents are an error for ParseOne.
	if _, err := yaml.ParseOne([]byte("---\na: 1\n---\nb: 2\n")); err == nil {
		t.Fatal("expected error for multi-document input")
	}
}

func TestParseMultiDocCount(t *testing.T) {
	docs, err := yaml.Parse([]byte("---\na: 1\n---\nb: 2\n...\n---\nc: 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}
}

func TestEmptyStream(t *testing.T) {
	docs, err := yaml.Parse([]byte("# only a comment\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 documents, got %d", len(docs))
	}
}
