// Package ccomment strips comments from source code without changing what the
// code does. Each language has a small scanner that knows its string syntax,
// so a "//" or "#" inside a string is left alone.
//
// Scanners only report where the comments are. The shared rewriter decides
// which ones to remove and how to tidy the whitespace left behind.
package ccomment

import "fmt"

type SpanKind int

const (
	// Code is never actually reported; whatever isn't a comment is code.
	Code SpanKind = iota
	LineComment
	BlockComment
)

// Span is one comment, as byte offsets into the source. End is exclusive, and
// a line comment's span stops before the newline.
type Span struct {
	Kind       SpanKind
	Start, End int
	FullLine   bool   // only whitespace precedes the comment on its line
	Text       string // the comment itself, including its delimiters
}

// Mode says what happens to a line that held nothing but a comment.
type Mode int

const (
	Drop  Mode = iota // remove the line
	Blank             // leave it empty, so line numbers don't change
)

type Options struct {
	KeepDoc    bool // keep Go doc comments
	KeepPragma bool // keep tool directives
	BlankLines Mode
	Minify     bool // squeeze runs of blank lines down to one
	GoAST      bool // Go only: find doc comments with go/parser
}

type Stats struct{ CommentsRemoved, CommentsKept, BytesRemoved int }

// Stripper is one language's comment remover.
type Stripper interface {
	Lang() string
	// Scan finds every comment in src, in order.
	Scan(src []byte) ([]Span, error)
	Strip(src []byte, opt Options) (out []byte, stats Stats, err error)
}

// UnterminatedError means a block comment runs to the end of the input.
type UnterminatedError struct{ Offset int }

func (e *UnterminatedError) Error() string {
	return fmt.Sprintf("unterminated block comment starting at byte offset %d", e.Offset)
}

func isHSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\f' || b == '\v'
}

// leadingOnlyWS reports whether only spaces and tabs come before start on its
// line.
func leadingOnlyWS(src []byte, start int) bool {
	i := start
	for i > 0 && src[i-1] != '\n' {
		i--
		if !isHSpace(src[i]) {
			return false
		}
	}
	return true
}

// onlyWSAfter reports whether only spaces and tabs follow end on its line.
func onlyWSAfter(src []byte, end int) bool {
	for i := end; i < len(src) && src[i] != '\n'; i++ {
		if !isHSpace(src[i]) {
			return false
		}
	}
	return true
}

func mkLine(src []byte, start, end int) Span {
	return Span{Kind: LineComment, Start: start, End: end,
		FullLine: leadingOnlyWS(src, start), Text: string(src[start:end])}
}

func mkBlock(src []byte, start, end int) Span {
	return Span{Kind: BlockComment, Start: start, End: end,
		FullLine: leadingOnlyWS(src, start), Text: string(src[start:end])}
}

// stripWith removes every span that keep rejects, and cleans up the
// whitespace each one leaves behind.
func stripWith(src []byte, spans []Span, opt Options, keep func(Span) bool) ([]byte, Stats) {
	var out []byte
	var st Stats
	cursor := 0

	for _, s := range spans {
		if keep(s) {
			st.CommentsKept++
			continue // it gets copied along with the code around it
		}
		st.CommentsRemoved++
		st.BytesRemoved += s.End - s.Start

		out = append(out, src[cursor:s.Start]...)
		cursor = s.End

		lineStart := s.Start
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}
		leadWS := leadingOnlyWS(src, s.Start)
		eol := s.End
		for eol < len(src) && src[eol] != '\n' {
			eol++
		}
		trailWS := onlyWSAfter(src, s.End)
		fullLine := leadWS && trailWS

		switch {
		case fullLine:
			// Take back the indentation already copied for this line.
			if n := s.Start - lineStart; n > 0 && n <= len(out) {
				out = out[:len(out)-n]
			}
			if opt.BlankLines == Drop {
				if eol < len(src) {
					eol++ // the newline goes too
				}
			} else if s.Kind == BlockComment {
				appendNewlines(&out, src[s.Start:s.End])
			}
			cursor = eol
		case s.Kind == LineComment || trailWS:
			// Comment at the end of a line of code.
			out = trimTrailingHSpace(out)
		default:
			// A block comment with code after it. Keep the newlines it
			// contained so the code after it stays on the same line number.
			appendNewlines(&out, src[s.Start:s.End])
		}
	}
	out = append(out, src[cursor:]...)

	if opt.Minify {
		out = collapseBlankLines(out)
	}
	return out, st
}

func trimTrailingHSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}

func appendNewlines(dst *[]byte, seg []byte) {
	for _, b := range seg {
		if b == '\n' || b == '\r' {
			*dst = append(*dst, b)
		}
	}
}

// collapseBlankLines turns each run of blank lines into a single one.
func collapseBlankLines(src []byte) []byte {
	var out []byte
	blanks := 0
	i := 0
	for i < len(src) {
		j := i
		for j < len(src) && src[j] != '\n' {
			j++
		}
		hasNL := j < len(src)
		line := src[i:j]
		if len(trimTrailingHSpace(line)) == 0 {
			blanks++
			if blanks > 1 {
				if hasNL {
					j++
				}
				i = j
				continue
			}
		} else {
			blanks = 0
		}
		out = append(out, line...)
		if hasNL {
			out = append(out, '\n')
			j++
		}
		i = j
	}
	return out
}
