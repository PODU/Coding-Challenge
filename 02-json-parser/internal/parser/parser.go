// Package parser is a recursive-descent JSON parser that checks input against
// the RFC 8259 grammar and builds a small AST.
package parser

import (
	"fmt"
	"strconv"

	"github.com/codingchallenges/jsonparser/internal/lexer"
)

// DefaultMaxDepth caps nesting so deeply nested input can't blow the stack.
// RFC 8259 sets no limit; WithMaxDepth(0) turns the check off.
const DefaultMaxDepth = 10000

// Parser holds one token of lookahead.
type Parser struct {
	lex      *lexer.Lexer
	cur      lexer.Token
	depth    int
	maxDepth int
}

// Option configures a Parser.
type Option func(*Parser)

// WithMaxDepth sets the recursion-depth guard (0 disables it).
func WithMaxDepth(d int) Option {
	return func(p *Parser) { p.maxDepth = d }
}

// New returns a Parser over input.
func New(input []byte, opts ...Option) *Parser {
	p := &Parser{lex: lexer.New(input), maxDepth: DefaultMaxDepth}
	for _, o := range opts {
		o(p)
	}
	p.next()
	return p
}

// Parse parses a complete JSON text and returns its AST. Errors are always
// *ParseError.
func Parse(input []byte, opts ...Option) (Value, error) {
	p := New(input, opts...)

	if p.cur.Type == lexer.EOF {
		return nil, &ParseError{Message: "unexpected end of input: empty document", Line: p.cur.Line, Col: p.cur.Col}
	}

	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	if p.cur.Type != lexer.EOF {
		return nil, &ParseError{
			Message: "trailing data after JSON value: found " + describe(p.cur),
			Line:    p.cur.Line,
			Col:     p.cur.Col,
		}
	}
	return v, nil
}

func (p *Parser) next() { p.cur = p.lex.NextToken() }

func (p *Parser) illegal() error {
	msg := p.cur.Literal
	if msg == "" {
		msg = "illegal token"
	}
	return &ParseError{Message: msg, Line: p.cur.Line, Col: p.cur.Col}
}

func (p *Parser) enter() error {
	p.depth++
	if p.maxDepth > 0 && p.depth > p.maxDepth {
		return &ParseError{
			Message: fmt.Sprintf("maximum nesting depth %d exceeded", p.maxDepth),
			Line:    p.cur.Line,
			Col:     p.cur.Col,
		}
	}
	return nil
}

func (p *Parser) leave() { p.depth-- }

func (p *Parser) parseValue() (Value, error) {
	tok := p.cur
	switch tok.Type {
	case lexer.LBRACE:
		return p.parseObject()
	case lexer.LBRACKET:
		return p.parseArray()
	case lexer.STRING:
		p.next()
		return &String{position{tok.Line, tok.Col}, tok.Literal}, nil
	case lexer.NUMBER:
		f, _ := strconv.ParseFloat(tok.Literal, 64)
		p.next()
		return &Number{position{tok.Line, tok.Col}, tok.Literal, f}, nil
	case lexer.TRUE:
		p.next()
		return &Bool{position{tok.Line, tok.Col}, true}, nil
	case lexer.FALSE:
		p.next()
		return &Bool{position{tok.Line, tok.Col}, false}, nil
	case lexer.NULL:
		p.next()
		return &Null{position{tok.Line, tok.Col}}, nil
	case lexer.ILLEGAL:
		return nil, p.illegal()
	case lexer.EOF:
		return nil, &ParseError{Message: "unexpected end of input: expected a value", Line: tok.Line, Col: tok.Col}
	default:
		return nil, &ParseError{Message: "expected a JSON value, found " + describe(tok), Line: tok.Line, Col: tok.Col}
	}
}

func (p *Parser) parseObject() (Value, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	open := p.cur
	p.next()

	obj := &Object{position: position{open.Line, open.Col}}

	if p.cur.Type == lexer.RBRACE {
		p.next()
		return obj, nil
	}

	for {
		// ILLEGAL tokens (e.g. an unquoted key) also land here so the message
		// mentions the key; describe() still includes the lexer's reason.
		if p.cur.Type != lexer.STRING {
			return nil, &ParseError{
				Message: "expected '\"' to start object key, found " + describe(p.cur),
				Line:    p.cur.Line,
				Col:     p.cur.Col,
			}
		}
		key := p.cur.Literal
		p.next()

		if p.cur.Type != lexer.COLON {
			return nil, &ParseError{
				Message: "expected ':' after object key, found " + describe(p.cur),
				Line:    p.cur.Line,
				Col:     p.cur.Col,
			}
		}
		p.next()

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj.Members = append(obj.Members, Member{Key: key, Value: val})

		switch p.cur.Type {
		case lexer.COMMA:
			p.next()
			if p.cur.Type == lexer.RBRACE {
				return nil, &ParseError{
					Message: "unexpected trailing comma before '}'",
					Line:    p.cur.Line,
					Col:     p.cur.Col,
				}
			}
		case lexer.RBRACE:
			p.next()
			return obj, nil
		case lexer.ILLEGAL:
			return nil, p.illegal()
		default:
			return nil, &ParseError{
				Message: "expected ',' or '}' after object member, found " + describe(p.cur),
				Line:    p.cur.Line,
				Col:     p.cur.Col,
			}
		}
	}
}

func (p *Parser) parseArray() (Value, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	open := p.cur
	p.next()

	arr := &Array{position: position{open.Line, open.Col}}

	if p.cur.Type == lexer.RBRACKET {
		p.next()
		return arr, nil
	}

	for {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr.Elements = append(arr.Elements, val)

		switch p.cur.Type {
		case lexer.COMMA:
			p.next()
			if p.cur.Type == lexer.RBRACKET {
				return nil, &ParseError{
					Message: "unexpected trailing comma before ']'",
					Line:    p.cur.Line,
					Col:     p.cur.Col,
				}
			}
		case lexer.RBRACKET:
			p.next()
			return arr, nil
		case lexer.ILLEGAL:
			return nil, p.illegal()
		default:
			return nil, &ParseError{
				Message: "expected ',' or ']' after array element, found " + describe(p.cur),
				Line:    p.cur.Line,
				Col:     p.cur.Col,
			}
		}
	}
}

func describe(tok lexer.Token) string {
	switch tok.Type {
	case lexer.EOF:
		return "end of input"
	case lexer.STRING:
		return "string " + strconv.Quote(tok.Literal)
	case lexer.NUMBER:
		return "number " + tok.Literal
	case lexer.TRUE, lexer.FALSE, lexer.NULL:
		return tok.Literal
	case lexer.ILLEGAL:
		if tok.Literal != "" {
			return tok.Literal
		}
		return "illegal token"
	default:
		return "'" + tok.Literal + "'"
	}
}
