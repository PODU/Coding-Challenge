// Package parser is the syntactic layer: it turns the scanner's lines into a
// tree of value.Value nodes, handling block and flow collections, block
// scalars, quoting, comments and multi-document streams.
package parser

import (
	"fmt"
	"strings"

	"yamlcc/internal/scanner"
	"yamlcc/internal/value"
)

// parseError is a located failure. It is panicked and recovered at the package
// boundary so the recursive descent doesn't thread errors everywhere.
type parseError struct {
	Line int
	Col  int
	Msg  string
}

func (e parseError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Line, e.Col, e.Msg)
}

type parser struct {
	lines  []scanner.Line
	pos    int
	strict bool
}

// Parse scans src and parses it into one value.Value per YAML document.
func Parse(src []byte, strict bool) (docs []value.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			if pe, ok := r.(parseError); ok {
				err = pe
				return
			}
			panic(r)
		}
	}()
	p := &parser{lines: scanner.SplitLines(src), strict: strict}
	docs = p.parseStream()
	return docs, nil
}

func (p *parser) eof() bool         { return p.pos >= len(p.lines) }
func (p *parser) cur() scanner.Line { return p.lines[p.pos] }
func (p *parser) errAt(l scanner.Line, format string, a ...interface{}) {
	panic(parseError{Line: l.Num, Col: l.Indent + 1, Msg: fmt.Sprintf(format, a...)})
}

func (p *parser) checkTab(l scanner.Line) {
	if l.HasTabIndent {
		p.errAt(l, "tabs are not allowed for indentation")
	}
}

func (p *parser) skipIgnorable() {
	for !p.eof() {
		l := p.cur()
		if l.Blank() || l.Comment() {
			p.pos++
			continue
		}
		break
	}
}

func (p *parser) skipDirectives() {
	for !p.eof() {
		l := p.cur()
		if l.Indent == 0 && strings.HasPrefix(l.Text, "%") {
			if p.strict {
				p.errAt(l, "directives are not allowed in --strict mode")
			}
			p.pos++
			continue
		}
		break
	}
}

func isDocStart(l scanner.Line) bool {
	return l.Indent == 0 && (l.Text == "---" || strings.HasPrefix(l.Text, "--- "))
}

func isDocEnd(l scanner.Line) bool {
	return l.Indent == 0 && (l.Text == "..." || strings.HasPrefix(l.Text, "... "))
}

func (p *parser) parseStream() []value.Value {
	var docs []value.Value
	allowBare := true
	for {
		p.skipDirectives()
		p.skipIgnorable()
		if p.eof() {
			break
		}
		line := p.cur()

		if isDocEnd(line) {
			p.pos++
			allowBare = true
			continue
		}

		if isDocStart(line) {
			inline := stripComment(strings.TrimSpace(line.Text[3:]))
			if inline != "" {
				p.lines[p.pos].Indent = 0
				p.lines[p.pos].Text = inline
				docs = append(docs, p.parseNode(0))
			} else {
				p.pos++ // consume the --- marker; body (if any) follows
				p.skipDirectives()
				p.skipIgnorable()
				if p.eof() || isDocStart(p.cur()) || isDocEnd(p.cur()) {
					docs = append(docs, value.NullValue())
				} else {
					docs = append(docs, p.parseNode(p.cur().Indent))
				}
			}
			allowBare = false
			continue
		}

		// Bare document (no explicit --- marker).
		if line.Indent != 0 {
			p.errAt(line, "bad indentation: line is not aligned with any block")
		}
		if !allowBare {
			p.errAt(line, "missing '---' separator between documents")
		}
		docs = append(docs, p.parseNode(line.Indent))
		allowBare = false
	}
	return docs
}

