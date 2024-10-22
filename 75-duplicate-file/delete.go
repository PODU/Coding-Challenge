package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DeleteOptions controls DeleteDuplicates.
type DeleteOptions struct {
	DryRun  bool
	Keep    string // "shortest-path", "first-found" or "interactive"
	LogFile string
	Out     io.Writer // progress and prompts
	In      io.Reader // answers to prompts
}

// DeleteDuplicates keeps one file from each group and removes the rest. Hardlinks
// to the kept file are left alone, since removing them frees no space.
func DeleteDuplicates(r *Report, opts DeleteOptions) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}

	var logw io.Writer
	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
		logw = f
	}

	var reader *bufio.Reader
	if opts.In != nil {
		reader = bufio.NewReader(opts.In)
	}

	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		fmt.Fprintln(opts.Out, line)
		if logw != nil {
			fmt.Fprintf(logw, "%s %s\n", time.Now().Format(time.RFC3339), line)
		}
	}

	for _, g := range r.Groups {
		keeper := chooseKeeper(g, opts.Keep, opts.Out, reader)
		logf("keep   %s", keeper.Path)
		for _, m := range g.members {
			if m.Path == keeper.Path {
				continue
			}
			if m.Dev != 0 && m.Dev == keeper.Dev && m.Ino == keeper.Ino {
				logf("skip   %s (hardlink of keeper, frees no space)", m.Path)
				continue
			}
			if opts.DryRun {
				logf("would-delete %s", m.Path)
				continue
			}
			if err := os.Remove(m.Path); err != nil {
				logf("error  %s: %v", m.Path, err)
				continue
			}
			logf("delete %s", m.Path)
		}
	}
	return nil
}

func chooseKeeper(g DuplicateGroup, strategy string, out io.Writer, in *bufio.Reader) FileInfo {
	switch strategy {
	case "interactive":
		if in != nil {
			if k, ok := promptKeeper(g, out, in); ok {
				return k
			}
		}
		fallthrough // no usable answer
	case "shortest-path":
		return shortestPath(g.members)
	case "first-found":
		return firstFound(g.members)
	default:
		return shortestPath(g.members)
	}
}

func shortestPath(members []FileInfo) FileInfo {
	best := members[0]
	for _, m := range members[1:] {
		if len(m.Path) < len(best.Path) || (len(m.Path) == len(best.Path) && m.Path < best.Path) {
			best = m
		}
	}
	return best
}

func firstFound(members []FileInfo) FileInfo {
	best := members[0]
	for _, m := range members[1:] {
		if m.Path < best.Path {
			best = m
		}
	}
	return best
}

func promptKeeper(g DuplicateGroup, out io.Writer, in *bufio.Reader) (FileInfo, bool) {
	sorted := append([]FileInfo(nil), g.members...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	fmt.Fprintf(out, "Duplicate group (size %d). Choose the file to KEEP:\n", g.Size)
	for i, m := range sorted {
		fmt.Fprintf(out, "  [%d] %s\n", i+1, m.Path)
	}
	fmt.Fprint(out, "Enter number: ")
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return FileInfo{}, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(sorted) {
		return FileInfo{}, false
	}
	return sorted[n-1], true
}
