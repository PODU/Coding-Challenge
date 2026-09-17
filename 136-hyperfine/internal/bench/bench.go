// Package bench owns the measurement loop: warmup, hooks, run-count decisions,
// shell-overhead correction and the warnings that come out of the numbers.
package bench

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"cchyperfine/internal/cli"
	"cchyperfine/internal/exec"
	"cchyperfine/internal/stats"
)

// Job is one fully-resolved benchmark: a concrete command line plus the hooks
// and metadata that belong to it.
type Job struct {
	Index      int    // 0-based position, used for "Benchmark N:"
	Command    string // after parameter substitution
	Name       string // display name (defaults to Command)
	ParamNames []string
	Parameters map[string]string
	Prepare    string
	Conclude   string
}

// Result is everything measured and inferred for one Job.
type Result struct {
	Job
	Times     []float64 // wall-clock seconds, shell overhead removed
	User      []float64
	System    []float64
	ExitCodes []int

	Wall   stats.Summary
	UserT  stats.Summary
	SysT   stats.Summary
	Alerts []Alert
}

// Alert is a diagnostic emitted after a benchmark completes.
type Alert struct {
	Kind    string
	Message string
}

// Progress is the subset of terminal behaviour bench needs. A nil Progress is
// fine; Runner guards every call.
type Progress interface {
	Start(total int)
	Advance(done, total int, estimate float64)
	Finish()
}

// Runner executes Jobs according to the parsed options.
type Runner struct {
	Opts     *cli.Options
	Progress Progress
	Stderr   io.Writer

	shellOverhead     float64
	shellOverheadSD   float64
	shellOverheadUser float64
	shellOverheadSys  float64
	shellCalibration  bool
}

// New builds a Runner.
func New(o *cli.Options, p Progress, stderr io.Writer) *Runner {
	return &Runner{Opts: o, Progress: p, Stderr: stderr}
}

// ShellOverhead reports the mean measured cost of spawning the shell, in
// seconds. It is zero until the first benchmark has run, and always zero when
// running without a shell.
func (r *Runner) ShellOverhead() float64 { return r.shellOverhead }

// Run measures a single job.
func (r *Runner) Run(j Job) (*Result, error) {
	o := r.Opts

	if err := r.calibrateShell(); err != nil {
		return nil, err
	}
	if o.Setup != "" {
		if err := r.hook("setup", o.Setup, 0); err != nil {
			return nil, err
		}
	}

	for k := 0; k < o.Warmup; k++ {
		if j.Prepare != "" {
			if err := r.hook("prepare", j.Prepare, k); err != nil {
				return nil, err
			}
		}
		if _, err := r.once(j, k); err != nil {
			return nil, err
		}
		if j.Conclude != "" {
			if err := r.hook("conclude", j.Conclude, k); err != nil {
				return nil, err
			}
		}
	}

	res := &Result{Job: j}
	target := o.Runs // 0 until we can estimate it
	if target > 0 {
		r.progressStart(target)
	} else {
		r.progressStart(o.MinRuns)
	}

	var elapsed float64
	for k := 0; ; k++ {
		if j.Prepare != "" {
			if err := r.hook("prepare", j.Prepare, k); err != nil {
				r.progressFinish()
				return nil, err
			}
		}
		t, err := r.once(j, k)
		if err != nil {
			r.progressFinish()
			return nil, err
		}
		if j.Conclude != "" {
			if err := r.hook("conclude", j.Conclude, k); err != nil {
				r.progressFinish()
				return nil, err
			}
		}

		if t.ExitCode != 0 && !o.IgnoreFail {
			r.progressFinish()
			return nil, fmt.Errorf("command terminated with non-zero exit code %d: %s\n"+
				"Use the '-i'/'--ignore-failure' option if you want to ignore this", t.ExitCode, j.Command)
		}

		corrected := r.correct(t)
		res.Times = append(res.Times, corrected.Real)
		res.User = append(res.User, corrected.User)
		res.System = append(res.System, corrected.System)
		res.ExitCodes = append(res.ExitCodes, t.ExitCode)
		elapsed += t.Real

		if target == 0 {
			target = r.estimateRuns(corrected.Real)
			r.progressStart(target)
		}
		r.progressAdvance(len(res.Times), target, elapsed/float64(len(res.Times))*float64(target))

		if len(res.Times) >= target {
			break
		}
	}
	r.progressFinish()

	if o.Cleanup != "" {
		if err := r.hook("cleanup", o.Cleanup, 0); err != nil {
			return nil, err
		}
	}

	res.Wall = stats.Summarize(res.Times)
	res.UserT = stats.Summarize(res.User)
	res.SysT = stats.Summarize(res.System)
	res.Alerts = r.alerts(res)
	return res, nil
}