// parseNode parses the node whose first line is the current line, which is
// assumed to sit at column `indent`.
func (p *parser) parseNode(indent int) value.Value {
	line := p.cur()
	p.checkTab(line)
	text := line.Text

	if len(text) > 0 && (text[0] == '[' || text[0] == '{') {
		return p.parseFlowAt(p.pos, text)
	}
	if isBlockHeader(text) {
		p.pos++
		return value.StrValue(p.readBlockScalar(indent, text))
	}
	if isDashStart(text) {
		return p.parseSequence(indent)
	}
	_, _, isMap, err := splitKeyValue(text)
	if err != nil {
		p.errAt(line, "%s", err.Error())
	}
	if isMap {
		return p.parseMapping(indent)
	}
	// Single-line scalar node.
	p.pos++
	return p.scalarValue(line, text)
}

func (p *parser) parseMapping(indent int) value.Value {
	var items []value.KV
	for {
		p.skipIgnorable()
		if p.eof() {
			break
		}
		line := p.cur()
		if line.Indent != indent {
			break
		}
		p.checkTab(line)
		if isDashStart(line.Text) {
			break
		}
		key, vtext, isMap, err := splitKeyValue(line.Text)
		if err != nil {
			p.errAt(line, "%s", err.Error())
		}
		if !isMap {
			break
		}
		keyIdx := p.pos

		var v value.Value
		if vtext != "" {
			v = p.inlineValue(keyIdx, indent, vtext)
		} else {
			p.pos = keyIdx + 1
			p.skipIgnorable()
			switch {
			case !p.eof() && p.cur().Indent > indent:
				v = p.parseNode(p.cur().Indent)
			case !p.eof() && p.cur().Indent == indent && isDashStart(p.cur().Text):
				// A sequence under the key, not indented past it.
				v = p.parseSequence(indent)
			default:
				v = value.NullValue()
			}
		}
		items = append(items, value.KV{Key: key, Val: v})
	}
	return value.MapValue(items)
}

// inlineValue parses a value on the same line as its key. keyIdx is the key's
// line index.
func (p *parser) inlineValue(keyIdx, indent int, vtext string) value.Value {
	if isBlockHeader(vtext) {
		p.pos = keyIdx + 1
		return value.StrValue(p.readBlockScalar(indent, vtext))
	}
	if vtext[0] == '[' || vtext[0] == '{' {
		return p.parseFlowAt(keyIdx, vtext)
	}
	p.pos = keyIdx + 1
	return p.scalarValue(p.lines[keyIdx], vtext)
}

func (p *parser) parseSequence(indent int) value.Value {
	items := []value.Value{}
	for {
		p.skipIgnorable()
		if p.eof() {
			break
		}
		line := p.cur()
		if line.Indent != indent || !isDashStart(line.Text) {
			break
		}
		p.checkTab(line)

		rest := line.Text[1:] // after the '-'
		nsp := leadingSpaces(rest)
		content := rest[nsp:]

		if content == "" {
			// The value is on the following, more indented, lines.
			p.pos++
			p.skipIgnorable()
			if !p.eof() && p.cur().Indent > indent {
				items = append(items, p.parseNode(p.cur().Indent))
			} else {
				items = append(items, value.NullValue())
			}
			continue
		}

		// Make the text after the dash look like a normal node at its own
		// indentation, then parse it as one.
		itemIndent := indent + 1 + nsp
		p.lines[p.pos].Indent = itemIndent
		p.lines[p.pos].Text = content
		items = append(items, p.parseNode(itemIndent))
	}
	return value.SeqValue(items)
}

