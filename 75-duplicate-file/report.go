package main

import (
	"encoding/json"
	"fmt"
	"io"
)

func writeJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// writeText prints the groups fdupes-style, separated by blank lines, then a
// summary line.
func writeText(w io.Writer, r *Report) {
	for _, g := range r.Groups {
		if g.Size == 0 {
			fmt.Fprintf(w, "Duplicates (size 0, empty):\n")
		} else {
			fmt.Fprintf(w, "Duplicates (size %d, %s):\n", g.Size, shortHash(g.Hash))
		}
		for _, p := range g.Paths {
			fmt.Fprintf(w, "  %s\n", p)
		}
		fmt.Fprintln(w)
	}

	redundant := r.Summary.Files - r.Summary.Groups
	fmt.Fprintf(w, "Summary: %d duplicate %s, %d redundant %s, %s reclaimable.",
		r.Summary.Groups, plural(r.Summary.Groups, "group", "groups"),
		redundant, plural(redundant, "file", "files"),
		humanBytes(r.Summary.WastedBytes))
	if n := len(r.Errors); n > 0 {
		fmt.Fprintf(w, " %d %s.", n, plural(n, "error", "errors"))
	}
	fmt.Fprintln(w)
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12] + "..."
	}
	return h
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
