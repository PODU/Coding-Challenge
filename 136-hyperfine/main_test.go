package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cchyperfine/internal/cli"
)

// capture runs the CLI with the given arguments, returning its exit code and
// the text it wrote to stdout and stderr.
func capture(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	code = run(args, out, errf)
	out.Close()
	errf.Close()

	o, _ := os.ReadFile(filepath.Join(dir, "stdout"))
	e, _ := os.ReadFile(filepath.Join(dir, "stderr"))
	return code, string(o), string(e)
}

func trivial() string {
	if os.PathSeparator == '\\' {
		return "cmd.exe /c exit 0"
	}
	return "true"
}

func TestHelp(t *testing.T) {
	code, out, _ := capture(t, "--help")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"USAGE:", "--parameter-scan", "HYPERFINE_ITERATION", "--export-json"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q", want)
		}
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := capture(t, "--version")
	if code != 0 || !strings.Contains(out, "cchyperfine") {
		t.Errorf("code = %d, stdout = %q", code, out)
	}
}

func TestUsageErrorExitsTwo(t *testing.T) {
	code, _, errOut := capture(t)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "no commands given") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestBenchmarkFailureExitsOne(t *testing.T) {
	code, _, errOut := capture(t, "-r", "1", "--style", "none", "exit 7")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "exit code 7") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestEndToEndSingleCommand(t *testing.T) {
	code, out, errOut := capture(t, "-N", "-r", "3", "--style", "none", trivial())
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Benchmark 1: "+trivial()) {
		t.Errorf("missing benchmark header:\n%s", out)
	}
	if !strings.Contains(out, "3 runs") {
		t.Errorf("missing run count:\n%s", out)
	}
	if strings.Contains(out, "Summary") {
		t.Errorf("a single benchmark should not print a summary:\n%s", out)
	}
}

func TestEndToEndComparisonAndExports(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "r.json")
	mdPath := filepath.Join(dir, "r.md")

	code, out, errOut := capture(t,
		"-N", "-r", "2", "--style", "none",
		"-n", "first", "-n", "second",
		"--export-json", jsonPath, "--export-markdown", mdPath,
		trivial(), trivial())
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Benchmark 1: first") || !strings.Contains(out, "Benchmark 2: second") {
		t.Errorf("--command-name not applied:\n%s", out)
	}
	if !strings.Contains(out, "Summary") || !strings.Contains(out, "times faster than") {
		t.Errorf("missing comparison summary:\n%s", out)
	}
	for _, p := range []string{jsonPath, mdPath} {
		if b, err := os.ReadFile(p); err != nil || len(b) == 0 {
			t.Errorf("export %s missing or empty (%v)", p, err)
		}
	}
}

func TestEndToEndParameterScanCreatesOneBenchmarkPerValue(t *testing.T) {
	code, out, errOut := capture(t, "-N", "-r", "1", "--style", "none",
		"-P", "n", "1", "3", trivial()+" {n}")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut)
	}
	for i, v := range []string{"1", "2", "3"} {
		want := "Benchmark " + string(rune('1'+i)) + ": " + trivial() + " " + v
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestReferenceIsBenchmarkedFirstAndUsedAsBaseline(t *testing.T) {
	code, out, errOut := capture(t, "-N", "-r", "2", "--style", "none",
		"--reference", trivial(), trivial()+" x")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Benchmark 1: "+trivial()+"\n") {
		t.Errorf("the reference should be benchmarked first:\n%s", out)
	}
	if !strings.Contains(out, trivial()+" ran") {
		t.Errorf("the reference should be the summary baseline:\n%s", out)
	}
}

func TestBuildJobsPairsHooksWithCommands(t *testing.T) {
	o, err := cli.Parse([]string{"-p", "a", "-p", "b", "one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := buildJobs(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	if jobs[0].Prepare != "a" || jobs[1].Prepare != "b" {
		t.Errorf("prepare hooks = %q, %q", jobs[0].Prepare, jobs[1].Prepare)
	}
	if jobs[0].Index != 0 || jobs[1].Index != 1 {
		t.Errorf("job indices = %d, %d", jobs[0].Index, jobs[1].Index)
	}
}

func TestBuildJobsSubstitutesParametersEverywhere(t *testing.T) {
	o, err := cli.Parse([]string{"-L", "n", "1,2", "-n", "run-{n}", "-p", "setup {n}", "cmd {n}"})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := buildJobs(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	if jobs[0].Command != "cmd 1" || jobs[0].Name != "run-1" || jobs[0].Prepare != "setup 1" {
		t.Errorf("job 0 = %+v", jobs[0])
	}
	if jobs[1].Parameters["n"] != "2" {
		t.Errorf("parameters not recorded: %+v", jobs[1].Parameters)
	}
}

func TestBuildJobsRejectsBadParameterSpec(t *testing.T) {
	o, err := cli.Parse([]string{"-P", "n", "5", "1", "cmd {n}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildJobs(o); err == nil {
		t.Error("expected an error for a range that never reaches its bound")
	}
}
