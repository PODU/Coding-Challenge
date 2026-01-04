package mcp

import (
	"regexp"
	"strings"
)

// Solution is a single language-specific solution link parsed from the README.
type Solution struct {
	Challenge string // cleaned heading text, e.g. "Challenge 1 - Write Your Own wc Tool"
	Number    string // leading challenge number if present, e.g. "1"
	Language  string // markdown link text, treated as the language label
	URL       string // link target
}

var (
	headingRe = regexp.MustCompile(`^#{1,6}\s+(.*\S)\s*$`)
	linkRe    = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	numRe     = regexp.MustCompile(`\d+`)
)

// cleanHeading turns "[Challenge 1](url)" into "Challenge 1".
func cleanHeading(s string) string {
	return strings.TrimSpace(linkRe.ReplaceAllString(s, "$1"))
}

// parseSolutions returns every link in the README, each tagged with the
// nearest heading above it. Links before the first heading are skipped.
func parseSolutions(readme string) []Solution {
	var out []Solution
	var heading, number string

	for _, line := range strings.Split(readme, "\n") {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			heading = cleanHeading(m[1])
			number = numRe.FindString(heading)
			continue
		}
		if heading == "" {
			continue
		}
		for _, lm := range linkRe.FindAllStringSubmatch(line, -1) {
			out = append(out, Solution{
				Challenge: heading,
				Number:    number,
				Language:  strings.TrimSpace(lm[1]),
				URL:       strings.TrimSpace(lm[2]),
			})
		}
	}
	return out
}

// Find returns the README's solutions matching query and, if set, lang. A
// numeric query must equal the challenge number; anything else is a
// case-insensitive substring match on the challenge name.
func Find(readme, query, lang string) []Solution {
	return filterSolutions(parseSolutions(readme), query, lang)
}

func filterSolutions(all []Solution, query, lang string) []Solution {
	q := strings.ToLower(strings.TrimSpace(query))
	l := strings.ToLower(strings.TrimSpace(lang))
	qIsNum := q != "" && isAllDigits(q)

	var out []Solution
	for _, s := range all {
		if q != "" {
			if qIsNum {
				if s.Number != q {
					continue
				}
			} else if !strings.Contains(strings.ToLower(s.Challenge), q) {
				continue
			}
		}
		if l != "" && !strings.Contains(strings.ToLower(s.Language), l) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
