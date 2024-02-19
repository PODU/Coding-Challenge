// Package wordlist streams candidates from a newline-delimited source, trimming
// whitespace and skipping blank lines, with memory independent of file size.
package wordlist

import (
	"bufio"
	"context"
	"io"
)

// maxLine caps a single line at 1 MiB.
const maxLine = 1 << 20

// Stream sends each word from r on the returned channel until r ends or ctx is
// cancelled. A scan error arrives on the error channel after the word channel
// closes.
func Stream(ctx context.Context, r io.Reader) (<-chan string, <-chan error) {
	out := make(chan string)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), maxLine)
		for sc.Scan() {
			word := trim(sc.Bytes())
			if word == "" {
				continue
			}
			select {
			case out <- word:
			case <-ctx.Done():
				return
			}
		}
		if err := sc.Err(); err != nil {
			errc <- err
		}
	}()
	return out, errc
}

// trim strips surrounding ASCII whitespace, including a CRLF's trailing CR.
func trim(b []byte) string {
	start := 0
	end := len(b)
	for start < end && isSpace(b[start]) {
		start++
	}
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return string(b[start:end])
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f'
}
