// Command mystrace is a small strace(1): it runs a command, or attaches to a
// PID, and prints each system call with its arguments and result.
//
// Tracing uses ptrace and so only works on Linux. Elsewhere the program builds
// but exits with an error.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"mystrace/internal/cli"
	"mystrace/internal/filter"
	"mystrace/internal/format"
	"mystrace/internal/summary"
	"mystrace/internal/syscalls"
	"mystrace/internal/tracer"
)

const usage = `Usage:
  mystrace [options] command [args...]
  mystrace [options] -p PID

Options:
  -p PID     Attach to and trace a running process (exclusive with command).
  -e EXPR    Filter syscalls: -e trace=open,read | -e open,read | -e trace=!close | -e trace=file
  -c         Print only a summary table of syscall counts/time/errors at exit.
  -o FILE    Write trace output to FILE instead of stderr.
  -s SIZE    Max printed string length before truncation (default 32).
  -f         (seam) Follow forks/clones - documented, not yet implemented.
  -h         Show this help.

Live tracing requires Linux.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mystrace: "+err.Error())
		fmt.Fprint(os.Stderr, "\n"+usage)
		return 2
	}
	if cfg.Help {
		fmt.Fprint(os.Stdout, usage)
		return 0
	}

	flt, err := filter.Parse(cfg.Exprs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mystrace: "+err.Error())
		return 2
	}

	out := os.Stderr
	if cfg.Output != "" {
		f, err := os.Create(cfg.Output)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mystrace: "+err.Error())
			return 1
		}
		defer f.Close()
		out = f
	}

	tr, err := tracer.New(*cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mystrace: "+err.Error())
		return 1
	}

	agg := summary.New()
	handler := makeHandler(cfg, flt, agg, out)

	code, err := tr.Run(handler)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mystrace: "+err.Error())
		return 1
	}

	if cfg.Count {
		fmt.Fprint(out, agg.Render())
	} else {
		fmt.Fprintln(out, format.ExitLine(code))
	}
	return code
}

func makeHandler(cfg *cli.Config, flt *filter.Filter, agg *summary.Aggregator, out io.Writer) func(tracer.RawCall) {
	return func(rc tracer.RawCall) {
		d := syscalls.Decode(rc, cfg.StrSize)
		if !flt.ShouldTrace(d.Name) {
			return
		}
		if cfg.Count {
			agg.Record(d.Name, durationOf(rc), syscalls.ErrnoFromRet(rc.HasRet, rc.Ret))
			return
		}
		fmt.Fprintln(out, format.Line(d.Name, d.Args, d.Ret))
	}
}

func durationOf(rc tracer.RawCall) time.Duration {
	return time.Duration(rc.DurNs) * time.Nanosecond
}
