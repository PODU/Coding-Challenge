// Command loc counts lines of code, like cloc or tokei: each line of each
// source file is counted as blank, comment or code, grouped by language.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"sync"

	"loc/internal/count"
	"loc/internal/report"
	"loc/internal/walk"
)

const version = "loc 1.0.0"

// stringSlice collects a flag that can be given more than once.
type stringSlice []string

func (s *stringSlice) String() string { return fmt.Sprintf("%v", []string(*s)) }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loc", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		byFile         = fs.Bool("by-file", false, "per-file breakdown instead of per-language summary")
		filesOnly      = fs.Bool("files", false, "just list discovered files and exit")
		asJSON         = fs.Bool("json", false, "emit JSON instead of the text table")
		sortKey        = fs.String("sort", "code", "sort key: code|lines|comments|blanks|name")
		includeUnknown = fs.Bool("include-unknown", false, "count files with unrecognized extensions as (unknown)")
		noIgnore       = fs.Bool("no-ignore", false, "do not skip .git/node_modules/vendor/etc.")
		cocomo         = fs.Bool("cocomo", false, "append a COCOMO organic-model estimate")
		avgWage        = fs.Float64("avg-wage", 56286, "annual wage for COCOMO cost")
		threads        = fs.Int("threads", runtime.GOMAXPROCS(0), "worker count")
		showVersion    = fs.Bool("version", false, "print version and exit")
		excludes       stringSlice
		langs          stringSlice
	)
	fs.Var(&excludes, "exclude", "skip paths matching glob (repeatable)")
	fs.Var(&langs, "lang", "restrict to the named language(s) (repeatable)")

	if err := fs.Parse(args); err != nil {
		return 1 // usage error; flag already printed the message
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	roots := fs.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	jobs, err := walk.Walk(roots, walk.Options{
		NoIgnore:       *noIgnore,
		IncludeUnknown: *includeUnknown,
		Excludes:       excludes,
		Langs:          langs,
	})
	if err != nil {
		fmt.Fprintln(stderr, "loc:", err)
		return 2
	}

	if *filesOnly {
		paths := make([]string, len(jobs))
		for i, j := range jobs {
			paths[i] = j.Path
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintln(stdout, p)
		}
		return 0
	}

	files := countAll(jobs, *threads, stderr)

	rep := report.Aggregate(files, *sortKey)

	if *asJSON {
		if err := report.RenderJSON(stdout, rep, *byFile); err != nil {
			fmt.Fprintln(stderr, "loc:", err)
			return 1
		}
	} else {
		report.RenderText(stdout, rep, *byFile)
		if *cocomo {
			fmt.Fprintln(stdout)
			report.RenderCocomo(stdout, report.EstimateCocomo(rep.Total.Code, *avgWage), *avgWage)
		}
	}
	return 0
}

// countAll counts the files with n workers. Results are sorted by path so the
// output doesn't depend on scheduling.
func countAll(jobs []walk.Job, n int, stderr io.Writer) []count.FileCount {
	if n < 1 {
		n = 1
	}
	jobCh := make(chan walk.Job)
	resCh := make(chan count.FileCount)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				fc, err := count.CountFile(j.Path, j.Def)
				if err != nil {
					fmt.Fprintln(stderr, "loc:", err)
					continue
				}
				resCh <- fc
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			jobCh <- j
		}
		close(jobCh)
	}()
	go func() {
		wg.Wait()
		close(resCh)
	}()

	var files []count.FileCount
	for fc := range resCh {
		files = append(files, fc)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}
