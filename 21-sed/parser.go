package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Parse compiles a sed script into an ordered list of commands.
func Parse(script string) ([]*Command, error) {
	p := &parser{s: script}
	var cmds []*Command
	for {
		p.skipSeparators()
		if p.pos >= len(p.s) {
			break
		}
		if p.s[p.pos] == '#' {
			p.skipLine()
			continue
		}
		cmd, err := p.parseCommand()
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

type parser struct {
	s   string
	pos int
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) skipSeparators() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\n', ';':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) skipLine() {
	for p.pos < len(p.s) && p.s[p.pos] != '\n' {
		p.pos++
	}
}

func (p *parser) atCommandEnd() bool {
	if p.pos >= len(p.s) {
		return true
	}
	switch p.s[p.pos] {
	case ';', '\n', ' ', '\t':
		return true
	}
	return false
}

func (p *parser) expectEnd() error {
	if !p.atCommandEnd() {
		return fmt.Errorf("extra characters after command: %q", p.s[p.pos:])
	}
	return nil
}

func (p *parser) parseCommand() (*Command, error) {
	addr, err := p.parseAddress()
	if err != nil {
		return nil, err
	}
	p.skipSpaces()
	neg := false
	for p.pos < len(p.s) && p.s[p.pos] == '!' {
		neg = true
		p.pos++
		p.skipSpaces()
	}
	action, err := p.parseAction()
	if err != nil {
		return nil, err
	}
	return &Command{Addr: addr, Neg: neg, Action: action}, nil
}

func (p *parser) parseAddress() (Address, error) {
	a1, ok, err := p.parseSingleAddr()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	p.skipSpaces()
	if p.pos < len(p.s) && p.s[p.pos] == ',' {
		p.pos++
		p.skipSpaces()
		a2, ok2, err := p.parseSingleAddr()
		if err != nil {
			return nil, err
		}
		if !ok2 {
			return nil, fmt.Errorf("expected address after ','")
		}
		return &rangeAddr{start: a1, end: a2}, nil
	}
	return &singleAddr{ep: a1}, nil
}

func (p *parser) parseSingleAddr() (endpoint, bool, error) {
	if p.pos >= len(p.s) {
		return endpoint{}, false, nil
	}
	c := p.s[p.pos]
	switch {
	case c >= '0' && c <= '9':
		n := 0
		for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			n = n*10 + int(p.s[p.pos]-'0')
			p.pos++
		}
		return endpoint{kind: epLine, n: n}, true, nil
	case c == '$':
		p.pos++
		return endpoint{kind: epLast}, true, nil
	case c == '/':
		p.pos++
		raw, err := p.readDelimited('/')
		if err != nil {
			return endpoint{}, false, err
		}
		re, err := compileRegex(raw, "")
		if err != nil {
			return endpoint{}, false, err
		}
		return endpoint{kind: epRegex, re: re}, true, nil
	case c == '\\':
		// \cREGEXc uses a custom delimiter.
		p.pos++
		if p.pos >= len(p.s) {
			return endpoint{}, false, fmt.Errorf("unexpected end of script after '\\'")
		}
		delim := p.s[p.pos]
		p.pos++
		raw, err := p.readDelimited(delim)
		if err != nil {
			return endpoint{}, false, err
		}
		re, err := compileRegex(raw, "")
		if err != nil {
			return endpoint{}, false, err
		}
		return endpoint{kind: epRegex, re: re}, true, nil
	default:
		return endpoint{}, false, nil
	}
}

func (p *parser) parseAction() (Action, error) {
	if p.pos >= len(p.s) {
		return nil, fmt.Errorf("missing command")
	}
	c := p.s[p.pos]
	p.pos++
	switch c {
	case 's':
		return p.parseSubst()
	case 'p':
		return printCmd{}, p.expectEnd()
	case 'd':
		return deleteCmd{}, p.expectEnd()
	case 'G':
		return appendBlank{}, p.expectEnd()
	default:
		return nil, fmt.Errorf("unknown command: %q", string(c))
	}
}

