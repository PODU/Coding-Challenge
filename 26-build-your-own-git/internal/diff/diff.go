// Package diff produces unified diffs between two texts using a longest common
// subsequence (LCS) over lines.
package diff

import (
	"fmt"
	"strings"
)

type opKind int

const (
	opEqual opKind = iota
	opInsert
	opDelete
)

// tagged is an op annotated with its 1-based line numbers on each side.
type tagged struct {
	kind opKind
	line string
	aNum int // line number in a (delete/equal), else 0
	bNum int // line number in b (insert/equal), else 0
}

const contextLines = 3

// nonlMarker is appended to a final line that lacks a trailing newline so the
// LCS treats "x\n" and "x" as distinct; it is stripped before display.
const nonlMarker = "\x00\x00\x00NONL"

// splitLines splits text into lines, reporting whether the final newline is
// missing. Returned elements exclude the terminating '\n'.
func splitLines(s string) (lines []string, noFinalNewline bool) {
	if s == "" {
		return nil, false
	}
	noFinalNewline = !strings.HasSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n"), noFinalNewline
}

// lcsTagged computes equal/insert/delete operations turning a into b, each
// tagged with line numbers.
func lcsTagged(a, b []string) []tagged {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var ops []tagged
	i, j := 0, 0
	an, bn := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			an++
			bn++
			ops = append(ops, tagged{opEqual, a[i], an, bn})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			an++
			ops = append(ops, tagged{opDelete, a[i], an, 0})
			i++
		} else {
			bn++
			ops = append(ops, tagged{opInsert, b[j], 0, bn})
			j++
		}
	}
	for ; i < n; i++ {
		an++
		ops = append(ops, tagged{opDelete, a[i], an, 0})
	}
	for ; j < m; j++ {
		bn++
		ops = append(ops, tagged{opInsert, b[j], 0, bn})
	}
	return ops
}

type hunkLine struct {
	prefix     string
	text       string
	lastOfSide bool
}

type hunk struct {
	aStart, aLen int
	bStart, bLen int
	lines        []hunkLine
}

// Unified returns a unified diff of a vs b, or "" when identical.
func Unified(aLabel, bLabel, a, b string) string {
	aLines, aNoNL := splitLines(a)
	bLines, bNoNL := splitLines(b)

	// Encode the missing-final-newline state into the comparison tokens so a
	// trailing-newline-only change is still reported (git does this too).
	aCmp := append([]string(nil), aLines...)
	bCmp := append([]string(nil), bLines...)
	if aNoNL && len(aCmp) > 0 {
		aCmp[len(aCmp)-1] += nonlMarker
	}
	if bNoNL && len(bCmp) > 0 {
		bCmp[len(bCmp)-1] += nonlMarker
	}

	ops := lcsTagged(aCmp, bCmp)
	hunks := buildHunks(ops)
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", aLabel)
	fmt.Fprintf(&sb, "+++ %s\n", bLabel)
	for _, h := range hunks {
		fmt.Fprintf(&sb, "@@ -%s +%s @@\n", rangeStr(h.aStart, h.aLen), rangeStr(h.bStart, h.bLen))
		for _, l := range h.lines {
			sb.WriteString(l.text)
			sb.WriteByte('\n')
			if l.lastOfSide {
				if (l.prefix == "-" && aNoNL) || (l.prefix == "+" && bNoNL) ||
					(l.prefix == " " && (aNoNL || bNoNL)) {
					sb.WriteString("\\ No newline at end of file\n")
				}
			}
		}
	}
	return sb.String()
}

func rangeStr(start, length int) string {
	if length == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, length)
}

func buildHunks(seq []tagged) []hunk {
	changed := func(i int) bool { return seq[i].kind != opEqual }
	n := len(seq)

	anyChange := false
	for i := 0; i < n; i++ {
		if changed(i) {
			anyChange = true
			break
		}
	}
	if !anyChange {
		return nil
	}

	var hunks []hunk
	i := 0
	for i < n {
		if !changed(i) {
			i++
			continue
		}
		start := i
		for ctx := 0; start > 0 && !changed(start-1) && ctx < contextLines; ctx++ {
			start--
		}
		end := i
		for end < n {
			if changed(end) {
				end++
				continue
			}
			gap, j := 0, end
			for j < n && !changed(j) {
				gap++
				j++
			}
			if j < n && gap <= contextLines*2 {
				end = j
				continue
			}
			break
		}
		for tail := 0; end < n && !changed(end) && tail < contextLines; tail++ {
			end++
		}

		hunks = append(hunks, makeHunk(seq[start:end]))
		i = end
	}
	return hunks
}

func makeHunk(seq []tagged) hunk {
	var h hunk
	for _, t := range seq {
		var prefix string
		switch t.kind {
		case opEqual:
			prefix = " "
		case opInsert:
			prefix = "+"
		case opDelete:
			prefix = "-"
		}
		display := strings.TrimSuffix(t.line, nonlMarker)
		h.lines = append(h.lines, hunkLine{prefix: prefix, text: prefix + display})

		if t.kind == opEqual || t.kind == opDelete {
			if h.aStart == 0 {
				h.aStart = t.aNum
			}
			h.aLen++
		}
		if t.kind == opEqual || t.kind == opInsert {
			if h.bStart == 0 {
				h.bStart = t.bNum
			}
			h.bLen++
		}
	}
	// An empty side is reported as starting at 0; a non-empty side starts at 1
	// or later.
	if h.aLen == 0 {
		h.aStart = 0
	} else if h.aStart == 0 {
		h.aStart = 1
	}
	if h.bLen == 0 {
		h.bStart = 0
	} else if h.bStart == 0 {
		h.bStart = 1
	}
	markLast(&h)
	return h
}

func markLast(h *hunk) {
	last := map[string]int{"-": -1, "+": -1, " ": -1}
	for i, l := range h.lines {
		last[l.prefix] = i
	}
	for _, idx := range last {
		if idx >= 0 {
			h.lines[idx].lastOfSide = true
		}
	}
}
