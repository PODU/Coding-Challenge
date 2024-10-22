// Command dupfind finds duplicate files under a set of directories and can
// delete the extra copies. Files are compared by size, then a hash of the
// first few KB, then a full hash, then byte by byte.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dupfind", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr, fs) }

	minSizeStr := fs.String("min-size", "1", "ignore files smaller than this (suffixes: k,M,G)")
	algo := fs.String("algo", "md5", "hash algorithm: md5 | sha256")
	format := fs.String("format", "text", "output format: text | json")
	follow := fs.Bool("follow-symlinks", false, "follow symbolic links during the walk")
	del := fs.Bool("delete", false, "delete duplicates, keeping one per group")
	dryRun := fs.Bool("dry-run", false, "with --delete, show what would be deleted")
	keep := fs.String("keep", "shortest-path", "which file to keep: shortest-path | first-found | interactive")
	logFile := fs.String("log", "", "append all delete actions to this file")
	quiet := fs.Bool("quiet", false, "suppress the per-group report")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	roots := fs.Args()
	if len(roots) == 0 {
		fmt.Fprintln(stderr, "error: at least one PATH is required")
		usage(stderr, fs)
		return 2
	}

	minSize, err := parseSize(*minSizeStr)
	if err != nil {
		fmt.Fprintf(stderr, "error: --min-size: %v\n", err)
		return 2
	}
	if *algo != "md5" && *algo != "sha256" {
		fmt.Fprintf(stderr, "error: --algo must be md5 or sha256\n")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "error: --format must be text or json\n")
		return 2
	}
	switch *keep {
	case "shortest-path", "first-found", "interactive":
	default:
		fmt.Fprintf(stderr, "error: --keep must be shortest-path, first-found, or interactive\n")
		return 2
	}

	if countReadableDirs(roots) == 0 {
		fmt.Fprintln(stderr, "error: no readable directory among the given paths")
		return 3
	}

	opts := Options{MinSize: minSize, Algo: *algo, FollowSymlinks: *follow}
	report, _, err := FindDuplicates(roots, opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 3
	}

	if *del {
		delOpts := DeleteOptions{
			DryRun:  *dryRun,
			Keep:    *keep,
			LogFile: *logFile,
			Out:     stdout,
			In:      os.Stdin,
		}
		if err := DeleteDuplicates(report, delOpts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 3
		}
	}

	if !*quiet {
		if *format == "json" {
			if err := writeJSON(stdout, report); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 3
			}
		} else {
			writeText(stdout, report)
		}
	}

	if len(report.Groups) > 0 {
		return 1
	}
	return 0
}

func countReadableDirs(roots []string) int {
	n := 0
	for _, r := range roots {
		info, err := os.Stat(r)
		if err != nil {
			continue
		}
		if info.IsDir() || info.Mode().IsRegular() {
			n++
		}
	}
	return n
}

func usage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Usage: dupfind [flags] PATH [PATH...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Recursively find content-identical duplicate files.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fs.PrintDefaults()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 no dupes, 1 dupes found, 2 usage error, 3 runtime error.")
}
