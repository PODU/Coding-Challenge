// Package algo implements the string matching algorithms and the scoring model.
//
// The scoring model follows fzf's: a match is worth a fixed amount, gaps cost,
// and a bonus is added for characters that a human would consider a natural
// "handle" for the item - the start of a word, a camelCase hump, the character
// after a path separator. The v2 algorithm finds the placement of the pattern
// that maximises that score with a Smith-Waterman style dynamic program; v1 is
// a greedy two-pass approximation that never allocates a matrix.
package algo

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type charClass int

const (
	charWhite charClass = iota
	charNonWord
	charDelimiter
	charLower
	charUpper
	charLetter
	charNumber
)

const (
	scoreMatch        = 16
	scoreGapStart     = -3
	scoreGapExtension = -1

	// A character following a word boundary is the one a human would type
	// first, so it is worth half a match on its own.
	bonusBoundary = scoreMatch / 2
	// ... and doubly so when it is the first character of the pattern.
	bonusFirstCharMultiplier = 2
	bonusNonWord             = scoreMatch / 2
	bonusCamel123            = bonusBoundary + scoreGapExtension
	// Enough to cancel out the gap penalties a run of matches avoided.
	bonusConsecutive = -(scoreGapStart + scoreGapExtension)
	// Whitespace and path-ish delimiters are stronger boundaries than, say, a
	// dot in the middle of a name.
	bonusBoundaryWhite     = bonusBoundary + 2
	bonusBoundaryDelimiter = bonusBoundary + 1
)

const delimiterChars = "/,:;|"

// Result is a match: the range of the item it covers and its score.
// A Start of -1 means "no match".
type Result struct {
	Start int
	End   int
	Score int
}

// NoMatch is the zero-value failure returned by every matcher.
var NoMatch = Result{-1, -1, 0}

func (r Result) Matched() bool { return r.Start >= 0 }

// Fn is the signature shared by every matcher, so a search term can hold the
// one it needs without a type switch at match time.
type Fn func(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int)

var asciiClass [utf8.RuneSelf]charClass

func init() {
	for i := range asciiClass {
		asciiClass[i] = classOfSlow(rune(i))
	}
}

func classOfSlow(r rune) charClass {
	switch {
	case unicode.IsLower(r):
		return charLower
	case unicode.IsUpper(r):
		return charUpper
	case unicode.IsNumber(r):
		return charNumber
	case unicode.IsLetter(r):
		return charLetter
	case unicode.IsSpace(r):
		return charWhite
	case strings.ContainsRune(delimiterChars, r):
		return charDelimiter
	}
	return charNonWord
}

func charClassOf(r rune) charClass {
	if r < utf8.RuneSelf {
		return asciiClass[r]
	}
	return classOfSlow(r)
}

func bonusFor(prevClass, class charClass) int {
	if class > charNonWord {
		switch prevClass {
		case charWhite:
			return bonusBoundaryWhite
		case charDelimiter:
			return bonusBoundaryDelimiter
		case charNonWord:
			return bonusBoundary
		}
	}
	if prevClass == charLower && class == charUpper ||
		prevClass != charNumber && class == charNumber {
		return bonusCamel123
	}
	switch class {
	case charNonWord, charDelimiter:
		return bonusNonWord
	case charWhite:
		return bonusBoundaryWhite
	}
	return 0
}

// Slab holds the scratch buffers for FuzzyMatchV2. One slab per matching
// goroutine turns a per-item allocation into a per-goroutine one.
type Slab struct {
	t      []rune
	b      []int
	h0, c0 []int
	f      []int
	h, c   []int
}

func NewSlab() *Slab { return &Slab{} }

func grow(buf []int, n int) []int {
	if cap(buf) >= n {
		buf = buf[:n]
		clear(buf)
		return buf
	}
	return make([]int, n, n*2)
}

func growRunes(buf []rune, n int) []rune {
	if cap(buf) >= n {
		return buf[:n]
	}
	return make([]rune, n, n*2)
}

func posArray(withPos bool, n int) []int {
	if withPos {
		return make([]int, 0, n)
	}
	return nil
}

// fuzzyIndex reports whether every pattern character occurs in order, and
// returns the index of the first character of the greedy match. No match can
// begin before that index, so everything to its left can be ignored.
func fuzzyIndex(text Chars, pattern []rune, caseSensitive bool) (int, bool) {
	firstIdx, idx, n := 0, 0, text.Len()
	for pidx := 0; pidx < len(pattern); pidx++ {
		found := false
		for ; idx < n; idx++ {
			c := text.Get(idx)
			if !caseSensitive {
				c = toLower(c)
			}
			if c == pattern[pidx] {
				if pidx == 0 {
					firstIdx = idx
				}
				idx++
				found = true
				break
			}
		}
		if !found {
			return 0, false
		}
	}
	return firstIdx, true
}

