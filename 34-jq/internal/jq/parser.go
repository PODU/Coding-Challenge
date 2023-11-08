package jq

import "fmt"

// Compile parses a jq filter program.
func Compile(src string) (Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	node, err := p.parseProgram()
	if err != nil {
		return nil, err
	}
	return node, nil
}

type parser struct {
	toks []token
	pos  int
}

func (p *parser) cur() token  { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }

func (p *parser) expect(k tokKind, what string) error {
	if p.cur().kind != k {
		return p.errf("expected %s", what)
	}
	p.pos++
	return nil
}

func (p *parser) errf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s at offset %d", msg, p.cur().pos)
}

func (p *parser) isIdent(v string) bool {
	t := p.cur()
	return t.kind == tIdent && t.val == v
}

func (p *parser) parseProgram() (Node, error) {
	n, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if p.cur().kind != tEOF {
		return nil, p.errf("unexpected token")
	}
	return n, nil
}

// infix returns the operator string and binding power of the current token, or
// ("",0) if it is not a binary operator.
func (p *parser) infix() (string, int) {
	t := p.cur()
	switch t.kind {
	case tPipe:
		return "|", 1
	case tComma:
		return ",", 2
	case tIdent:
		switch t.val {
		case "or":
			return "or", 4
		case "and":
			return "and", 5
		}
		return "", 0
	case tOp:
		switch t.val {
		case "//":
			return "//", 3
		case "==", "!=", "<", "<=", ">", ">=":
			return t.val, 6
		case "+", "-":
			return t.val, 7
		case "*", "/", "%":
			return t.val, 8
		}
	}
	return "", 0
}

func rightAssoc(op string) bool {
	return op == "|" || op == "," || op == "//"
}

func (p *parser) parseBinary(minbp int) (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op, bp := p.infix()
		if bp == 0 || bp < minbp {
			break
		}
		p.pos++ // consume operator
		nextMin := bp + 1
		if rightAssoc(op) {
			nextMin = bp
		}
		right, err := p.parseBinary(nextMin)
		if err != nil {
			return nil, err
		}
		left = makeBinary(op, left, right)
	}
	return left, nil
}

func makeBinary(op string, l, r Node) Node {
	switch op {
	case "|":
		return pipe{l, r}
	case ",":
		return comma{l, r}
	case "//":
		return alternative{l, r}
	default:
		return binOp{op, l, r}
	}
}

func (p *parser) parseUnary() (Node, error) {
	if p.cur().kind == tOp && p.cur().val == "-" {
		p.pos++
		x, err := p.parsePostfixTerm()
		if err != nil {
			return nil, err
		}
		return unaryNeg{x}, nil
	}
	return p.parsePostfixTerm()
}

func (p *parser) parsePostfixTerm() (Node, error) {
	head, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parseSuffix(head)
}

func (p *parser) parseSuffix(head Node) (Node, error) {
	for {
		switch p.cur().kind {
		case tField:
			name := p.next().val
			head = pipe{head, field{name}}
		case tLBracket:
			p.pos++
			n, err := p.parseBracket()
			if err != nil {
				return nil, err
			}
			head = pipe{head, n}
		case tDot:
			// expr."str" or expr.[...]
			p.pos++
			switch p.cur().kind {
			case tString:
				s, ok := partsStatic(p.next().parts)
				if !ok {
					return nil, p.errf("interpolated string not allowed as field")
				}
				head = pipe{head, field{s}}
			case tLBracket:
				p.pos++
				n, err := p.parseBracket()
				if err != nil {
					return nil, err
				}
				head = pipe{head, n}
			default:
				return nil, p.errf("expected field or index after '.'")
			}
		case tQuestion:
			p.pos++
			head = optional{head}
		default:
			return head, nil
		}
	}
}

