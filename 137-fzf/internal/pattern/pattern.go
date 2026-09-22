// Package pattern turns a query string into something that can be matched
// against an item.
//
// In extended-search mode (the default) a query is a whitespace-separated list
// of terms that must all match, where each term can carry a prefix that
// changes how it matches: 'exact, ^prefix, suffix$, !negation, and | to make
// the neighbouring terms alternatives of each other.
package pattern

import (
	"strings"

	"ccfzf/internal/algo"
)

type TermType int

const (
	TermFuzzy TermType = iota
	TermExact
	TermPrefix
	TermSuffix
	TermEqual
)

// CaseMode decides how a query's case is interpreted.
type CaseMode int

const (
	CaseSmart CaseMode = iota // case-insensitive until the query has an uppercase letter
	CaseIgnore
	CaseRespect
)

// Term is one space-separated piece of the query.
type Term struct {
	Typ           TermType
	Inv           bool
	Text          []rune
	CaseSensitive bool
	Orig          string
	fn            algo.Fn
}

// TermSet is a group of terms joined by |: any one of them satisfies the set.
type TermSet []Term

type Pattern struct {
	Raw      string
	TermSets []TermSet
	Extended bool
	Forward  bool
	fuzzyFn  algo.Fn
	exactFn  algo.Fn
}

// Options are the query-independent switches that affect matching.
type Options struct {
	Extended bool // extended-search mode (-x / +x)
	Exact    bool // -e: bare terms match exactly instead of fuzzily
	Case     CaseMode
	AlgoV1   bool // --algo=v1
	Forward  bool // false when --tiebreak=end asks for last-occurrence matches
}

// DefaultOptions mirrors fzf's defaults.
func DefaultOptions() Options {
	return Options{Extended: true, Case: CaseSmart, Forward: true}
}

// Build compiles a query. It never fails: an unparseable fragment is simply
// treated as literal text, because the query is being retyped on every
// keystroke and an error message would be useless.
func Build(query string, opts Options) *Pattern {
	fuzzyFn := algo.FuzzyMatchV2
	if opts.AlgoV1 {
		fuzzyFn = algo.FuzzyMatchV1
	}
	p := &Pattern{
		Raw:      query,
		Extended: opts.Extended,
		Forward:  opts.Forward,
		fuzzyFn:  fuzzyFn,
		exactFn:  algo.ExactMatchNaive,
	}
	if opts.Extended {
		p.TermSets = p.parseTerms(query, opts)
	} else {
		text := strings.TrimSpace(query)
		if text != "" {
			typ := TermFuzzy
			if opts.Exact {
				typ = TermExact
			}
			p.TermSets = []TermSet{{p.newTerm(typ, false, text, opts.Case)}}
		}
	}
	return p
}

func (p *Pattern) IsEmpty() bool { return len(p.TermSets) == 0 }

func (p *Pattern) newTerm(typ TermType, inv bool, text string, mode CaseMode) Term {
	caseSensitive := false
	switch mode {
	case CaseRespect:
		caseSensitive = true
	case CaseSmart:
		caseSensitive = text != algo.ToLowerString(text)
	}
	t := Term{Typ: typ, Inv: inv, Orig: text, CaseSensitive: caseSensitive}
	if caseSensitive {
		t.Text = []rune(text)
	} else {
		t.Text = []rune(algo.ToLowerString(text))
	}
	switch typ {
	case TermFuzzy:
		t.fn = p.fuzzyFn
	case TermExact:
		t.fn = p.exactFn
	case TermPrefix:
		t.fn = algo.PrefixMatch
	case TermSuffix:
		t.fn = algo.SuffixMatch
	case TermEqual:
		t.fn = algo.EqualMatch
	}
	return t
}

// splitTokens splits on unescaped whitespace, keeping "\ " as a literal space.
func splitTokens(query string) []string {
	const escapedSpace = "\x00"
	replaced := strings.ReplaceAll(query, "\\ ", escapedSpace)
	var tokens []string
	for _, tok := range strings.Fields(replaced) {
		tokens = append(tokens, strings.ReplaceAll(tok, escapedSpace, " "))
	}
	return tokens
}

func (p *Pattern) parseTerms(query string, opts Options) []TermSet {
	var sets []TermSet
	var set TermSet
	switchSet := true
	afterBar := false

	for _, token := range splitTokens(query) {
		if token == "|" {
			// The next term is an alternative to the previous one.
			switchSet = false
			afterBar = true
			continue
		}
		typ := TermFuzzy
		inv := false
		text := token

		if strings.HasPrefix(text, "!") {
			inv = true
			text = text[1:]
		}
		switch {
		case strings.HasPrefix(text, "'"):
			// In exact mode the quote means the opposite: match fuzzily.
			if opts.Exact {
				typ = TermFuzzy
			} else {
				typ = TermExact
			}
			text = text[1:]
		case strings.HasPrefix(text, "^") && strings.HasSuffix(text, "$") && len(text) > 1:
			typ = TermEqual
			text = text[1 : len(text)-1]
		case strings.HasPrefix(text, "^"):
			typ = TermPrefix
			text = text[1:]
		case strings.HasSuffix(text, "$"):
			typ = TermSuffix
			text = text[:len(text)-1]
		case opts.Exact:
			typ = TermExact
		}
		if text == "" {
			// A lone modifier such as "^" is still being typed; ignore it, but
			// do not lose a pending bar.
			if afterBar {
				switchSet = false
			}
			continue
		}
		if switchSet && len(set) > 0 {
			sets = append(sets, set)
			set = nil
		}
		set = append(set, p.newTerm(typ, inv, text, opts.Case))
		switchSet = true
		afterBar = false
	}
	if len(set) > 0 {
		sets = append(sets, set)
	}
	return sets
}

// Match runs the pattern against one item's matching text.
//
// The returned Result spans every term that matched (so --tiebreak=begin/end
// see the whole match, not just one term) and carries the summed score.
// When withPos is set, pos holds every matched character index, unsorted.
func (p *Pattern) Match(text algo.Chars, withPos bool, slab *algo.Slab) (algo.Result, []int, bool) {
	if len(p.TermSets) == 0 {
		return algo.Result{Start: 0, End: 0, Score: 0}, nil, true
	}
	total := 0
	minStart, maxEnd := -1, 0
	var allPos []int

	for _, set := range p.TermSets {
		matched := false
		for _, term := range set {
			res, pos := term.fn(term.CaseSensitive, p.Forward, text, term.Text, withPos && !term.Inv, slab)
			if res.Matched() {
				if term.Inv {
					// This item is excluded unless a sibling term rescues it.
					continue
				}
				matched = true
				total += res.Score
				if minStart < 0 || res.Start < minStart {
					minStart = res.Start
				}
				if res.End > maxEnd {
					maxEnd = res.End
				}
				allPos = append(allPos, pos...)
				break
			} else if term.Inv {
				// The term it must not contain is absent: the set is satisfied
				// but contributes no score or offsets.
				matched = true
				continue
			}
		}
		if !matched {
			return algo.NoMatch, nil, false
		}
	}
	if minStart < 0 {
		// Every set was satisfied by negation alone.
		minStart, maxEnd = 0, 0
	}
	return algo.Result{Start: minStart, End: maxEnd, Score: total}, allPos, true
}
