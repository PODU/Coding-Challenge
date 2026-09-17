// Command cchyperfine is a command-line benchmarking tool in the style of
// hyperfine: it runs commands repeatedly, corrects for shell startup cost, and
// reports statistics along with warnings about when not to trust them.
//
// Coding Challenge #136 -- https://codingchallenges.substack.com/p/coding-challenge-136-hyperfine
package main

import (
	"fmt"
	"os"

	"cchyperfine/internal/bench"
	"cchyperfine/internal/cli"
	"cchyperfine/internal/export"
	"cchyperfine/internal/param"
	"cchyperfine/internal/report"
)

const version = "cchyperfine 1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	opts, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if opts.ShowHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if opts.ShowVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	style := report.StyleFor(opts.Style, stderr)
	unit, forced := report.ParseUnit(opts.TimeUnit)
	rep := &report.Reporter{Out: stdout, Style: style, Unit: unit, UnitForced: forced}

	jobs, err := buildJobs(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	bar := report.NewBar(stderr, style)
	runner := bench.New(opts, bar, stderr)

	results := make([]*bench.Result, 0, len(jobs))
	for _, j := range jobs {
		rep.Header(j.Index, j.Name)
		res, err := runner.Run(j)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		rep.Result(res)
		results = append(results, res)
	}

	sorted := report.SortResults(results, opts.SortMethod)
	rep.Summary(sorted, opts.Reference)

	w := export.Writer{
		JSON:     opts.ExportJSON,
		CSV:      opts.ExportCSV,
		Markdown: opts.ExportMarkdown,
		Asciidoc: opts.ExportAsciidoc,
		Orgmode:  opts.ExportOrgmode,
	}
	if err := w.WriteAll(sorted); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// buildJobs turns the parsed options into the concrete list of benchmarks:
// every command crossed with every point in the parameter space, with the
// `--reference` command (if any) benchmarked first so it is measured under the
// same conditions as the commands it is compared against.
func buildJobs(o *cli.Options) ([]bench.Job, error) {
	bindings, err := param.Expand(o.Scans, o.Lists)
	if err != nil {
		return nil, err
	}

	type template struct {
		command  string
		name     string
		prepare  string
		conclude string
	}
	var templates []template

	if o.Reference != "" {
		templates = append(templates, template{
			command:  o.Reference,
			prepare:  o.PrepareFor(0),
			conclude: o.ConcludeFor(0),
		})
	}
	for i, c := range o.Commands {
		name := ""
		if i < len(o.Names) {
			name = o.Names[i]
		}
		templates = append(templates, template{
			command:  c,
			name:     name,
			prepare:  o.PrepareFor(i),
			conclude: o.ConcludeFor(i),
		})
	}

	var jobs []bench.Job
	for _, t := range templates {
		for _, b := range bindings {
			command := b.Substitute(t.command)
			name := command
			if t.name != "" {
				name = b.Substitute(t.name)
			}
			jobs = append(jobs, bench.Job{
				Index:      len(jobs),
				Command:    command,
				Name:       name,
				ParamNames: b.Names,
				Parameters: copyParams(b.Values),
				Prepare:    b.Substitute(t.prepare),
				Conclude:   b.Substitute(t.conclude),
			})
		}
	}
	return jobs, nil
}

func copyParams(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

const usage = `cchyperfine -- a command-line benchmarking tool

USAGE:
    cchyperfine [OPTIONS] <command>...

ARGS:
    <command>...    The command(s) to benchmark. Each is run through a shell
                    unless -N/--shell=none is given.

BENCHMARK CONTROL:
    -r, --runs <NUM>            Exact number of timed runs. Without it, the run
                                count is chosen so the benchmark takes about 3 s.
    -m, --min-runs <NUM>        Lower bound on the automatic run count [10].
    -M, --max-runs <NUM>        Upper bound on the automatic run count.
    -w, --warmup <NUM>          Untimed runs performed before measuring.

HOOKS (never included in the measurement):
    -s, --setup <CMD>           Run once before all runs of a command.
    -p, --prepare <CMD>         Run before each timed run. Repeat once per command.
    -C, --conclude <CMD>        Run after each timed run. Repeat once per command.
    -c, --cleanup <CMD>         Run once after all runs of a command.

EXECUTION:
    -S, --shell <SHELL>         Shell to use, or "none" to execute directly.
    -N                          Shorthand for --shell=none.
    -i, --ignore-failure        Do not abort when a command exits non-zero.
        --input <WHERE>         "null", or a file to feed to the command's stdin.
        --output <WHERE>        "null", "pipe", "inherit", or a file path.
        --show-output           Shorthand for --output=inherit.

COMPARISON:
    -n, --command-name <NAME>   Display name for the n-th command.
        --reference <CMD>       Benchmark CMD and compare everything against it.
        --sort <METHOD>         auto | command | mean-time [auto].

PARAMETERS:
    -P, --parameter-scan <VAR> <MIN> <MAX>   Sweep {VAR} over a numeric range.
    -D, --parameter-step-size <DELTA>        Step for --parameter-scan [1].
    -L, --parameter-list <VAR> <VALUES>      Sweep {VAR} over a comma-separated list.

OUTPUT:
    -u, --time-unit <UNIT>      microsecond | millisecond | second.
        --style <TYPE>          auto | basic | full | nocolor | color | none.
        --export-json <FILE>
        --export-csv <FILE>
        --export-markdown <FILE>
        --export-asciidoc <FILE>
        --export-orgmode <FILE>

    -h, --help                  Print this help.
    -V, --version               Print version information.

The environment variable $HYPERFINE_ITERATION is set to the zero-based index of
the current run for the benchmarked command and for every hook.

EXAMPLES:
    cchyperfine 'sleep 0.3'
    cchyperfine -w 3 'grep -R TODO .' 'rg TODO'
    cchyperfine -N -P threads 1 8 -D 2 'build --jobs {threads}'
    cchyperfine --export-json results.json 'cmd a' 'cmd b'
`