// scalarValue resolves a one-line scalar. Plain scalars get implicit typing;
// quoted scalars are always strings.
func (p *parser) scalarValue(line scanner.Line, text string) value.Value {
	text = strings.TrimLeft(text, " ")
	if text == "" {
		return value.NullValue()
	}
	switch text[0] {
	case '"':
		s, ni, err := scanner.ReadDoubleQuoted(text, 0)
		if err != nil {
			p.errAt(line, "%s", err.Error())
		}
		p.requireTrailingEmpty(line, text[ni:])
		return value.StrValue(s)
	case '\'':
		s, ni, err := scanner.ReadSingleQuoted(text, 0)
		if err != nil {
			p.errAt(line, "%s", err.Error())
		}
		p.requireTrailingEmpty(line, text[ni:])
		return value.StrValue(s)
	case '&', '*':
		p.errAt(line, "unsupported: anchors and aliases (&/*) are not implemented")
	case '!':
		p.errAt(line, "unsupported: tags (!) are not implemented")
	}
	plain := strings.TrimSpace(stripComment(text))
	return value.ResolvePlain(plain)
}

func (p *parser) requireTrailingEmpty(line scanner.Line, rest string) {
	rest = strings.TrimSpace(rest)
	if rest != "" && !strings.HasPrefix(rest, "#") {
		p.errAt(line, "unexpected text after quoted scalar: %q", rest)
	}
}

func isBlockHeader(text string) bool {
	if len(text) == 0 || (text[0] != '|' && text[0] != '>') {
		return false
	}
	tok := text
	if sp := strings.IndexByte(text, ' '); sp >= 0 {
		tok = text[:sp]
		after := strings.TrimSpace(text[sp:])
		if after != "" && !strings.HasPrefix(after, "#") {
			return false
		}
	}
	for _, c := range tok[1:] {
		if c != '+' && c != '-' && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// readBlockScalar reads a literal (|) or folded (>) block scalar. parentIndent
// is the indentation of the owning node; the cursor is on the first content
// line.
func (p *parser) readBlockScalar(parentIndent int, header string) string {
	tok := header
	if sp := strings.IndexByte(header, ' '); sp >= 0 {
		tok = header[:sp]
	}
	folded := tok[0] == '>'
	var chomp byte // 0 clip (default), '-' strip, '+' keep
	explicitIndent := 0
	for k := 1; k < len(tok); k++ {
		c := tok[k]
		switch {
		case c == '+' || c == '-':
			chomp = c
		case c >= '1' && c <= '9':
			explicitIndent = int(c - '0')
		}
	}

	var raws []string
	var indents []int
	for !p.eof() {
		l := p.cur()
		if l.Text == "" {
			raws = append(raws, "")
			indents = append(indents, -1)
			p.pos++
			continue
		}
		if l.Indent > parentIndent {
			raws = append(raws, l.Raw())
			indents = append(indents, l.Indent)
			p.pos++
			continue
		}
		break
	}

	// Trailing blanks only matter for the keep ('+') chomp.
	trailingBlanks := 0
	for len(raws) > 0 && raws[len(raws)-1] == "" {
		raws = raws[:len(raws)-1]
		indents = indents[:len(indents)-1]
		trailingBlanks++
	}

	if len(raws) == 0 {
		if chomp == '+' {
			return strings.Repeat("\n", trailingBlanks)
		}
		return ""
	}

	blockIndent := parentIndent + explicitIndent
	if explicitIndent == 0 {
		for i := range raws {
			if raws[i] != "" {
				blockIndent = indents[i]
				break
			}
		}
	}

	lines := make([]string, len(raws))
	for i, r := range raws {
		if r == "" {
			lines[i] = ""
			continue
		}
		if len(r) >= blockIndent {
			lines[i] = r[blockIndent:]
		} else {
			lines[i] = ""
		}
	}

	var body string
	if folded {
		body = foldLines(lines)
	} else {
		body = strings.Join(lines, "\n")
	}

	switch chomp {
	case '-': // strip
		return body
	case '+': // keep
		return body + "\n" + strings.Repeat("\n", trailingBlanks)
	default: // clip
		return body + "\n"
	}
}

func foldLines(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i == 0 {
			b.WriteString(l)
			continue
		}
		switch {
		case l == "":
			b.WriteByte('\n')
		case lines[i-1] == "":
			b.WriteString(l)
		case isMoreIndented(l) || isMoreIndented(lines[i-1]):
			b.WriteByte('\n')
			b.WriteString(l)
		default:
			b.WriteByte(' ')
			b.WriteString(l)
		}
	}
	return b.String()
}

func isMoreIndented(l string) bool { return len(l) > 0 && l[0] == ' ' }

// parseFlowAt collects a balanced flow collection, which may span lines, from
// startIdx and parses it.
func (p *parser) parseFlowAt(startIdx int, startText string) value.Value {
	var sb strings.Builder
	sb.WriteString(startText)
	idx := startIdx
	for !scanBalanced(sb.String()) {
		idx++
		if idx >= len(p.lines) {
			p.errAt(p.lines[startIdx], "unterminated flow collection")
		}
		sb.WriteByte('\n')
		sb.WriteString(p.lines[idx].Text)
	}
	p.pos = idx + 1

	fp := &flowParser{s: sb.String()}
	v, err := fp.parse()
	if err != nil {
		p.errAt(p.lines[startIdx], "%s", err.Error())
	}
	return v
}

// scanBalanced reports whether s is a complete flow collection. An open
// bracket or quote means more lines are needed.
func scanBalanced(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			_, ni, err := scanner.ReadSingleQuoted(s, i)
			if err != nil {
				return false
			}
			i = ni - 1
		case '"':
			_, ni, err := scanner.ReadDoubleQuoted(s, i)
			if err != nil {
				return false
			}
			i = ni - 1
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
	}
	return depth <= 0
}