// FuzzyMatchV2 finds the highest-scoring placement of pattern in text.
//
// Four phases: trim the text to the region a match can occupy, score the first
// pattern character at every position while collecting per-position bonuses,
// fill the score matrix row by row, then backtrace for the matched positions.
// The matrix is only (len(pattern) x width) where width spans the first
// possible to the last possible matched character, not the whole item.
func FuzzyMatchV2(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	m := len(pattern)
	if m == 0 {
		return Result{0, 0, 0}, posArray(withPos, 0)
	}
	n := text.Len()
	if m > n {
		return NoMatch, nil
	}
	if slab == nil {
		slab = NewSlab()
	}

	// Phase 1: reject, and find where the search region starts.
	idx, ok := fuzzyIndex(text, pattern, caseSensitive)
	if !ok {
		return NoMatch, nil
	}
	// All indices below are relative to idx; idx is added back at the end.
	width := n - idx
	t := growRunes(slab.t, width)
	b := grow(slab.b, width)
	h0 := grow(slab.h0, width)
	c0 := grow(slab.c0, width)
	f := grow(slab.f, m)
	slab.t, slab.b, slab.h0, slab.c0, slab.f = t, b, h0, c0, f

	// Phase 2: bonuses, and the score of matching pattern[0] at each position.
	maxScore, maxScorePos := 0, 0
	pidx, lastIdx := 0, 0
	pchar0, pchar := pattern[0], pattern[0]
	prevH0, prevClass, inGap := 0, charWhite, false
	if idx > 0 {
		prevClass = charClassOf(text.Get(idx - 1))
	}
	for off := 0; off < width; off++ {
		char := text.Get(idx + off)
		class := charClassOf(char)
		if !caseSensitive {
			char = toLower(char)
		}
		t[off] = char
		bonus := bonusFor(prevClass, class)
		b[off] = bonus
		prevClass = class

		if char == pchar {
			if pidx < m {
				f[pidx] = off
				pidx++
				if pidx < m {
					pchar = pattern[pidx]
				}
			}
			lastIdx = off
		}
		if char == pchar0 {
			score := scoreMatch + bonus*bonusFirstCharMultiplier
			h0[off] = score
			c0[off] = 1
			if m == 1 && score > maxScore {
				maxScore, maxScorePos = score, off
				// A boundary match of a one-character pattern is already the
				// best score available; stop looking.
				if bonus >= bonusBoundary {
					break
				}
			}
			inGap = false
		} else {
			if inGap {
				h0[off] = max(prevH0+scoreGapExtension, 0)
			} else {
				h0[off] = max(prevH0+scoreGapStart, 0)
			}
			c0[off] = 0
			inGap = true
		}
		prevH0 = h0[off]
	}
	if pidx != m {
		return NoMatch, nil
	}
	if m == 1 {
		res := Result{idx + maxScorePos, idx + maxScorePos + 1, maxScore}
		if !withPos {
			return res, nil
		}
		return res, []int{idx + maxScorePos}
	}

	// Phase 3: the score matrix. Row i holds the best score of matching
	// pattern[:i+1] ending at each column.
	f0 := f[0]
	mwidth := lastIdx - f0 + 1
	h := grow(slab.h, mwidth*m)
	c := grow(slab.c, mwidth*m)
	slab.h, slab.c = h, c
	copy(h, h0[f0:lastIdx+1])
	copy(c, c0[f0:lastIdx+1])

	maxScore, maxScorePos = 0, 0
	for pidx := 1; pidx < m; pidx++ {
		row := pidx * mwidth
		prow := row - mwidth
		pchar := pattern[pidx]
		start := f[pidx]
		inGap := false
		for col := start; col <= lastIdx; col++ {
			j := col - f0
			var s1, s2, consecutive int
			leftVal := 0
			if col > start {
				leftVal = h[row+j-1]
			}
			if inGap {
				s2 = leftVal + scoreGapExtension
			} else {
				s2 = leftVal + scoreGapStart
			}
			if pchar == t[col] {
				s1 = h[prow+j-1] + scoreMatch
				bns := b[col]
				consecutive = c[prow+j-1] + 1
				if consecutive > 1 {
					// A run that began on a word boundary keeps that bonus for
					// its whole length; but if this character is itself a
					// stronger boundary, start the run here instead.
					fb := b[col-consecutive+1]
					if bns >= bonusBoundary && bns > fb {
						consecutive = 1
					} else {
						bns = max(bns, max(bonusConsecutive, fb))
					}
				}
				if s1+bns < s2 {
					s1 += bonusConsecutive
					consecutive = 0
				} else {
					s1 += bns
				}
			}
			c[row+j] = consecutive
			inGap = s1 < s2
			score := max(max(s1, s2), 0)
			if pidx == m-1 && score > maxScore {
				maxScore, maxScorePos = score, col
			}
			h[row+j] = score
		}
	}

	// Phase 4: backtrace from the best end position to recover the columns
	// that were matched rather than skipped.
	j := f0
	var pos []int
	if withPos {
		pos = posArray(true, m)
		i := m - 1
		j = maxScorePos
		preferMatch := true
		for {
			ii := i * mwidth
			j0 := j - f0
			s := h[ii+j0]
			var s1, s2 int
			if i > 0 && j >= f[i] {
				s1 = h[ii-mwidth+j0-1]
			}
			if j > f[i] {
				s2 = h[ii+j0-1]
			}
			if s > s1 && (s > s2 || s == s2 && preferMatch) {
				pos = append(pos, idx+j)
				if i == 0 {
					break
				}
				i--
			}
			preferMatch = c[ii+j0] > 1 || ii+mwidth+j0+1 < len(c) && c[ii+mwidth+j0+1] > 0
			j--
		}
		// Collected end-to-start; callers expect ascending order.
		for l, r := 0, len(pos)-1; l < r; l, r = l+1, r-1 {
			pos[l], pos[r] = pos[r], pos[l]
		}
	}
	return Result{idx + j, idx + maxScorePos + 1, maxScore}, pos
}

