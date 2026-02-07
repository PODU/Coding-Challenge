package validator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGoldenFiles walks testdata/golden/<keyword>/ directories. Each directory
// contains a schema.json plus instance files:
//   - valid_*.json     must validate with zero errors,
//   - invalid_*.json    must fail, and the produced "<instancePath>|<keyword>"
//     error keys must match the matching invalid_*.golden file exactly.
//
// This is the file-based golden harness, complementing
// the in-code table tests.
func TestGoldenFiles(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "golden")
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read golden root: %v", err)
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(root, d.Name())
		t.Run(d.Name(), func(t *testing.T) {
			schemaBytes, err := os.ReadFile(filepath.Join(dir, "schema.json"))
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			schema, err := ParseJSON(schemaBytes)
			if err != nil {
				t.Fatalf("parse schema: %v", err)
			}

			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				name := e.Name()
				if !strings.HasSuffix(name, ".json") || name == "schema.json" {
					continue
				}
				runGoldenInstance(t, schema, dir, name)
			}
		})
	}
}

func runGoldenInstance(t *testing.T, schema interface{}, dir, name string) {
	t.Run(name, func(t *testing.T) {
		instBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read instance: %v", err)
		}
		instance, err := ParseJSON(instBytes)
		if err != nil {
			t.Fatalf("parse instance: %v", err)
		}

		res := New(schema, Options{}).Validate(instance)

		switch {
		case strings.HasPrefix(name, "valid"):
			if !res.Valid {
				t.Fatalf("expected valid, got errors: %+v", res.Errors)
			}
		case strings.HasPrefix(name, "invalid"):
			if res.Valid {
				t.Fatalf("expected invalid, got valid")
			}
			goldenPath := filepath.Join(dir, strings.TrimSuffix(name, ".json")+".golden")
			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}
			want := splitNonEmpty(string(wantBytes))
			got := make([]string, 0, len(res.Errors))
			for _, e := range res.Errors {
				got = append(got, e.InstancePath+"|"+e.Keyword)
			}
			sort.Strings(want)
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("golden mismatch\n got: %v\nwant: %v\nfull: %+v", got, want, res.Errors)
			}
		default:
			t.Fatalf("instance %q must be named valid_*/invalid_*", name)
		}
	})
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
