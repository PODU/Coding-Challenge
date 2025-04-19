package ccomment

// backtick mode for the C-family scanner.
const (
	btNone     = 0 // backtick is ordinary code (C/C++)
	btTemplate = 1 // JS template literal `...${expr}...`
	btRaw      = 2 // Go raw string `...` (no escapes)
)

// cConfig parameterizes the shared C-family scanner for JS/TS, C, C++ and Go.
type cConfig struct {
	name     string
	regex    bool // JS regex literals
	lineCont bool // backslash at EOL continues a // comment (C/C++)
	backtick int  // btNone | btTemplate | btRaw
	pragma   func(Span) bool
}

// cStripper is the shared C-family backend.
type cStripper struct{ cfg cConfig }

func (c cStripper) Lang() string { return c.cfg.name }

func (c cStripper) Strip(src []byte, opt Options) ([]byte, Stats, error) {
	return basicStrip(c, src, opt, c.cfg.pragma)
}

func (c cStripper) Scan(src []byte) ([]Span, error) {
	cfg := c.cfg
	var spans []Span
	i, n := 0, len(src)
	var lastSig byte // last significant (non-space) code byte
	var word, lastWord string

	flush := func() {
		if word != "" {
			lastWord = word
			word = ""
		}
	}

	for i < n {
		ch := src[i]
		switch {
		case ch == '/' && i+1 < n && src[i+1] == '/':
			flush()
			start := i
			i += 2
			for i < n {
				if src[i] == '\n' {
					if cfg.lineCont && cBackslashBeforeNL(src, i) {
						i++
						continue
					}
					break
				}
				if src[i] == '\r' {
					break
				}
				i++
			}
			spans = append(spans, mkLine(src, start, i))
			lastSig = 0

		case ch == '/' && i+1 < n && src[i+1] == '*':
			flush()
			start := i
			i += 2
			closed := false
			for i+1 < n {
				if src[i] == '*' && src[i+1] == '/' {
					i += 2
					closed = true
					break
				}
				i++
			}
			if !closed {
				return spans, &UnterminatedError{Offset: start}
			}
			spans = append(spans, mkBlock(src, start, i))
			lastSig = 0

		case ch == '"':
			flush()
			i = cSkipQuoted(src, i, '"')
			lastSig = '"'

		case ch == '\'':
			flush()
			i = cSkipQuoted(src, i, '\'')
			lastSig = '\''

		case cfg.backtick == btRaw && ch == '`':
			flush()
			i++
			for i < n && src[i] != '`' {
				i++
			}
			if i < n {
				i++
			}
			lastSig = '`'

		case cfg.backtick == btTemplate && ch == '`':
			flush()
			i = cSkipTemplate(src, i)
			lastSig = '`'

		case cfg.regex && ch == '/':
			flush()
			if regexAllowed(lastSig, lastWord) {
				i = cSkipRegex(src, i)
				lastSig = '/'
			} else {
				lastSig = '/'
				i++
			}

		default:
			if isIdentByte(ch) {
				word += string(ch)
			} else if word != "" {
				lastWord = word
				word = ""
			}
			if !isHSpace(ch) && ch != '\n' {
				lastSig = ch
			}
			i++
		}
	}
	return spans, nil
}

func cBackslashBeforeNL(src []byte, nl int) bool {
	j := nl - 1
	if j >= 0 && src[j] == '\r' {
		j--
	}
	return j >= 0 && src[j] == '\\'
}

func cSkipQuoted(src []byte, i int, q byte) int {
	n := len(src)
	i++ // opening quote
	for i < n {
		switch src[i] {
		case '\\':
			i += 2
			continue
		case q:
			return i + 1
		case '\n':
			return i // unterminated; do not swallow the newline
		}
		i++
	}
	return n
}

// cSkipTemplate scans a JS template literal, descending into ${...}
// interpolations with brace tracking.
func cSkipTemplate(src []byte, i int) int {
	n := len(src)
	i++ // opening backtick
	for i < n {
		switch {
		case src[i] == '\\':
			i += 2
		case src[i] == '`':
			return i + 1
		case i+1 < n && src[i] == '$' && src[i+1] == '{':
			i += 2
			depth := 1
			for i < n && depth > 0 {
				switch src[i] {
				case '\\':
					i += 2
					continue
				case '{':
					depth++
				case '}':
					depth--
				case '`':
					i = cSkipTemplate(src, i)
					continue
				case '"':
					i = cSkipQuoted(src, i, '"')
					continue
				case '\'':
					i = cSkipQuoted(src, i, '\'')
					continue
				}
				i++
			}
		default:
			i++
		}
	}
	return n
}

// cSkipRegex scans a JS regex literal /.../flags, honoring escapes and
// character classes.
func cSkipRegex(src []byte, i int) int {
	n := len(src)
	i++ // opening slash
	inClass := false
	for i < n {
		switch src[i] {
		case '\\':
			i += 2
			continue
		case '\n':
			return i // invalid regex; bail without consuming the newline
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				i++
				for i < n && isLetter(src[i]) {
					i++
				}
				return i
			}
		}
		i++
	}
	return n
}

// regexAllowed decides whether a "/" begins a regex literal based on the
// previous significant token. This is heuristic, as the language permits.
func regexAllowed(lastSig byte, lastWord string) bool {
	switch {
	case lastSig == 0:
		return true
	case isValueEnder(lastSig):
		// An identifier just ended; regex follows only after certain keywords.
		if (isIdentByte(lastSig)) && regexKeyword(lastWord) {
			return true
		}
		return false
	default:
		return true // operator/punctuation: regex allowed
	}
}

func isValueEnder(b byte) bool {
	return isIdentByte(b) || b == ')' || b == ']' || b == '}' ||
		b == '"' || b == '\'' || b == '`'
}

func isIdentByte(b byte) bool {
	return isLetter(b) || isDigit(b) || b == '_' || b == '$'
}

func regexKeyword(w string) bool {
	switch w {
	case "return", "typeof", "instanceof", "in", "of", "new", "delete",
		"void", "do", "else", "yield", "await", "case", "throw":
		return true
	}
	return false
}
