package query

import (
	"fmt"
	"strconv"
)

// parseError describes a parse failure at a token position.
type parseError struct {
	pos int
	msg string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("filter syntax error at column %d: %s", e.pos, e.msg)
}

// Compile lexes and parses a filter string into an executable Filter.
// An empty/whitespace-only filter compiles to the identity filter.
func Compile(src string) (Filter, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	if p.peek().typ == tEOF {
		return Identity{}, nil
	}
	f, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}
	if p.peek().typ != tEOF {
		t := p.peek()
		return nil, &parseError{t.pos, fmt.Sprintf("unexpected %s", t.typ)}
	}
	return f, nil
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) peekN(n int) token {
	if p.i+n < len(p.toks) {
		return p.toks[p.i+n]
	}
	return p.toks[len(p.toks)-1] // tEOF
}
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

// pipeline = filter { "|" filter }
func (p *parser) parsePipeline() (Filter, error) {
	left, err := p.parseFilter()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tPipe {
		p.next() // consume '|'
		right, err := p.parseFilter()
		if err != nil {
			return nil, err
		}
		left = Pipe{Left: left, Right: right}
	}
	return left, nil
}

// filter = "[" pipeline "]"   (collection)
//
//	| term
//
// A leading "[" is a collection unless it is exactly `["string"]` or `[int]`,
// which is the leading-bracket object/array access form.
func (p *parser) parseFilter() (Filter, error) {
	if p.peek().typ == tLBracket && !p.isLeadingSubscript() {
		lb := p.next() // consume '['
		inner, err := p.parsePipeline()
		if err != nil {
			return nil, err
		}
		if p.peek().typ != tRBracket {
			return nil, &parseError{p.peek().pos, fmt.Sprintf("expected ']' to close collection opened at column %d", lb.pos)}
		}
		p.next() // consume ']'
		return Collect{Inner: inner}, nil
	}
	return p.parseTerm()
}

// isLeadingSubscript reports whether the bracket at the cursor is `[string]`
// or `[int]` (a leading object/array access), as opposed to a collection.
func (p *parser) isLeadingSubscript() bool {
	return p.peek().typ == tLBracket &&
		(p.peekN(1).typ == tString || p.peekN(1).typ == tInt) &&
		p.peekN(2).typ == tRBracket
}

// term = "." ( path | )   (identity when '.' stands alone)
//
//	| path             (leading-bracket form)
func (p *parser) parseTerm() (Filter, error) {
	t := p.peek()
	switch t.typ {
	case tDot:
		// identity unless followed by an identifier or subscript bracket
		if p.peekN(1).typ == tIdent || p.peekN(1).typ == tLBracket {
			return p.parsePath()
		}
		p.next() // consume '.'
		return Identity{}, nil
	case tLBracket:
		return p.parsePath()
	default:
		return nil, &parseError{t.pos, fmt.Sprintf("expected '.' or '[' but found %s", t.typ)}
	}
}

// path = segment { segment } ; segments are composed via Pipe.
func (p *parser) parsePath() (Filter, error) {
	var segs []Filter
	for {
		t := p.peek()
		if t.typ == tDot {
			nt := p.peekN(1)
			if nt.typ == tIdent {
				p.next() // '.'
				id := p.next()
				seg := Field{Name: id.val, Optional: p.consumeOptional()}
				segs = append(segs, seg)
				continue
			}
			if nt.typ == tLBracket {
				p.next() // '.'
				seg, err := p.parseSubscript()
				if err != nil {
					return nil, err
				}
				segs = append(segs, seg)
				continue
			}
			// '.' with nothing meaningful after -> stop; let caller validate.
			break
		}
		if t.typ == tLBracket {
			seg, err := p.parseSubscript()
			if err != nil {
				return nil, err
			}
			segs = append(segs, seg)
			continue
		}
		break
	}
	if len(segs) == 0 {
		return nil, &parseError{p.peek().pos, "expected a path segment"}
	}
	f := segs[0]
	for _, s := range segs[1:] {
		f = Pipe{Left: f, Right: s}
	}
	return f, nil
}

// parseSubscript parses "[" ( string | int | <empty> ) "]" with optional '?'.
// The cursor is positioned at '['.
func (p *parser) parseSubscript() (Filter, error) {
	lb := p.next() // consume '['
	t := p.peek()
	switch t.typ {
	case tString:
		p.next()
		if err := p.expect(tRBracket); err != nil {
			return nil, err
		}
		return Field{Name: t.val, Optional: p.consumeOptional()}, nil
	case tInt:
		p.next()
		n, err := strconv.Atoi(t.val)
		if err != nil {
			return nil, &parseError{t.pos, fmt.Sprintf("invalid integer %q", t.val)}
		}
		if err := p.expect(tRBracket); err != nil {
			return nil, err
		}
		return Index{N: n, Optional: p.consumeOptional()}, nil
	case tRBracket:
		p.next() // consume ']'
		return Iterate{Optional: p.consumeOptional()}, nil
	default:
		return nil, &parseError{t.pos, fmt.Sprintf("expected string, integer or ']' inside subscript opened at column %d, found %s", lb.pos, t.typ)}
	}
}

func (p *parser) consumeOptional() bool {
	if p.peek().typ == tQuestion {
		p.next()
		return true
	}
	return false
}

func (p *parser) expect(tt tokenType) error {
	if p.peek().typ != tt {
		return &parseError{p.peek().pos, fmt.Sprintf("expected %s but found %s", tt, p.peek().typ)}
	}
	p.next()
	return nil
}