// FuzzyMatchV1 is the greedy algorithm: scan forward for the first placement
// that contains every pattern character, then scan backward from its end to
// pull the start as far right as possible. It cannot see a better placement
// further along the item, but it runs in O(n) with no matrix.
func FuzzyMatchV1(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	if len(pattern) == 0 {
		return Result{0, 0, 0}, posArray(withPos, 0)
	}
	n := text.Len()
	pidx, sidx, eidx := 0, -1, -1
	for index := 0; index < n; index++ {
		i := index
		if !forward {
			i = n - index - 1
		}
		char := text.Get(i)
		if !caseSensitive {
			char = toLower(char)
		}
		pchar := pattern[indexAt(pidx, len(pattern), forward)]
		if char == pchar {
			if sidx < 0 {
				sidx = i
			}
			pidx++
			if pidx == len(pattern) {
				eidx = i
				break
			}
		}
	}
	if sidx < 0 || eidx < 0 {
		return NoMatch, nil
	}
	if !forward {
		sidx, eidx = eidx, sidx
	}
	// Tighten the start: the last chance each pattern character had.
	pidx--
	for index := eidx; index >= sidx; index-- {
		i := index
		if !forward {
			i = sidx + eidx - index
		}
		char := text.Get(i)
		if !caseSensitive {
			char = toLower(char)
		}
		pchar := pattern[indexAt(pidx, len(pattern), forward)]
		if char == pchar {
			pidx--
			if pidx < 0 {
				sidx = i
				break
			}
		}
	}
	if !forward {
		sidx, eidx = eidx, sidx
	}
	score, pos := calculateScore(caseSensitive, text, pattern, sidx, eidx+1, withPos)
	return Result{sidx, eidx + 1, score}, pos
}

func indexAt(index int, length int, forward bool) int {
	if forward {
		return index
	}
	return length - index - 1
}

