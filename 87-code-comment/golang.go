package ccomment

import "bytes"

// goStripper uses the C-family scanner, and can keep doc comments as well as
// directives like //go:build.
type goStripper struct{ inner cStripper }

func newGoStripper() goStripper {
	return goStripper{inner: cStripper{cfg: cConfig{
		name:     "go",
		backtick: btRaw,
		pragma:   goIsPragma,
	}}}
}

func (g goStripper) Lang() string { return "go" }

func (g goStripper) Scan(src []byte) ([]Span, error) { return g.inner.Scan(src) }

func (g goStripper) Strip(src []byte, opt Options) ([]byte, Stats, error) {
	spans, err := g.Scan(src)
	if err != nil {
		return nil, Stats{}, err
	}
	li := newLineIndex(src, spans)
	keep := func(s Span) bool {
		return (opt.KeepPragma && goIsPragma(s)) || (opt.KeepDoc && li.isDoc(src, s))
	}
	out, st := stripWith(src, spans, opt, keep)
	return out, st, nil
}

// lineIndex maps offsets to lines and records which lines are nothing but a
// "//" comment. isDoc uses it.
type lineIndex struct {
	starts   []int // offset where each line begins
	src      []byte
	cmtLines map[int]bool
}

func newLineIndex(src []byte, spans []Span) *lineIndex {
	li := &lineIndex{src: src, starts: []int{0}, cmtLines: map[int]bool{}}
	for i, b := range src {
		if b == '\n' {
			li.starts = append(li.starts, i+1)
		}
	}
	for _, s := range spans {
		if s.Kind == LineComment && s.FullLine && onlyWSAfter(src, s.End) {
			li.cmtLines[li.lineOf(s.Start)] = true
		}
	}
	return li
}

func (li *lineIndex) count() int { return len(li.starts) }

func (li *lineIndex) lineOf(off int) int {
	lo, hi := 0, len(li.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if li.starts[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func (li *lineIndex) lineBytes(line int) []byte {
	if line < 0 || line >= len(li.starts) {
		return nil
	}
	start := li.starts[line]
	end := len(li.src)
	if line+1 < len(li.starts) {
		end = li.starts[line+1]
	}
	return li.src[start:end]
}

var goDeclKeywords = [][]byte{
	[]byte("package"), []byte("func"), []byte("type"),
	[]byte("const"), []byte("var"),
}

// isTopDecl reports whether line starts a top-level declaration. Only
// unindented keywords count, which rules out declarations inside functions.
func (li *lineIndex) isTopDecl(line int) bool {
	b := li.lineBytes(line)
	if len(b) == 0 || isHSpace(b[0]) {
		return false
	}
	for _, kw := range goDeclKeywords {
		if bytes.HasPrefix(b, kw) {
			rest := b[len(kw):]
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' ||
				rest[0] == '(' || rest[0] == '\n' || rest[0] == '\r' {
				return true
			}
		}
	}
	return false
}

// isDoc reports whether s is part of a doc comment: a block of full-line
// comments directly above a top-level declaration, with no blank line between.
func (li *lineIndex) isDoc(src []byte, s Span) bool {
	if !s.FullLine || !onlyWSAfter(src, s.End) {
		return false
	}
	next := li.lineOf(s.End) + 1
	for next < li.count() && li.cmtLines[next] {
		next++
	}
	if next >= li.count() {
		return false
	}
	return li.isTopDecl(next)
}
