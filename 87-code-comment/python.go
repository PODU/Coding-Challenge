package ccomment

// pythonStripper removes "#" comments, skipping over strings of every kind.
type pythonStripper struct{}

func (pythonStripper) Lang() string { return "python" }

func (pythonStripper) Scan(src []byte) ([]Span, error) {
	var spans []Span
	i, n := 0, len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '#':
			start := i
			for i < n && src[i] != '\n' && src[i] != '\r' {
				i++
			}
			spans = append(spans, mkLine(src, start, i))
		case c == '\'' || c == '"':
			if i+2 < n && src[i+1] == c && src[i+2] == c {
				i = pySkipTriple(src, i, c)
			} else {
				i = pySkipString(src, i, c)
			}
		default:
			i++
		}
	}
	return spans, nil
}

func (p pythonStripper) Strip(src []byte, opt Options) ([]byte, Stats, error) {
	return basicStrip(p, src, opt, pyIsPragma)
}

// pySkipString returns the index just past the string starting at i. Raw
// strings need no special case: a backslash before a quote doesn't end an
// r"..." string either.
func pySkipString(src []byte, i int, q byte) int {
	n := len(src)
	i++
	for i < n {
		c := src[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == q {
			return i + 1
		}
		if c == '\n' {
			return i // unterminated; don't run on into the next line
		}
		i++
	}
	return n
}

func pySkipTriple(src []byte, i int, q byte) int {
	n := len(src)
	i += 3
	for i < n {
		c := src[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == q && i+2 < n && src[i+1] == q && src[i+2] == q {
			return i + 3
		}
		if c == q && i+2 >= n {
			return n
		}
		i++
	}
	return n
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
