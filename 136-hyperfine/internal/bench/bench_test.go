package bench

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cchyperfine/internal/cli"
	"cchyperfine/internal/exec"
)

// opts builds a minimal option set for tests: a fixed run count keeps them fast
// and deterministic, and the shell is disabled so no calibration runs happen
// unless a test asks for one.
func opts(runs int) *cli.Options {
	o, err := cli.Parse([]string{"-r", itoa(runs), "-N", "placeholder"})
	if err != nil {
		panic(err)
	}
	return o
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// shellOpts is like opts but keeps the platform shell, which is needed for any
// test that uses shell syntax such as `exit 3` or output redirection.
func shellOpts(runs int) *cli.Options {
	o, err := cli.Parse([]string{"-r", itoa(runs), "placeholder"})
	if err != nil {
		panic(err)
	}
	return o
}

func trivial() string {
	if isWindows() {
		return "cmd.exe /c exit 0"
	}
	return "true"
}

func isWindows() bool { return os.PathSeparator == '\\' }

func TestRunHonoursExplicitRunCount(t *testing.T) {
	r := New(opts(4), nil, io.Discard)
	res, err := r.Run(Job{Command: trivial(), Name: trivial()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Times) != 4 {
		t.Errorf("got %d timings, want 4", len(res.Times))
	}
	if len(res.ExitCodes) != 4 || res.ExitCodes[0] != 0 {
		t.Errorf("exit codes = %v", res.ExitCodes)
	}
	if res.Wall.Count != 4 {
		t.Errorf("summary count = %d, want 4", res.Wall.Count)
	}
}

func TestWarmupRunsAreNotMeasured(t *testing.T) {
	o := opts(2)
	o.Warmup = 3
	r := New(o, nil, io.Discard)
	res, err := r.Run(Job{Command: trivial()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Times) != 2 {
		t.Errorf("got %d timings, want only the 2 measured runs", len(res.Times))
	}
}

func TestHooksRunInOrderAndAreNotMeasured(t *testing.T) {
	log := filepath.Join(t.TempDir(), "hooks.txt")
	appendTo := func(word string) string {
		return "echo " + word + " >> " + log
	}

	o := shellOpts(2)
	o.Warmup = 1
	o.Setup = appendTo("setup")
	o.Cleanup = appendTo("cleanup")

	r := New(o, nil, io.Discard)
	res, err := r.Run(Job{
		Command:  trivial(),
		Prepare:  appendTo("prepare"),
		Conclude: appendTo("conclude"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Times) != 2 {
		t.Fatalf("got %d timings, want 2", len(res.Times))
	}

	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("hooks did not run: %v", err)
	}
	var got []string
	for _, line := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			got = append(got, s)
		}
	}
	// setup once, then prepare/conclude around each of 1 warmup + 2 timed runs,
	// then cleanup once.
	want := []string{"setup", "prepare", "conclude", "prepare", "conclude", "prepare", "conclude", "cleanup"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hook order = %v, want %v", got, want)
	}
}

func TestNonZeroExitAborts(t *testing.T) {
	r := New(shellOpts(2), nil, io.Discard)
	_, err := r.Run(Job{Command: "exit 3"})
	if err == nil {
		t.Fatal("expected an error for a failing command")
	}
	if !strings.Contains(err.Error(), "--ignore-failure") {
		t.Errorf("error should point at the escape hatch: %v", err)
	}
}

func TestIgnoreFailureRecordsAnAlert(t *testing.T) {
	o := shellOpts(2)
	o.IgnoreFail = true
	r := New(o, nil, io.Discard)
	res, err := r.Run(Job{Command: "exit 3"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasAlert(res, AlertNonZeroExit) {
		t.Errorf("expected a non-zero-exit alert, got %+v", res.Alerts)
	}
	for _, c := range res.ExitCodes {
		if c != 3 {
			t.Errorf("exit codes = %v, want all 3", res.ExitCodes)
			break
		}
	}
}

func TestFailingHookAbortsEvenWithIgnoreFailure(t *testing.T) {
	o := shellOpts(2)
	o.IgnoreFail = true
	r := New(o, nil, io.Discard)
	_, err := r.Run(Job{Command: trivial(), Prepare: "exit 1"})
	if err == nil || !strings.Contains(err.Error(), "prepare") {
		t.Fatalf("a failing prepare must abort the benchmark, got %v", err)
	}
}

func TestShellCalibrationHappensOnceAndOnlyWithAShell(t *testing.T) {
	r := New(shellOpts(1), nil, io.Discard)
	if _, err := r.Run(Job{Command: trivial()}); err != nil {
		t.Fatal(err)
	}
	if r.ShellOverhead() <= 0 {
		t.Errorf("shell overhead = %v, want a positive measurement", r.ShellOverhead())
	}

	direct := New(opts(1), nil, io.Discard)
	if _, err := direct.Run(Job{Command: trivial()}); err != nil {
		t.Fatal(err)
	}
	if direct.ShellOverhead() != 0 {
		t.Errorf("shell overhead without a shell = %v, want 0", direct.ShellOverhead())
	}
}

func TestEstimateRuns(t *testing.T) {
	o := opts(0)
	o.MinRuns = 10
	o.MaxRuns = 0
	r := New(o, nil, io.Discard)

	// 3 s of benchmarking at 1 s per run is 3 runs, but --min-runs wins.
	if got := r.estimateRuns(1.0); got != 10 {
		t.Errorf("estimateRuns(1s) = %d, want the 10-run floor", got)
	}
	// 3 s at 10 s per run rounds up to a single run, still floored at 10.
	if got := r.estimateRuns(10.0); got != 10 {
		t.Errorf("estimateRuns(10s) = %d, want 10", got)
	}
	// 3 s at 1 ms per run is 3000 runs.
	if got := r.estimateRuns(0.001); got != 3000 {
		t.Errorf("estimateRuns(1ms) = %d, want 3000", got)
	}

	o.MaxRuns = 100
	if got := r.estimateRuns(0.001); got != 100 {
		t.Errorf("estimateRuns with --max-runs = %d, want 100", got)
	}
}

func TestAutomaticRunCountIsUsedWhenRunsIsUnset(t *testing.T) {
	o := opts(0)
	o.MinRuns = 3
	o.MaxRuns = 3
	r := New(o, nil, io.Discard)
	res, err := r.Run(Job{Command: trivial()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Times) != 3 {
		t.Errorf("got %d timings, want 3", len(res.Times))
	}
}

func TestIterationEnvIsExposed(t *testing.T) {
	out := filepath.Join(t.TempDir(), "iters.txt")
	marker := "$HYPERFINE_ITERATION"
	if isWindows() {
		marker = "%HYPERFINE_ITERATION%"
	}
	r := New(shellOpts(3), nil, io.Discard)
	if _, err := r.Run(Job{Command: "echo " + marker + " >> " + out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(b))
	if len(got) != 3 || got[0] != "0" || got[2] != "2" {
		t.Errorf("iterations = %v, want 0 1 2", got)
	}
}

func TestCorrectClampsAtZero(t *testing.T) {
	r := &Runner{Opts: opts(1), shellOverhead: 1.0, shellOverheadUser: 1.0, shellOverheadSys: 1.0}
	got := r.correct(exec.Timing{Real: 0.5, User: 0.1, System: 0.1, ExitCode: 0})
	if got.Real != 0 || got.User != 0 || got.System != 0 {
		t.Errorf("correct = %+v, want all components clamped to 0", got)
	}
	if got.ExitCode != 0 {
		t.Errorf("correct dropped the exit code")
	}
}

func TestTooFastAlert(t *testing.T) {
	r := New(opts(1), nil, io.Discard)
	res := &Result{Times: []float64{0.001, 0.001}}
	res.Wall.Mean = 0.001
	if alerts := r.alerts(res); !containsKind(alerts, AlertTooFast) {
		t.Errorf("expected a too-fast alert, got %+v", alerts)
	}
}

func TestSlowFirstRunAlertBeatsTheGenericOutlierAlert(t *testing.T) {
	r := New(opts(1), nil, io.Discard)
	res := &Result{Times: []float64{9.0, 1.00, 1.01, 0.99, 1.02}}
	res.Wall.Mean = 2.6
	alerts := r.alerts(res)
	if !containsKind(alerts, AlertSlowFirst) {
		t.Errorf("expected a slow-first-run alert, got %+v", alerts)
	}
	if containsKind(alerts, AlertOutliers) {
		t.Errorf("the generic outlier alert should be suppressed, got %+v", alerts)
	}
}

func TestOutlierAlert(t *testing.T) {
	r := New(opts(1), nil, io.Discard)
	res := &Result{Times: []float64{1.00, 1.01, 0.99, 12.0, 1.02}}
	res.Wall.Mean = 3.2
	if alerts := r.alerts(res); !containsKind(alerts, AlertOutliers) {
		t.Errorf("expected an outlier alert, got %+v", alerts)
	}
}

func TestNoAlertsForACleanBenchmark(t *testing.T) {
	r := New(opts(1), nil, io.Discard)
	res := &Result{Times: []float64{1.00, 1.01, 0.99, 1.02}, ExitCodes: []int{0, 0, 0, 0}}
	res.Wall.Mean = 1.005
	if alerts := r.alerts(res); len(alerts) != 0 {
		t.Errorf("clean benchmark produced alerts: %+v", alerts)
	}
}

// recordingProgress captures the progress callbacks a benchmark makes.
type recordingProgress struct {
	starts   []int
	advances int
	finishes int
}

func (p *recordingProgress) Start(total int)                      { p.starts = append(p.starts, total) }
func (p *recordingProgress) Advance(done, total int, est float64) { p.advances++ }
func (p *recordingProgress) Finish()                              { p.finishes++ }

func TestProgressIsDriven(t *testing.T) {
	p := &recordingProgress{}
	r := New(opts(3), p, io.Discard)
	if _, err := r.Run(Job{Command: trivial()}); err != nil {
		t.Fatal(err)
	}
	if len(p.starts) == 0 || p.starts[0] != 3 {
		t.Errorf("starts = %v, want the run count up front", p.starts)
	}
	if p.advances != 3 {
		t.Errorf("advances = %d, want 3", p.advances)
	}
	if p.finishes != 1 {
		t.Errorf("finishes = %d, want 1", p.finishes)
	}
}

func hasAlert(r *Result, kind string) bool { return containsKind(r.Alerts, kind) }

func containsKind(alerts []Alert, kind string) bool {
	for _, a := range alerts {
		if a.Kind == kind {
			return true
		}
	}
	return false
}
