// Command cctop is a small top clone: a full-screen process table that
// refreshes every few seconds.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/example/cctop/internal/engine"
	"github.com/example/cctop/internal/model"
	"github.com/example/cctop/internal/source"
	"github.com/example/cctop/internal/tui"
)

const version = "cctop 1.0.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("cctop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		delay      float64
		iterations int
		sortKey    string
		reverse    bool
		userFilter string
		memUnit    string
		demo       bool
		once       bool
		solaris    bool
		showVer    bool
	)
	fs.Float64Var(&delay, "d", 2.0, "refresh interval in seconds")
	fs.Float64Var(&delay, "delay", 2.0, "refresh interval in seconds")
	fs.IntVar(&iterations, "n", 0, "exit after N refreshes (0 = forever)")
	fs.IntVar(&iterations, "iterations", 0, "exit after N refreshes (0 = forever)")
	fs.StringVar(&sortKey, "s", "cpu", "initial sort: cpu|mem|pid|time|user|command")
	fs.StringVar(&sortKey, "sort", "cpu", "initial sort: cpu|mem|pid|time|user|command")
	fs.BoolVar(&reverse, "reverse", false, "start with ascending sort")
	fs.StringVar(&userFilter, "u", "", "only show processes owned by <name>")
	fs.StringVar(&userFilter, "user", "", "only show processes owned by <name>")
	fs.StringVar(&memUnit, "M", "auto", "memory unit: kib|mib|gib|auto")
	fs.StringVar(&memUnit, "mem-unit", "auto", "memory unit: kib|mib|gib|auto")
	fs.BoolVar(&demo, "demo", false, "run against the built-in fake source")
	fs.BoolVar(&once, "once", false, "print one plain-text frame and exit")
	fs.BoolVar(&solaris, "solaris", false, "divide %CPU by CPU count (Solaris mode)")
	fs.BoolVar(&showVer, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if showVer {
		fmt.Println(version)
		return 0
	}

	sk, ok := model.ParseSortKey(sortKey)
	if !ok {
		fmt.Fprintf(os.Stderr, "cctop: invalid sort key %q\n", sortKey)
		return 2
	}
	mu, ok := model.ParseMemUnit(memUnit)
	if !ok {
		fmt.Fprintf(os.Stderr, "cctop: invalid memory unit %q\n", memUnit)
		return 2
	}

	view := model.ViewState{
		Sort:       sk,
		Reverse:    reverse,
		Unit:       mu,
		UserFilter: userFilter,
		Solaris:    solaris,
	}
	interval := time.Duration(delay * float64(time.Second))
	if interval <= 0 {
		interval = 2 * time.Second
	}

	var src source.ProcessSource
	if demo {
		src = source.NewFake(interval)
	} else {
		src = source.NewGopsutil()
	}
	defer src.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if once {
		return runOnce(ctx, src, view, interval)
	}
	return runTUI(ctx, src, view, interval, iterations)
}

// runOnce prints a single frame as plain text. %CPU is a rate, so it needs two
// samples a short time apart.
func runOnce(ctx context.Context, src source.ProcessSource, view model.ViewState, interval time.Duration) int {
	prev, err := src.Sample(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cctop: %v\n", err)
		return 1
	}
	select {
	case <-time.After(min(interval, 500*time.Millisecond)):
	case <-ctx.Done():
		return 0
	}
	cur, err := src.Sample(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cctop: %v\n", err)
		return 1
	}
	snap := engine.Compute(prev, cur, view)
	for _, line := range tui.Frame(snap, interval.Seconds(), 100, 0) {
		fmt.Println(line)
	}
	return 0
}

func runTUI(ctx context.Context, src source.ProcessSource, view model.ViewState, interval time.Duration, iters int) int {
	app := tui.NewApp(src, view, interval, iters)
	if err := app.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "cctop: %v\n", err)
		return 1
	}
	return 0
}
