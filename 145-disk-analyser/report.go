package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"ccdisk/internal/analysis"
	"ccdisk/internal/scan"
)

type reportOpts struct {
	measure scan.Measure
	depth   int
	top     int
	filter  analysis.Filter
}

// writeReport prints the same analysis as the UI as plain text, for scripts
// and for checking the numbers against du, ls and find.
func writeReport(w io.Writer, root *scan.Node, warns []scan.Warning, elapsed time.Duration, o reportOpts) {
	m := o.measure
	hs := analysis.HumanSize
	fmt.Fprintf(w, "ccdisk report for %s\n", root.Path())
	fmt.Fprintf(w, "Scanned %d items in %s", root.Items+1, elapsed.Round(time.Millisecond))
	if len(warns) > 0 {
		fmt.Fprintf(w, " (%d warning(s))", len(warns))
	}
	fmt.Fprintln(w)
	for _, wn := range warns {
		fmt.Fprintf(w, "warning: %s\n", wn)
	}
	fmt.Fprintf(w, "\nTotal: %s disk usage, %s apparent size, %d files\n", hs(root.TotalUsage), hs(root.TotalApparent), root.Files)
	if free, total, err := scan.FreeSpace(root.Path()); err == nil {
		fmt.Fprintf(w, "Free space: %s of %s\n", hs(free), hs(total))
	}

	fmt.Fprintf(w, "\nTree (%s, depth %d):\n", m, o.depth)
	// prefix is drawn before this node's name; indent before its children's.
	var tree func(n *scan.Node, prefix, indent string, depth int)
	tree = func(n *scan.Node, prefix, indent string, depth int) {
		name := n.Name
		if n.Parent == nil {
			name = n.DisplayPath()
		}
		fmt.Fprintf(w, "%10s %6s  %s%s%s\n", hs(n.Size(m)), pctOf(n.Size(m), root.Size(m)), prefix, name, notes(n))
		if depth >= o.depth {
			return
		}
		kids := analysis.LargestChildren(n, m)
		for i, c := range kids {
			if i == len(kids)-1 {
				tree(c, indent+"└─ ", indent+"   ", depth+1)
			} else {
				tree(c, indent+"├─ ", indent+"│  ", depth+1)
			}
		}
	}
	tree(root, "", "", 0)

	fmt.Fprintf(w, "\nLargest files (%s", m)
	if o.filter.MinSize > 0 {
		fmt.Fprintf(w, ", at least %s", hs(o.filter.MinSize))
	}
	if o.filter.MinAge > 0 {
		fmt.Fprintf(w, ", unmodified for %s", humanAge(o.filter.MinAge))
	}
	fmt.Fprintln(w, "):")
	for _, f := range analysis.Largest(root, o.filter, o.top) {
		fmt.Fprintf(w, "%10s  %s  %s\n", hs(f.Size(m)), f.ModTime.Format("2006-01-02"), f.RelPath(root))
	}

	cs := analysis.Reclaimable(root)
	fmt.Fprintf(w, "\nReclaimable (%s total, nothing removed):\n", hs(analysis.ReclaimableTotal(cs, m)))
	for _, c := range cs {
		fmt.Fprintf(w, "%10s  %-12s  %s\n", hs(c.Node.Size(m)), c.Category, c.Node.RelPath(root))
	}

	counts := [3]int{}
	var walk func(n *scan.Node)
	walk = func(n *scan.Node) {
		if n.Kind == scan.File {
			counts[analysis.BandOf(n.ModTime, o.filter.Now)]++
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	fmt.Fprintf(w, "\nFile ages: %d %s, %d %s, %d %s\n", counts[0], analysis.BandNames[0], counts[1], analysis.BandNames[1], counts[2], analysis.BandNames[2])
}

func notes(n *scan.Node) string {
	var s []string
	switch n.Kind {
	case scan.Dir:
		s = append(s, plural(n.Files, "file"))
	case scan.Symlink:
		s = append(s, "link → "+n.Target+", not followed")
	}
	if n.Kind == scan.File && n.Links > 1 && n.CountedUsage == 0 {
		s = append(s, "hard link, space counted under another name")
	}
	if n.Kind == scan.File && n.Apparent > 0 && n.Usage < n.Apparent/2 {
		s = append(s, "sparse: "+analysis.HumanSize(n.Apparent)+" apparent")
	}
	if n.Err != nil {
		s = append(s, "unreadable")
	}
	if len(s) == 0 {
		return ""
	}
	return "  (" + strings.Join(s, ", ") + ")"
}

func pctOf(part, whole int64) string {
	if whole <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(part)/float64(whole))
}

func humanAge(d time.Duration) string {
	switch {
	case d%analysis.Year == 0:
		return fmt.Sprintf("%dy", d/analysis.Year)
	case d%analysis.Month == 0:
		return fmt.Sprintf("%dmo", d/analysis.Month)
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	return d.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
