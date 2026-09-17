// Package cli parses the command line into a validated Options value.
package cli

import (
	"fmt"
	"strings"

	"cchyperfine/internal/exec"
	"cchyperfine/internal/param"
)

// Default tuning constants. They mirror hyperfine's so that results from the
// two tools are comparable.
const (
	DefaultMinRuns          = 10
	MinBenchmarkTimeSeconds = 3.0
	ShellCalibrationRuns    = 50
	MinAccurateTimeSeconds  = 0.005
)

// OutputMode says what happens to a benchmarked command's stdout/stderr.
type OutputMode int

const (
	OutputNull    OutputMode = iota // discard (the default)
	OutputPipe                      // capture but discard; identical observable effect, kept for parity
	OutputInherit                   // forward to our own terminal
	OutputFile                      // redirect to a file
)

// Options is the fully validated configuration of one hyperfine invocation.
type Options struct {
	Commands []string // raw command templates, before parameter substitution
	Names    []string // --command-name, positionally matched to Commands

	Runs    int // explicit --runs; 0 means "decide automatically"
	MinRuns int
	MaxRuns int // 0 means unbounded
	Warmup  int

	Setup    string
	Prepare  []string // one per command, or a single entry applied to all
	Conclude []string
	Cleanup  string

	Shell       exec.Shell
	ShellGiven  bool
	TimeUnit    string // "", "microsecond", "millisecond", "second"
	IgnoreFail  bool
	Reference   string
	SortMethod  string // "auto", "command", "mean-time"
	Style       string // "auto", "basic", "full", "nocolor", "color", "none"
	ShowOutput  bool
	OutputMode  OutputMode
	OutputFile  string
	InputPath   string // "" (none), "null", or a file path
	InputIsNull bool

	Scans []param.Scan
	Lists []param.List
	Step  string

	ExportJSON     string
	ExportCSV      string
	ExportMarkdown string
	ExportAsciidoc string
	ExportOrgmode  string

	ShowHelp    bool
	ShowVersion bool
}

func newOptions() *Options {
	return &Options{
		MinRuns:    DefaultMinRuns,
		Shell:      exec.DefaultShell(),
		SortMethod: "auto",
		Style:      "auto",
		OutputMode: OutputNull,
	}
}

// PrepareFor returns the --prepare command that applies to command index i.
// A single --prepare applies to every command; otherwise they pair up by
// position.
func (o *Options) PrepareFor(i int) string { return pick(o.Prepare, i) }

// ConcludeFor returns the --conclude command that applies to command index i.
func (o *Options) ConcludeFor(i int) string { return pick(o.Conclude, i) }

func pick(xs []string, i int) string {
	switch {
	case len(xs) == 0:
		return ""
	case len(xs) == 1:
		return xs[0]
	case i < len(xs):
		return xs[i]
	default:
		return ""
	}
}

func (o *Options) validate() error {
	if o.ShowHelp || o.ShowVersion {
		return nil
	}
	if len(o.Commands) == 0 {
		return fmt.Errorf("no commands given (try --help)")
	}
	if o.Runs < 0 || o.MinRuns < 1 || o.MaxRuns < 0 || o.Warmup < 0 {
		return fmt.Errorf("run counts must be non-negative")
	}
	if o.MaxRuns > 0 && o.MaxRuns < o.MinRuns {
		return fmt.Errorf("--max-runs (%d) must not be smaller than --min-runs (%d)", o.MaxRuns, o.MinRuns)
	}
	if len(o.Names) > len(o.Commands) {
		return fmt.Errorf("got %d --command-name values but only %d commands", len(o.Names), len(o.Commands))
	}
	if len(o.Prepare) > 1 && len(o.Prepare) != len(o.Commands) {
		return fmt.Errorf("--prepare must be given once, or once per command (%d)", len(o.Commands))
	}
	if len(o.Conclude) > 1 && len(o.Conclude) != len(o.Commands) {
		return fmt.Errorf("--conclude must be given once, or once per command (%d)", len(o.Commands))
	}
	switch o.TimeUnit {
	case "", "microsecond", "millisecond", "second":
	default:
		return fmt.Errorf("--time-unit: unknown unit %q (microsecond, millisecond, second)", o.TimeUnit)
	}
	switch o.SortMethod {
	case "auto", "command", "mean-time":
	default:
		return fmt.Errorf("--sort: unknown method %q (auto, command, mean-time)", o.SortMethod)
	}
	switch o.Style {
	case "auto", "basic", "full", "nocolor", "color", "none":
	default:
		return fmt.Errorf("--style: unknown style %q (auto, basic, full, nocolor, color, none)", o.Style)
	}
	if o.Reference != "" {
		if !o.Shell.Enabled() && strings.TrimSpace(o.Reference) == "" {
			return fmt.Errorf("--reference: empty command")
		}
	}
	if o.Step != "" && len(o.Scans) == 0 {
		return fmt.Errorf("--parameter-step-size requires --parameter-scan")
	}
	if len(o.Scans) > 0 {
		o.Scans[0].Step = o.Step
	}
	return nil
}
