package count

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"loc/internal/lang"
)

// scanState carries over from one line to the next, so block comments and
// strings can span lines.
type scanState struct {
	inBlock    bool
	blockStart string // needed to spot nested openers
	blockEnd   string
	blockDepth int
	inString   bool
	strEnd     string
	strEscape  bool // whether backslash escapes apply in this string
	inFence    bool // Markdown fenced code block
}

type lineKind int

const (
	kindBlank lineKind = iota
	kindComment
	kindCode
)

// classify returns the kind of one line and updates st for the next.
func classify(def *lang.LanguageDef, line string, st *scanState) lineKind {
	switch def.Special {
	case lang.PlainText:
		if strings.TrimSpace(line) == "" {
			return kindBlank
		}
		return kindCode
	case lang.Markdown:
		return classifyMarkdown(line, st)
	}

	// Like cloc, a whitespace-only line is blank even inside a block comment.
	// Whitespace can't change the state, so there's no need to scan it.
	if strings.TrimSpace(line) == "" {
		return kindBlank
	}

	sawCode, sawComment := scanCode(def, line, st)
	if sawComment && !sawCode {
		return kindComment
	}
	return kindCode
}

// classifyMarkdown counts fenced code blocks, fences included, as code and
// prose as comment.
func classifyMarkdown(line string, st *scanState) lineKind {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		st.inFence = !st.inFence
		return kindCode
	}
	if trimmed == "" {
		return kindBlank
	}
	if st.inFence {
		return kindCode
	}
	return kindComment
}

// Kinds of marker returned by bestMatch.
const (
	mNone = iota
	mString
	mBlock
	mLine
)

// bestMatch finds the longest string, block-comment or line-comment marker at
// the start of s, so Python's `"""` beats `"`. On a tie strings win, then
// block comments.
func bestMatch(def *lang.LanguageDef, s string) (kind, length int, pair [2]string) {
	best := 0
	bk := mNone
	var bp [2]string
	for _, p := range def.Strings {
		if len(p[0]) > best && strings.HasPrefix(s, p[0]) {
			best, bk, bp = len(p[0]), mString, p
		}
	}
	for _, p := range def.BlockComments {
		if len(p[0]) > best && strings.HasPrefix(s, p[0]) {
			best, bk, bp = len(p[0]), mBlock, p
		}
	}
	for _, lc := range def.LineComments {
		if len(lc) > best && strings.HasPrefix(s, lc) {
			best, bk, bp = len(lc), mLine, [2]string{lc, ""}
		}
	}
	return bk, best, bp
}

// scanCode reports whether the line has any code and whether it has any
// comment.
func scanCode(def *lang.LanguageDef, line string, st *scanState) (sawCode, sawComment bool) {
	i, n := 0, len(line)
	for i < n {
		switch {
		case st.inBlock:
			if def.NestedBlocks && st.blockStart != "" && strings.HasPrefix(line[i:], st.blockStart) {
				st.blockDepth++
				sawComment = true
				i += len(st.blockStart)
				continue
			}
			if strings.HasPrefix(line[i:], st.blockEnd) {
				sawComment = true
				i += len(st.blockEnd)
				if def.NestedBlocks {
					st.blockDepth--
					if st.blockDepth <= 0 {
						st.blockDepth = 0
						st.inBlock = false
					}
				} else {
					st.inBlock = false
				}
				continue
			}
			sawComment = true
			i += runeWidth(line[i:])

		case st.inString:
			if st.strEscape && line[i] == '\\' {
				sawCode = true
				i += 2
				continue
			}
			if strings.HasPrefix(line[i:], st.strEnd) {
				sawCode = true
				i += len(st.strEnd)
				st.inString = false
				continue
			}
			sawCode = true
			i += runeWidth(line[i:])

		default:
			r, w := utf8.DecodeRuneInString(line[i:])
			if w == 0 {
				w = 1
			}
			if unicode.IsSpace(r) {
				i += w
				continue
			}
			kind, mlen, pair := bestMatch(def, line[i:])
			switch kind {
			case mString:
				sawCode = true
				st.inString = true
				st.strEnd = pair[1]
				st.strEscape = pair[0] != "`"
				i += mlen
			case mBlock:
				sawComment = true
				st.inBlock = true
				st.blockStart = pair[0]
				st.blockEnd = pair[1]
				st.blockDepth = 1
				i += mlen
			case mLine:
				sawComment = true
				return sawCode, sawComment
			default:
				sawCode = true
				i += w
			}
		}
	}
	return sawCode, sawComment
}

func runeWidth(s string) int {
	_, w := utf8.DecodeRuneInString(s)
	if w == 0 {
		return 1
	}
	return w
}