type flowParser struct {
	s string
	i int
}

func (f *flowParser) parse() (value.Value, error) {
	f.skipWS()
	v, err := f.node()
	if err != nil {
		return value.Value{}, err
	}
	return v, nil
}

func (f *flowParser) skipWS() {
	for f.i < len(f.s) {
		c := f.s[f.i]
		if c == ' ' || c == '\t' || c == '\n' {
			f.i++
			continue
		}
		break
	}
}

func (f *flowParser) peek() byte {
	if f.i < len(f.s) {
		return f.s[f.i]
	}
	return 0
}

func (f *flowParser) node() (value.Value, error) {
	f.skipWS()
	if f.i >= len(f.s) {
		return value.Value{}, fmt.Errorf("unexpected end of flow node")
	}
	switch f.s[f.i] {
	case '[':
		return f.seq()
	case '{':
		return f.mapping()
	case '"':
		s, ni, err := scanner.ReadDoubleQuoted(f.s, f.i)
		if err != nil {
			return value.Value{}, err
		}
		f.i = ni
		return value.StrValue(s), nil
	case '\'':
		s, ni, err := scanner.ReadSingleQuoted(f.s, f.i)
		if err != nil {
			return value.Value{}, err
		}
		f.i = ni
		return value.StrValue(s), nil
	default:
		return f.plain(), nil
	}
}

func (f *flowParser) seq() (value.Value, error) {
	f.i++
	items := []value.Value{}
	f.skipWS()
	if f.peek() == ']' {
		f.i++
		return value.SeqValue(items), nil
	}
	for {
		v, err := f.node()
		if err != nil {
			return value.Value{}, err
		}
		items = append(items, v)
		f.skipWS()
		switch f.peek() {
		case ',':
			f.i++
			f.skipWS()
			if f.peek() == ']' {
				f.i++
				return value.SeqValue(items), nil
			}
		case ']':
			f.i++
			return value.SeqValue(items), nil
		default:
			return value.Value{}, fmt.Errorf("expected ',' or ']' in flow sequence")
		}
	}
}