// estimateRuns picks a run count from a single observed timing: enough runs to
// spend MinBenchmarkTimeSeconds measuring, clamped into [--min-runs, --max-runs].
func (r *Runner) estimateRuns(sample float64) int {
	o := r.Opts
	n := o.MinRuns
	if sample > 0 {
		n = int(math.Ceil(cli.MinBenchmarkTimeSeconds / sample))
	}
	if n < o.MinRuns {
		n = o.MinRuns
	}
	if o.MaxRuns > 0 && n > o.MaxRuns {
		n = o.MaxRuns
	}
	if n < 1 {
		n = 1
	}
	return n
}

// once executes the benchmarked command for iteration k.
func (r *Runner) once(j Job, k int) (exec.Timing, error) {
	streams, closer, err := r.streams()
	if err != nil {
		return exec.Timing{}, err
	}
	defer closer()
	return exec.Run(r.Opts.Shell, j.Command, iterationEnv(k), streams)
}

// hook runs setup/prepare/conclude/cleanup. Hooks are never timed, and a
// non-zero exit from one aborts the benchmark regardless of --ignore-failure:
// a failed prepare means the timings that follow are meaningless.
func (r *Runner) hook(kind, command string, k int) error {
	streams, closer, err := r.streams()
	if err != nil {
		return err
	}
	defer closer()
	t, err := exec.Run(r.Opts.Shell, command, iterationEnv(k), streams)
	if err != nil {
		return fmt.Errorf("%s command failed: %w", kind, err)
	}
	if t.ExitCode != 0 {
		return fmt.Errorf("%s command terminated with non-zero exit code %d: %s", kind, t.ExitCode, command)
	}
	return nil
}

// iterationEnv exposes the current iteration to the command, matching
// hyperfine's $HYPERFINE_ITERATION.
func iterationEnv(k int) []string {
	return append(os.Environ(), "HYPERFINE_ITERATION="+strconv.Itoa(k))
}

// streams wires up stdin/stdout/stderr for one execution and returns a closer
// for any files it opened.
func (r *Runner) streams() (exec.Streams, func(), error) {
	o := r.Opts
	var s exec.Streams
	var toClose []io.Closer
	closer := func() {
		for _, c := range toClose {
			_ = c.Close()
		}
	}

	switch o.OutputMode {
	case cli.OutputInherit:
		s.Stdout, s.Stderr = os.Stdout, os.Stderr
	case cli.OutputFile:
		f, err := os.OpenFile(o.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return s, closer, fmt.Errorf("--output: %w", err)
		}
		toClose = append(toClose, f)
		s.Stdout, s.Stderr = f, f
	default: // null / pipe: discarded either way
	}

	if o.InputPath != "" {
		f, err := os.Open(o.InputPath)
		if err != nil {
			return s, closer, fmt.Errorf("--input: %w", err)
		}
		toClose = append(toClose, f)
		s.Stdin = f
	}
	return s, closer, nil
}

// calibrateShell measures how long it takes the configured shell to start and
// exit, so that cost can be removed from every later measurement.
func (r *Runner) calibrateShell() error {
	if r.shellCalibration || !r.Opts.Shell.Enabled() {
		return nil
	}
	r.shellCalibration = true

	var real, user, sys []float64
	for i := 0; i < cli.ShellCalibrationRuns; i++ {
		t, err := exec.Run(r.Opts.Shell, "", nil, exec.Discard)
		if err != nil {
			return fmt.Errorf("could not measure shell startup time: %w", err)
		}
		real = append(real, t.Real)
		user = append(user, t.User)
		sys = append(sys, t.System)
	}
	r.shellOverhead = stats.Mean(real)
	r.shellOverheadSD = stats.StdDev(real)
	r.shellOverheadUser = stats.Mean(user)
	r.shellOverheadSys = stats.Mean(sys)
	return nil
}

// correct removes the shell startup cost from a raw timing. Clamping at zero
// keeps a command that is faster than the shell's own noise from reporting a
// negative duration.
func (r *Runner) correct(t exec.Timing) exec.Timing {
	if r.shellOverhead == 0 {
		return t
	}
	return exec.Timing{
		Real:     math.Max(0, t.Real-r.shellOverhead),
		User:     math.Max(0, t.User-r.shellOverheadUser),
		System:   math.Max(0, t.System-r.shellOverheadSys),
		ExitCode: t.ExitCode,
	}
}

func (r *Runner) progressStart(total int) {
	if r.Progress != nil {
		r.Progress.Start(total)
	}
}

func (r *Runner) progressAdvance(done, total int, est float64) {
	if r.Progress != nil {
		r.Progress.Advance(done, total, est)
	}
}

func (r *Runner) progressFinish() {
	if r.Progress != nil {
		r.Progress.Finish()
	}
}