func (p *parser) parseSubst() (Action, error) {
	if p.pos >= len(p.s) {
		return nil, fmt.Errorf("unterminated `s' command")
	}
	delim := p.s[p.pos]
	p.pos++
	if delim == '\\' || delim == '\n' {
		return nil, fmt.Errorf("invalid `s' delimiter %q", string(delim))
	}
	pattern, err := p.readDelimited(delim)
	if err != nil {
		return nil, err
	}
	repl, err := p.readDelimited(delim)
	if err != nil {
		return nil, err
	}

	global, printFlag := false, false
	n := 0
	var reFlags string
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		switch {
		case c == 'g':
			global = true
			p.pos++
		case c == 'p':
			printFlag = true
			p.pos++
		case c >= '0' && c <= '9':
			n = 0
			for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
				n = n*10 + int(p.s[p.pos]-'0')
				p.pos++
			}
		case c == 'i' || c == 'I':
			if !strings.Contains(reFlags, "i") {
				reFlags += "i"
			}
			p.pos++
		case c == 'm' || c == 'M':
			if !strings.Contains(reFlags, "m") {
				reFlags += "m"
			}
			p.pos++
		default:
			goto done
		}
	}
done:
	if err := p.expectEnd(); err != nil {
		return nil, err
	}
	re, err := compileRegex(pattern, reFlags)
	if err != nil {
		return nil, err
	}
	return &subst{
		re:     re,
		repl:   translateReplacement(repl),
		global: global,
		n:      n,
		print:  printFlag,
	}, nil
}

// readDelimited reads up to the next unescaped delim and consumes it.
// \<delim> becomes a plain delim; other escapes are left for the regex and
// replacement translators.
func (p *parser) readDelimited(delim byte) (string, error) {
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '\\' && p.pos+1 < len(p.s) {
			nx := p.s[p.pos+1]
			if nx == delim {
				b.WriteByte(delim)
			} else {
				b.WriteByte('\\')
				b.WriteByte(nx)
			}
			p.pos += 2
			continue
		}
		if c == delim {
			p.pos++
			return b.String(), nil
		}
		if c == '\n' {
			break // a bare newline cannot appear inside a delimited section
		}
		b.WriteByte(c)
		p.pos++
	}
	return "", fmt.Errorf("unterminated expression: missing closing %q", string(delim))
}

// compileRegex compiles a sed (BRE) pattern with optional i/m flags.
func compileRegex(bre, flags string) (*regexp.Regexp, error) {
	if bre == "" {
		return nil, fmt.Errorf("empty regular expression // is not supported")
	}
	pat := breToRE2(bre)
	if flags != "" {
		pat = "(?" + flags + ")" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", bre, err)
	}
	return re, nil
}

// breToRE2 converts a Basic Regular Expression to RE2. In BRE the escaped
// forms \( \) \{ \} \+ \? \| are special and the bare ones are literal, the
// opposite of RE2. Bracket expressions are copied as-is.
func breToRE2(p string) string {
	var b strings.Builder
	n := len(p)
	for i := 0; i < n; {
		c := p[i]
		switch c {
		case '\\':
			if i+1 >= n {
				b.WriteString(`\\`)
				i++
				continue
			}
			d := p[i+1]
			switch d {
			case '(', ')', '{', '}', '+', '?', '|':
				b.WriteByte(d)
			case '/':
				b.WriteByte('/')
			default:
				b.WriteByte('\\')
				b.WriteByte(d)
			}
			i += 2
		case '(', ')', '{', '}', '+', '?', '|':
			b.WriteByte('\\')
			b.WriteByte(c)
			i++
		case '[':
			j := i + 1
			if j < n && p[j] == '^' {
				j++
			}
			if j < n && p[j] == ']' { // a leading ] is a literal member
				j++
			}
			for j < n && p[j] != ']' {
				j++
			}
			if j < n {
				j++
			}
			b.WriteString(p[i:j])
			i = j
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// translateReplacement turns a sed replacement into a Regexp.Expand template:
// \1..\9 and & become ${1}..${9} and ${0}, $ is doubled, and \n, \t become
// real control characters.
func translateReplacement(r string) string {
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch c {
		case '\\':
			if i+1 >= len(r) {
				b.WriteByte('\\')
				continue
			}
			i++
			d := r[i]
			switch {
			case d >= '0' && d <= '9':
				b.WriteString("${")
				b.WriteByte(d)
				b.WriteByte('}')
			case d == 'n':
				b.WriteByte('\n')
			case d == 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(d) // \&, \\ and anything unknown are literal
			}
		case '&':
			b.WriteString("${0}")
		case '$':
			b.WriteString("$$")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