func (f *flowParser) mapping() (value.Value, error) {
	f.i++
	items := []value.KV{}
	f.skipWS()
	if f.peek() == '}' {
		f.i++
		return value.MapValue(items), nil
	}
	for {
		key, err := f.key()
		if err != nil {
			return value.Value{}, err
		}
		f.skipWS()
		if f.peek() != ':' {
			return value.Value{}, fmt.Errorf("expected ':' in flow mapping")
		}
		f.i++
		v, err := f.node()
		if err != nil {
			return value.Value{}, err
		}
		items = append(items, value.KV{Key: key, Val: v})
		f.skipWS()
		switch f.peek() {
		case ',':
			f.i++
			f.skipWS()
			if f.peek() == '}' {
				f.i++
				return value.MapValue(items), nil
			}
		case '}':
			f.i++
			return value.MapValue(items), nil
		default:
			return value.Value{}, fmt.Errorf("expected ',' or '}' in flow mapping")
		}
	}
}

func (f *flowParser) key() (string, error) {
	f.skipWS()
	switch f.peek() {
	case '"':
		s, ni, err := scanner.ReadDoubleQuoted(f.s, f.i)
		if err != nil {
			return "", err
		}
		f.i = ni
		return s, nil
	case '\'':
		s, ni, err := scanner.ReadSingleQuoted(f.s, f.i)
		if err != nil {
			return "", err
		}
		f.i = ni
		return s, nil
	}
	start := f.i
	for f.i < len(f.s) {
		c := f.s[f.i]
		if c == ':' || c == ',' || c == '}' {
			break
		}
		f.i++
	}
	return strings.TrimSpace(f.s[start:f.i]), nil
}

func (f *flowParser) plain() value.Value {
	start := f.i
	for f.i < len(f.s) {
		c := f.s[f.i]
		if c == ',' || c == ']' || c == '}' {
			break
		}
		f.i++
	}
	return value.ResolvePlain(strings.TrimSpace(f.s[start:f.i]))
}

func isDashStart(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

// stripComment drops a trailing "#" comment. A '#' not preceded by whitespace,
// as in a URL, is kept.
func stripComment(s string) string {
	for k := 0; k < len(s); k++ {
		if s[k] == '#' && (k == 0 || s[k-1] == ' ' || s[k-1] == '\t') {
			return strings.TrimRight(s[:k], " \t")
		}
	}
	return s
}

// splitKeyValue decides whether text is a "key: value" entry, returning the
// decoded key, the value text, and whether it is one. Quotes and brackets are
// skipped when looking for the separating colon.
func splitKeyValue(text string) (key string, val string, isMap bool, err error) {
	if text == "" {
		return "", "", false, nil
	}
	if text[0] == '"' || text[0] == '\'' {
		var k string
		var ni int
		if text[0] == '"' {
			k, ni, err = scanner.ReadDoubleQuoted(text, 0)
		} else {
			k, ni, err = scanner.ReadSingleQuoted(text, 0)
		}
		if err != nil {
			return "", "", false, err
		}
		j := ni
		for j < len(text) && text[j] == ' ' {
			j++
		}
		if j < len(text) && text[j] == ':' && (j+1 == len(text) || text[j+1] == ' ') {
			return k, strings.TrimSpace(text[j+1:]), true, nil
		}
		return "", "", false, nil // a quoted scalar, not a key
	}

	depth := 0
	for j := 0; j < len(text); j++ {
		switch text[j] {
		case '\'':
			_, nj, e := scanner.ReadSingleQuoted(text, j)
			if e != nil {
				return "", "", false, e
			}
			j = nj - 1
		case '"':
			_, nj, e := scanner.ReadDoubleQuoted(text, j)
			if e != nil {
				return "", "", false, e
			}
			j = nj - 1
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case '#':
			if depth == 0 && j > 0 && text[j-1] == ' ' {
				return "", "", false, nil
			}
		case ':':
			if depth == 0 && (j+1 == len(text) || text[j+1] == ' ') {
				return strings.TrimSpace(text[:j]), strings.TrimSpace(text[j+1:]), true, nil
			}
		}
	}
	return "", "", false, nil
}
