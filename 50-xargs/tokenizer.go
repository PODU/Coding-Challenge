package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// Mode is how input is split into items.
type Mode int

const (
	ModeWhitespace Mode = iota // blanks, with quotes and backslash escapes
	ModeNull                   // -0: NUL bytes only, no quoting
	ModeDelimiter              // -d: one given byte, no quoting
)

var errUnbalancedQuote = errors.New("ccxargs: unmatched quote; by default quotes are special to ccxargs unless you use the -0 option")

// TokenizerConfig controls how input is split into items.
type TokenizerConfig struct {
	Mode      Mode
	Delimiter byte // ModeDelimiter only
	// EOFString (-E) ends the input when seen as an item. Whitespace mode only.
	EOFString string
	EOFSet    bool
}

// Tokenizer reads items from a stream one at a time.
type Tokenizer struct {
	sc      *bufio.Scanner
	cfg     TokenizerConfig
	stopped bool
}

// NewTokenizer returns a Tokenizer reading from r.
func NewTokenizer(r io.Reader, cfg TokenizerConfig) *Tokenizer {
	sc := bufio.NewScanner(r)
	// bufio.Scanner's default 64 KiB token limit is too small for some items.
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	switch cfg.Mode {
	case ModeNull:
		sc.Split(makeByteSplit(0))
	case ModeDelimiter:
		sc.Split(makeByteSplit(cfg.Delimiter))
	default:
		sc.Split(splitWhitespace)
	}
	return &Tokenizer{sc: sc, cfg: cfg}
}

// Next returns the next item, or ok == false at the end of input (including
// reaching the -E string).
func (t *Tokenizer) Next() (item string, ok bool, err error) {
	if t.stopped {
		return "", false, nil
	}
	if !t.sc.Scan() {
		return "", false, t.sc.Err()
	}
	tok := t.sc.Text()
	if t.cfg.Mode == ModeWhitespace && t.cfg.EOFSet && tok == t.cfg.EOFString {
		t.stopped = true
		return "", false, nil
	}
	return tok, true, nil
}

// TokenizeAll reads every item. On error it also returns the items read so far.
func TokenizeAll(r io.Reader, cfg TokenizerConfig) ([]string, error) {
	t := NewTokenizer(r, cfg)
	var items []string
	for {
		it, ok, err := t.Next()
		if err != nil {
			return items, err
		}
		if !ok {
			return items, nil
		}
		items = append(items, it)
	}
}

// makeByteSplit splits on delim. Empty items between delimiters are kept, but a
// trailing delimiter doesn't produce an empty last item.
func makeByteSplit(delim byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, delim); i >= 0 {
			// data[:0] is non-nil, so an empty item is still returned.
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}

// isBlank includes '\r' so CRLF input works.
func isBlank(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// splitWhitespace splits on blanks. Single or double quotes group blanks into
// one item and a backslash escapes the next character.
func splitWhitespace(data []byte, atEOF bool) (int, []byte, error) {
	start := 0
	for start < len(data) && isBlank(data[start]) {
		start++
	}
	if start == len(data) {
		// Only blanks so far: skip them and wait for more.
		return start, nil, nil
	}

	token := []byte{} // non-nil, so '' still yields an empty item
	var quote byte    // the open quote character, or 0
	i := start
	for i < len(data) {
		c := data[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				i++
				continue
			}
			token = append(token, c)
			i++
			continue
		}
		switch {
		case isBlank(c):
			return i, token, nil
		case c == '\'' || c == '"':
			quote = c
			i++
		case c == '\\':
			if i+1 >= len(data) {
				if atEOF {
					// A backslash at the very end is kept as is.
					return i + 1, append(token, c), nil
				}
				return 0, nil, nil // need the escaped char
			}
			token = append(token, data[i+1])
			i += 2
		default:
			token = append(token, c)
			i++
		}
	}

	// Out of data in the middle of a token.
	if !atEOF {
		return 0, nil, nil
	}
	if quote != 0 {
		return 0, nil, errUnbalancedQuote
	}
	return i, token, nil
}
