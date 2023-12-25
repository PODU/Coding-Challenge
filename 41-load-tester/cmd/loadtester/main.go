// Command loadtester is a small HTTP(S) load-testing tool: it fires N requests
// at a target URL (or a cycled file of URLs) using C concurrent workers,
// captures per-request timing (total, TTFB, TTLB), and prints a statistical
// summary.
//
// Usage:
//
//	loadtester -u <url> [-n N] [-c C] [-m METHOD] [-H 'K: V'] [-d body] [-t 5s] [-json]
//	loadtester -f <file> [...]
//
// Exit codes: 0 = ran to completion; 1 = invalid configuration; 2 = all
// requests failed.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"loadtester/internal/config"
	"loadtester/internal/report"
	"loadtester/internal/runner"
	"loadtester/internal/stats"
	"loadtester/internal/urls"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "loadtester: %v\n", err)
		fmt.Fprintf(stderr, "usage: loadtester -u <url> [-n N] [-c C] [-m METHOD] [-H 'K: V'] [-d body] [-t 5s] [-json]\n")
		return 1
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(stderr, "loadtester: warning: %s\n", w)
	}

	var provider runner.Provider
	if cfg.File != "" {
		p, err := urls.FromFile(cfg.File, cfg.N)
		if err != nil {
			fmt.Fprintf(stderr, "loadtester: %v\n", err)
			return 1
		}
		provider = p
	} else {
		provider = urls.Single(cfg.URL, cfg.N)
	}

	// On Ctrl-C, in-flight requests finish and a partial summary is printed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	req := runner.NewHTTPRequester(cfg.Method, cfg.Headers, cfg.Body, cfg.Timeout, cfg.C)

	collector, wall := runner.Run(ctx, provider, cfg.C, req)
	summary := collector.Summarize(wall)

	if err := writeReport(stdout, summary, cfg.JSON); err != nil {
		fmt.Fprintf(stderr, "loadtester: writing report: %v\n", err)
		return 1
	}

	if summary.Total > 0 && summary.Success == 0 {
		return 2
	}
	return 0
}

func writeReport(out *os.File, s stats.Summary, asJSON bool) error {
	if asJSON {
		return report.JSON(out, s)
	}
	return report.Table(out, s)
}
