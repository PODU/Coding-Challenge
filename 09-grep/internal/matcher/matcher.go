// Package matcher keeps the regex engine behind an interface so it can be
// swapped for a hand-written one later.
package matcher

import "regexp"

// Matcher reports whether a single line satisfies the compiled pattern.
type Matcher interface {
	Matches(line string) bool
}

type regexMatcher struct {
	re *regexp.Regexp
}

func (m regexMatcher) Matches(line string) bool { return m.re.MatchString(line) }

// New compiles pattern with the -i and -w options applied. An empty pattern
// matches every line, even with -w.
func New(pattern string, ignoreCase, wordMatch bool) (Matcher, error) {
	p := pattern
	if wordMatch && p != "" {
		p = `\b(?:` + p + `)\b`
	}
	if ignoreCase {
		p = `(?i)` + p
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	return regexMatcher{re: re}, nil
}