func (p *parser) parseBracket() (Node, error) {
	if p.cur().kind == tRBracket {
		p.pos++
		return iterate{}, nil
	}
	if p.cur().kind == tColon {
		p.pos++
		hi, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tRBracket, "']'"); err != nil {
			return nil, err
		}
		return sliceExpr{nil, hi}, nil
	}
	lo, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if p.cur().kind == tColon {
		p.pos++
		if p.cur().kind == tRBracket {
			p.pos++
			return sliceExpr{lo, nil}, nil
		}
		hi, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tRBracket, "']'"); err != nil {
			return nil, err
		}
		return sliceExpr{lo, hi}, nil
	}
	if err := p.expect(tRBracket, "']'"); err != nil {
		return nil, err
	}
	return indexExpr{lo}, nil
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.cur()
	switch t.kind {
	case tDot:
		p.pos++
		switch p.cur().kind {
		case tLBracket:
			p.pos++
			return p.parseBracket()
		case tString:
			s, ok := partsStatic(p.next().parts)
			if !ok {
				return nil, p.errf("interpolated string not allowed as field")
			}
			return field{s}, nil
		default:
			return identity{}, nil
		}
	case tField:
		p.pos++
		return field{t.val}, nil
	case tDotDot:
		p.pos++
		return recurseDefault{}, nil
	case tNumber:
		p.pos++
		return literal{t.num}, nil
	case tString:
		p.pos++
		return p.buildString(t.parts, "")
	case tFormat:
		p.pos++
		if p.cur().kind == tString {
			st := p.next()
			return p.buildString(st.parts, t.val)
		}
		return formatExpr{t.val}, nil
	case tLBracket:
		p.pos++
		if p.cur().kind == tRBracket {
			p.pos++
			return arrayConstruct{nil}, nil
		}
		inner, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tRBracket, "']'"); err != nil {
			return nil, err
		}
		return arrayConstruct{inner}, nil
	case tLBrace:
		return p.parseObject()
	case tLParen:
		p.pos++
		e, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tRParen, "')'"); err != nil {
			return nil, err
		}
		return e, nil
	case tIdent:
		switch t.val {
		case "null":
			p.pos++
			return literal{nil}, nil
		case "true":
			p.pos++
			return literal{true}, nil
		case "false":
			p.pos++
			return literal{false}, nil
		case "if":
			return p.parseIf()
		case "try":
			return p.parseTry()
		case "and", "or", "then", "else", "elif", "end", "catch":
			return nil, p.errf("unexpected keyword %q", t.val)
		case "def", "reduce", "foreach", "as", "import", "include", "label":
			return nil, p.errf("%q is not supported in ccjq", t.val)
		default:
			return p.parseFuncCall(t.val)
		}
	default:
		return nil, p.errf("unexpected token")
	}
}

