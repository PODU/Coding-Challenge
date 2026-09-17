package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cchyperfine/internal/bench"
	"cchyperfine/internal/stats"
)

func result(name string, params map[string]string, times ...float64) *bench.Result {
	r := &bench.Result{Job: bench.Job{Command: name, Name: name, Parameters: params}, Times: times}
	for range times {
		r.ExitCodes = append(r.ExitCodes, 0)
	}
	r.Wall = stats.Summarize(times)
	r.UserT = stats.Summarize([]float64{0.001})
	r.SysT = stats.Summarize([]float64{0.002})
	return r
}

func sample() []*bench.Result {
	return []*bench.Result{
		result("fast > out", map[string]string{"n": "1"}, 0.010, 0.012, 0.011),
		result("slow", map[string]string{"n": "2"}, 0.020, 0.022, 0.021),
	}
}

func write(t *testing.T, w Writer, results []*bench.Result) {
	t.Helper()
	if err := w.WriteAll(results); err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func TestJSONSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	write(t, Writer{JSON: path}, sample())

	var got struct {
		Results []struct {
			Command    string            `json:"command"`
			Mean       float64           `json:"mean"`
			StdDev     float64           `json:"stddev"`
			Median     float64           `json:"median"`
			Min        float64           `json:"min"`
			Max        float64           `json:"max"`
			Times      []float64         `json:"times"`
			ExitCodes  []int             `json:"exit_codes"`
			Parameters map[string]string `json:"parameters"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(read(t, path)), &got); err != nil {
		t.Fatalf("export is not valid JSON: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(got.Results))
	}
	first := got.Results[0]
	if first.Command != "fast > out" {
		t.Errorf("command = %q", first.Command)
	}
	if len(first.Times) != 3 || len(first.ExitCodes) != 3 {
		t.Errorf("raw samples not exported: %+v", first)
	}
	if first.Parameters["n"] != "1" {
		t.Errorf("parameters not exported: %+v", first.Parameters)
	}
	if first.Min != 0.010 || first.Max != 0.012 {
		t.Errorf("min/max = %v/%v", first.Min, first.Max)
	}
}

func TestJSONDoesNotEscapeRedirections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	write(t, Writer{JSON: path}, sample())
	out := read(t, path)
	if strings.Contains(out, "u003e") {
		t.Error("the > in a command was HTML-escaped")
	}
	if !strings.Contains(out, `"fast > out"`) {
		t.Errorf("command not written verbatim:\n%s", out)
	}
}

func TestCSVHeaderAndParameterColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	write(t, Writer{CSV: path}, sample())

	lines := strings.Split(strings.TrimSpace(read(t, path)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header and two rows:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	want := "command,mean,stddev,median,user,system,min,max,parameter_n"
	if strings.TrimSpace(lines[0]) != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[1]), ",1") {
		t.Errorf("first row is missing its parameter value: %q", lines[1])
	}
}

func TestMarkdownTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.md")
	write(t, Writer{Markdown: path}, sample())

	out := read(t, path)
	if !strings.Contains(out, "| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |") {
		t.Errorf("unexpected header:\n%s", out)
	}
	if !strings.Contains(out, "| `fast > out` |") {
		t.Errorf("command not wrapped in a code span:\n%s", out)
	}
	if !strings.Contains(out, "| 1.00 |") {
		t.Errorf("fastest command should have relative speed 1.00:\n%s", out)
	}
	if !strings.Contains(out, "±") {
		t.Errorf("relative speed should carry an error margin:\n%s", out)
	}
}

func TestAsciidocAndOrgmode(t *testing.T) {
	dir := t.TempDir()
	adoc := filepath.Join(dir, "out.adoc")
	org := filepath.Join(dir, "out.org")
	write(t, Writer{Asciidoc: adoc, Orgmode: org}, sample())

	a := read(t, adoc)
	if !strings.HasPrefix(a, `[cols="<,>,>,>,>"]`) || !strings.Contains(a, "|===") {
		t.Errorf("asciidoc table malformed:\n%s", a)
	}
	o := read(t, org)
	if !strings.Contains(o, "| =slow= |") {
		t.Errorf("orgmode should use verbatim markup:\n%s", o)
	}
}

func TestWriteAllReportsTheFirstFailure(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "ok.csv")
	bad := filepath.Join(dir, "missing-dir", "out.json")

	err := Writer{JSON: bad, CSV: good}.WriteAll(sample())
	if err == nil {
		t.Fatal("expected an error for an unwritable path")
	}
	if _, statErr := os.Stat(good); statErr != nil {
		t.Error("a failing export should not prevent the others from being written")
	}
}

func TestEscapeBackticks(t *testing.T) {
	if got := escapeBackticks("echo `date`"); strings.Contains(got, "`") {
		t.Errorf("backticks survived escaping: %q", got)
	}
}
