package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdataDir resolves the repository's testdata directory relative to this
// package (internal/parser).
func testdataDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("testdata not found at %s: %v", dir, err)
	}
	return dir
}

// TestStagedFixtures runs the staged challenge fixtures. Files whose name
// starts with "valid" must parse; files starting with "invalid" must fail.
func TestStagedFixtures(t *testing.T) {
	root := testdataDir(t)
	for _, step := range []string{"step1", "step2", "step3", "step4"} {
		step := step
		t.Run(step, func(t *testing.T) {
			entries, err := os.ReadDir(filepath.Join(root, step))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				name := e.Name()
				path := filepath.Join(root, step, name)
				t.Run(name, func(t *testing.T) {
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					_, perr := Parse(data)
					wantValid := strings.HasPrefix(name, "valid")
					if wantValid && perr != nil {
						t.Errorf("%s: want valid, got error: %v", name, perr)
					}
					if !wantValid && perr == nil {
						t.Errorf("%s: want invalid, but parsed OK", name)
					}
				})
			}
		})
	}
}

// jsonCheckerWhitelist lists json.org JSON_checker "fail" cases that are
// actually valid under RFC 8259 (the design follows the RFC):
//   - fail1:  a top-level string is a legal JSON text in RFC 8259.
//   - fail18: RFC 8259 imposes no nesting-depth limit.
var jsonCheckerWhitelist = map[string]bool{
	"fail1.json":  true,
	"fail18.json": true,
}

// TestJSONCheckerConformance runs the canonical json.org suite: pass1-3 must
// validate; fail1-33 must be rejected except the documented RFC-8259
// divergences in jsonCheckerWhitelist.
func TestJSONCheckerConformance(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "json_checker")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			_, perr := Parse(data)

			wantValid := strings.HasPrefix(name, "pass") || jsonCheckerWhitelist[name]
			if wantValid && perr != nil {
				t.Errorf("%s: want valid, got error: %v", name, perr)
			}
			if !wantValid && perr == nil {
				t.Errorf("%s: want invalid, but parsed OK", name)
			}
		})
	}
}