func (p *parser) parseFuncCall(name string) (Node, error) {
	p.pos++ // consume name
	var args []Node
	if p.cur().kind == tLParen {
		p.pos++
		for {
			a, err := p.parseBinary(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if p.cur().kind == tSemicolon {
				p.pos++
				continue
			}
			break
		}
		if err := p.expect(tRParen, "')'"); err != nil {
			return nil, err
		}
	}
	if !builtinExists(name) {
		return nil, fmt.Errorf("%s/%d is not defined", name, len(args))
	}
	if !builtinArityOK(name, len(args)) {
		return nil, fmt.Errorf("%s/%d is not defined", name, len(args))
	}
	return funcCall{name, args}, nil
}

func (p *parser) parseIf() (Node, error) {
	p.pos++ // if
	var conds, thens []Node
	cond, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if !p.isIdent("then") {
		return nil, p.errf("expected 'then'")
	}
	p.pos++
	thenN, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	conds = append(conds, cond)
	thens = append(thens, thenN)
	for p.isIdent("elif") {
		p.pos++
		c, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if !p.isIdent("then") {
			return nil, p.errf("expected 'then'")
		}
		p.pos++
		tn, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		conds = append(conds, c)
		thens = append(thens, tn)
	}
	var els Node
	if p.isIdent("else") {
		p.pos++
		els, err = p.parseBinary(0)
		if err != nil {
			return nil, err
		}
	}
	if !p.isIdent("end") {
		return nil, p.errf("expected 'end'")
	}
	p.pos++
	return ifExpr{conds, thens, els}, nil
}

func (p *parser) parseTry() (Node, error) {
	p.pos++ // try
	body, err := p.parsePostfixTerm()
	if err != nil {
		return nil, err
	}
	var handler Node
	if p.isIdent("catch") {
		p.pos++
		handler, err = p.parsePostfixTerm()
		if err != nil {
			return nil, err
		}
	}
	return tryCatch{body, handler}, nil
}

func (p *parser) parseObject() (Node, error) {
	p.pos++ // {
	if p.cur().kind == tRBrace {
		p.pos++
		return objectConstruct{nil}, nil
	}
	var entries []objEntry
	for {
		e, err := p.parseObjEntry()
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
		if p.cur().kind == tComma {
			p.pos++
			continue
		}
		break
	}
	if err := p.expect(tRBrace, "'}'"); err != nil {
		return nil, err
	}
	return objectConstruct{entries}, nil
}

func (p *parser) parseObjEntry() (objEntry, error) {
	var keyNode Node
	var shorthand Node
	t := p.cur()
	switch t.kind {
	case tIdent:
		p.pos++
		keyNode = literal{t.val}
		shorthand = field{t.val}
	case tString:
		p.pos++
		s, ok := partsStatic(t.parts)
		if ok {
			keyNode = literal{s}
			shorthand = field{s}
		} else {
			kn, err := p.buildString(t.parts, "")
			if err != nil {
				return objEntry{}, err
			}
			keyNode = kn
		}
	case tFormat:
		// @base64 "..." key
		p.pos++
		if p.cur().kind != tString {
			return objEntry{}, p.errf("expected string after format in object key")
		}
		st := p.next()
		kn, err := p.buildString(st.parts, t.val)
		if err != nil {
			return objEntry{}, err
		}
		keyNode = kn
	case tLParen:
		p.pos++
		kn, err := p.parseBinary(0)
		if err != nil {
			return objEntry{}, err
		}
		if err := p.expect(tRParen, "')'"); err != nil {
			return objEntry{}, err
		}
		keyNode = kn
	default:
		return objEntry{}, p.errf("invalid object key")
	}

	if p.cur().kind == tColon {
		p.pos++
		val, err := p.parseObjVal()
		if err != nil {
			return objEntry{}, err
		}
		return objEntry{keyNode, val}, nil
	}
	if shorthand == nil {
		return objEntry{}, p.errf("object key requires a value")
	}
	return objEntry{keyNode, shorthand}, nil
}

// parseObjVal parses an object value, which jq's grammar (ExpD) limits to a
// pipe of postfix terms with optional unary minus. So, as in jq, `{a: 1+2}`
// has to be written `{a: (1+2)}`.
func (p *parser) parseObjVal() (Node, error) {
	left, err := p.parseObjValTerm()
	if err != nil {
		return nil, err
	}
	for p.cur().kind == tPipe {
		p.pos++
		right, err := p.parseObjValTerm()
		if err != nil {
			return nil, err
		}
		left = pipe{left, right}
	}
	return left, nil
}

func (p *parser) parseObjValTerm() (Node, error) {
	if p.cur().kind == tOp && p.cur().val == "-" {
		p.pos++
		x, err := p.parsePostfixTerm()
		if err != nil {
			return nil, err
		}
		return unaryNeg{x}, nil
	}
	return p.parsePostfixTerm()
}

func (p *parser) buildString(parts []strSeg, format string) (Node, error) {
	if format == "" {
		if s, ok := partsStatic(parts); ok {
			return literal{s}, nil
		}
	}
	ip := make([]interpPart, 0, len(parts))
	for _, seg := range parts {
		if !seg.IsExpr {
			ip = append(ip, interpPart{text: seg.Text})
			continue
		}
		sub, err := Compile(seg.Expr)
		if err != nil {
			return nil, err
		}
		ip = append(ip, interpPart{isExpr: true, expr: sub})
	}
	return strInterp{parts: ip, format: format}, nil
}

func partsStatic(parts []strSeg) (string, bool) {
	s := ""
	for _, p := range parts {
		if p.IsExpr {
			return "", false
		}
		s += p.Text
	}
	return s, true
}
