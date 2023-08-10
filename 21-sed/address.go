package main

import "regexp"

// Address decides whether a command applies to the current line. A nil Address
// on a Command means "every line".
type Address interface {
	Matches(s *State) bool
}

const (
	epLine = iota
	epLast // $
	epRegex
)

// endpoint is a line number, `$`, or a regex. Single addresses and both ends
// of a range are built from it.
type endpoint struct {
	kind int
	n    int
	re   *regexp.Regexp
}

func (e endpoint) matchStart(s *State) bool {
	switch e.kind {
	case epLine:
		return s.LineNo == e.n
	case epLast:
		return s.IsLast
	case epRegex:
		return e.re.MatchString(s.Line)
	}
	return false
}

type singleAddr struct {
	ep endpoint
}

func (a *singleAddr) Matches(s *State) bool { return a.ep.matchStart(s) }

// rangeAddr is `A,B`. Once the start matches it stays active until the end
// matches, and it can trigger again afterwards, as in real sed.
type rangeAddr struct {
	start  endpoint
	end    endpoint
	active bool
}

func (r *rangeAddr) Matches(s *State) bool {
	if !r.active {
		if !r.start.matchStart(s) {
			return false
		}
		r.active = true
		// A numeric end that is <= the start line closes the range
		// immediately, so only the start line is selected (sed behaviour).
		if r.end.kind == epLine && r.end.n <= s.LineNo {
			r.active = false
		}
		return true
	}
	// Inside the range. The end is only checked from the line after the start.
	switch r.end.kind {
	case epLine:
		if s.LineNo >= r.end.n {
			r.active = false
		}
	case epLast:
		if s.IsLast {
			r.active = false
		}
	case epRegex:
		if r.end.re.MatchString(s.Line) {
			r.active = false
		}
	}
	return true
}
