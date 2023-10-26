// Command crontab parses, validates, describes and schedules cron expressions.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"crontab/internal/clock"
	"crontab/internal/cron"
	"crontab/internal/job"
	"crontab/internal/scheduler"
)

const version = "1.0.0"

const usage = `crontab - parse, describe and schedule cron expressions

Usage:
  crontab validate "<expr>"
  crontab describe "<expr>"
  crontab next "<expr>" [--from <RFC3339>] [-n <count>] [--tz <zone>]
  crontab list [--file <path>]
  crontab add "<expr>" "<command>" [--file <path>]
  crontab run [--file <path>] [--tz <zone>] [--once]
  crontab version

Global:
  --file <path>   crontab file (default: ./crontab or $CRONTAB_FILE)

Exit codes: 0 success, 1 validation/parse error, 2 usage error, 3 runtime error.
`

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run runs a subcommand and returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "validate":
		return cmdValidate(rest, stdout, stderr)
	case "describe":
		return cmdDescribe(rest, stdout, stderr)
	case "next":
		return cmdNext(rest, stdout, stderr)
	case "list":
		return cmdList(rest, stdout, stderr)
	case "add":
		return cmdAdd(rest, stdout, stderr)
	case "run":
		return cmdRun(rest, stdout, stderr)
	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, `usage: crontab validate "<expr>"`)
		return 2
	}
	if _, err := cron.Parse(pos[0]); err != nil {
		fmt.Fprintf(stderr, "invalid: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "valid")
	return 0
}

func cmdDescribe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("describe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, `usage: crontab describe "<expr>"`)
		return 2
	}
	sch, err := cron.Parse(pos[0])
	if err != nil {
		fmt.Fprintf(stderr, "invalid: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, cron.Describe(sch))
	return 0
}

func cmdNext(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "", "reference time (RFC3339); default now")
	n := fs.Int("n", 1, "number of fire times to print")
	tz := fs.String("tz", "", "time zone (IANA name); default local")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, `usage: crontab next "<expr>" [--from <RFC3339>] [-n <count>] [--tz <zone>]`)
		return 2
	}

	loc, err := loadLocation(*tz)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	var ref time.Time
	if *from == "" {
		ref = time.Now().In(loc)
	} else {
		ref, err = time.Parse(time.RFC3339, *from)
		if err != nil {
			fmt.Fprintf(stderr, "invalid --from %q: %v\n", *from, err)
			return 2
		}
		// Without --tz, keep the offset written in --from.
		if *tz != "" {
			ref = ref.In(loc)
		}
	}

	sch, err := cron.Parse(pos[0])
	if err != nil {
		fmt.Fprintf(stderr, "invalid: %v\n", err)
		return 1
	}
	if *n < 1 {
		fmt.Fprintln(stderr, "-n must be >= 1")
		return 2
	}
	runs, err := sch.NextRuns(ref, *n)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	for _, t := range runs {
		fmt.Fprintln(stdout, t.Format(time.RFC3339))
	}
	return 0
}

func cmdList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "crontab file")
	tz := fs.String("tz", "", "time zone (IANA name); default local")
	if _, err := parseFlexible(fs, args); err != nil {
		return 2
	}
	loc, err := loadLocation(*tz)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	jobs, err := job.Load(crontabPath(*file))
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 3
	}
	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "no jobs")
		return 0
	}
	now := time.Now().In(loc)
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "EXPRESSION\tNEXT\tDESCRIPTION\tCOMMAND")
	for _, j := range jobs {
		next := "-"
		if t, err := j.Schedule.NextRun(now); err == nil {
			next = t.Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", j.Expr, next, cron.Describe(j.Schedule), j.Command)
	}
	tw.Flush()
	return 0
}

func cmdAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "crontab file")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 2 {
		fmt.Fprintln(stderr, `usage: crontab add "<expr>" "<command>" [--file <path>]`)
		return 2
	}
	j, err := job.Add(crontabPath(*file), pos[0], pos[1])
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "added: %s\t%s   (%s)\n", j.Expr, j.Command, cron.Describe(j.Schedule))
	return 0
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "crontab file")
	tz := fs.String("tz", "", "time zone (IANA name); default local")
	once := fs.Bool("once", false, "run a single tick then exit")
	if _, err := parseFlexible(fs, args); err != nil {
		return 2
	}
	loc, err := loadLocation(*tz)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	jobs, err := job.Load(crontabPath(*file))
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 3
	}
	if len(jobs) == 0 {
		fmt.Fprintln(stderr, "no jobs to run")
		return 0
	}

	sch := scheduler.New(jobs, scheduler.Options{
		Clock:  clock.NewReal(loc),
		Runner: scheduler.ExecRunner{},
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stdout, "crontab: scheduling %d job(s); press Ctrl-C to stop\n", len(jobs))
	if *once {
		if err := sch.RunOnce(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 3
		}
		return 0
	}
	if err := sch.Run(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 3
	}
	return 0
}

// parseFlexible lets flags appear anywhere among the operands; the flag
// package on its own stops at the first non-flag. It returns the operands.
func parseFlexible(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	return positional, nil
}

// crontabPath picks the crontab file: --file, then $CRONTAB_FILE, then
// ./crontab.
func crontabPath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("CRONTAB_FILE"); env != "" {
		return env
	}
	return "crontab"
}

func loadLocation(tz string) (*time.Location, error) {
	if tz == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid --tz %q: %v", tz, err)
	}
	return loc, nil
}
