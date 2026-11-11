// Command ccdisk is a disk space analyser: it scans a directory tree, shows
// it as a treemap in the terminal, points out large, old and reclaimable
// items, and moves what you choose to the Trash after a review.
// Coding Challenge #145.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"ccdisk/internal/analysis"
	"ccdisk/internal/cleanup"
	"ccdisk/internal/scan"
	"ccdisk/internal/tui"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ccdisk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	report := fs.Bool("report", false, "print a text report instead of the interactive treemap")
	apparent := fs.Bool("apparent", false, "show apparent size instead of disk usage")
	oneFS := fs.Bool("x", false, "stay on one filesystem")
	workers := fs.Int("workers", 0, "directories to read at once (default 4 per CPU)")
	depth := fs.Int("depth", 2, "report: tree depth")
	top := fs.Int("top", 10, "report: number of largest files")
	minSize := fs.String("min-size", "0", "report: only files at least this big (e.g. 10M, 1G)")
	minAge := fs.String("min-age", "0", "report: only files unmodified this long (e.g. 30d, 6mo, 1y)")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: ccdisk [flags] [DIR]\n\nScan DIR (default: the current directory) and browse it as a treemap.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "ccdisk: only one directory can be scanned at a time")
		return 2
	}
	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	ms, err := parseSize(*minSize)
	if err != nil {
		fmt.Fprintf(stderr, "ccdisk: -min-size: %v\n", err)
		return 2
	}
	ma, err := parseAge(*minAge)
	if err != nil {
		fmt.Fprintf(stderr, "ccdisk: -min-age: %v\n", err)
		return 2
	}
	measure := scan.DiskUsage
	if *apparent {
		measure = scan.ApparentSize
	}
	opts := scan.Options{Workers: *workers, OneFS: *oneFS}

	if *report {
		t0 := time.Now()
		n, warns, err := scan.Scan(context.Background(), root, opts)
		if err != nil {
			fmt.Fprintf(stderr, "ccdisk: %v\n", err)
			return 1
		}
		writeReport(stdout, n, warns, time.Since(t0), reportOpts{
			measure: measure, depth: *depth, top: *top,
			filter: analysis.Filter{MinSize: ms, MinAge: ma, Measure: measure, Now: time.Now()},
		})
		return 0
	}

	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		fmt.Fprintf(stderr, "ccdisk: %s: %v\n", root, err)
		return 1
	}
	a := tui.New(root, opts, cleanup.NewTrasher())
	a.Measure = measure
	if err := tui.Run(a, root, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(stderr, "ccdisk: %v\n", err)
		return 1
	}
	if a.Root == nil && a.Status != "" {
		fmt.Fprintf(stderr, "ccdisk: %s\n", a.Status)
		return 1
	}
	return 0
}

// parseSize accepts plain bytes or a K/M/G/T suffix (powers of 1024).
func parseSize(s string) (int64, error) {
	s = strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(s)), "B")
	mult := int64(1)
	if s != "" {
		if i := strings.IndexByte("KMGT", s[len(s)-1]); i >= 0 {
			mult = int64(1) << (10 * (i + 1))
			s = s[:len(s)-1]
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return int64(f * float64(mult)), nil
}

// parseAge accepts d (days), w, mo (30 days), y (365 days), or a Go
// duration such as 12h.
func parseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "0" || s == "" {
		return 0, nil
	}
	for _, u := range []struct {
		suffix string
		d      time.Duration
	}{{"mo", analysis.Month}, {"y", analysis.Year}, {"w", 7 * 24 * time.Hour}, {"d", 24 * time.Hour}} {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.Atoi(strings.TrimSuffix(s, u.suffix))
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid age %q", s)
			}
			return time.Duration(n) * u.d, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid age %q", s)
	}
	return d, nil
}