// calculateScore walks a known match range and applies the scoring model,
// which is also how the non-fuzzy matchers get a comparable score.
func calculateScore(caseSensitive bool, text Chars, pattern []rune, sidx, eidx int, withPos bool) (int, []int) {
	pidx, score, inGap, consecutive, firstBonus := 0, 0, false, 0, 0
	pos := posArray(withPos, len(pattern))
	prevClass := charWhite
	if sidx > 0 {
		prevClass = charClassOf(text.Get(sidx - 1))
	}
	for idx := sidx; idx < eidx; idx++ {
		char := text.Get(idx)
		class := charClassOf(char)
		if !caseSensitive {
			char = toLower(char)
		}
		if pidx < len(pattern) && char == pattern[pidx] {
			if withPos {
				pos = append(pos, idx)
			}
			score += scoreMatch
			bonus := bonusFor(prevClass, class)
			if consecutive == 0 {
				firstBonus = bonus
			} else {
				if bonus >= bonusBoundary && bonus > firstBonus {
					firstBonus = bonus
				}
				bonus = max(max(bonus, firstBonus), bonusConsecutive)
			}
			if pidx == 0 {
				score += bonus * bonusFirstCharMultiplier
			} else {
				score += bonus
			}
			inGap = false
			consecutive++
			pidx++
		} else {
			if inGap {
				score += scoreGapExtension
			} else {
				score += scoreGapStart
			}
			inGap = true
			consecutive = 0
			firstBonus = 0
		}
		prevClass = class
	}
	return score, pos
}

// ExactMatchNaive matches a contiguous run of characters, picking the
// occurrence that scores best rather than simply the first one.
func ExactMatchNaive(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	m := len(pattern)
	if m == 0 {
		return Result{0, 0, 0}, posArray(withPos, 0)
	}
	n := text.Len()
	if m > n {
		return NoMatch, nil
	}
	bestStart, bestScore := -1, 0
	for index := 0; index+m <= n; index++ {
		start := index
		if !forward {
			start = n - m - index
		}
		ok := true
		for i := 0; i < m; i++ {
			char := text.Get(start + i)
			if !caseSensitive {
				char = toLower(char)
			}
			if char != pattern[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		score, _ := calculateScore(caseSensitive, text, pattern, start, start+m, false)
		if bestStart < 0 || score > bestScore {
			bestStart, bestScore = start, score
		}
	}
	if bestStart < 0 {
		return NoMatch, nil
	}
	_, pos := calculateScore(caseSensitive, text, pattern, bestStart, bestStart+m, withPos)
	return Result{bestStart, bestStart + m, bestScore}, pos
}

func PrefixMatch(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	if len(pattern) == 0 {
		return Result{0, 0, 0}, posArray(withPos, 0)
	}
	// A leading space in the pattern is significant; otherwise skip the item's
	// own leading whitespace so "^foo" matches an indented line.
	trimmed := 0
	if !unicode.IsSpace(pattern[0]) {
		for trimmed < text.Len() && unicode.IsSpace(text.Get(trimmed)) {
			trimmed++
		}
	}
	if text.Len()-trimmed < len(pattern) {
		return NoMatch, nil
	}
	for i, r := range pattern {
		char := text.Get(trimmed + i)
		if !caseSensitive {
			char = toLower(char)
		}
		if char != r {
			return NoMatch, nil
		}
	}
	score, pos := calculateScore(caseSensitive, text, pattern, trimmed, trimmed+len(pattern), withPos)
	return Result{trimmed, trimmed + len(pattern), score}, pos
}

func SuffixMatch(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	end := text.Len()
	if len(pattern) == 0 || !unicode.IsSpace(pattern[len(pattern)-1]) {
		for end > 0 && unicode.IsSpace(text.Get(end-1)) {
			end--
		}
	}
	if len(pattern) == 0 {
		return Result{end, end, 0}, posArray(withPos, 0)
	}
	start := end - len(pattern)
	if start < 0 {
		return NoMatch, nil
	}
	for i, r := range pattern {
		char := text.Get(start + i)
		if !caseSensitive {
			char = toLower(char)
		}
		if char != r {
			return NoMatch, nil
		}
	}
	score, pos := calculateScore(caseSensitive, text, pattern, start, end, withPos)
	return Result{start, end, score}, pos
}

func EqualMatch(caseSensitive bool, forward bool, text Chars, pattern []rune, withPos bool, slab *Slab) (Result, []int) {
	start, end := 0, text.Len()
	if len(pattern) == 0 {
		return NoMatch, nil
	}
	if !unicode.IsSpace(pattern[0]) {
		for start < end && unicode.IsSpace(text.Get(start)) {
			start++
		}
	}
	if !unicode.IsSpace(pattern[len(pattern)-1]) {
		for end > start && unicode.IsSpace(text.Get(end-1)) {
			end--
		}
	}
	if end-start != len(pattern) {
		return NoMatch, nil
	}
	for i, r := range pattern {
		char := text.Get(start + i)
		if !caseSensitive {
			char = toLower(char)
		}
		if char != r {
			return NoMatch, nil
		}
	}
	score, pos := calculateScore(caseSensitive, text, pattern, start, end, withPos)
	return Result{start, end, score}, pos
}
