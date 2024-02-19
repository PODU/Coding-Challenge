// Package rainbow is a precomputed lookup table, one "<hexdigest>\t<plaintext>"
// line each. It isn't a real reduction-function rainbow table, just disk traded
// for instant lookups against fast unsalted hashes.
package rainbow

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"passcrack/hashreg"
	"passcrack/saltutil"
)

const maxLine = 1 << 20

// Build hashes each word (through salter) and writes a "<digest>\t<word>" line
// to w, returning how many it wrote.
func Build(ctx context.Context, words io.Reader, w io.Writer, algo hashreg.Algo, salter saltutil.Salter) (int, error) {
	sc := bufio.NewScanner(words)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	bw := bufio.NewWriter(w)
	count := 0
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		word := strings.TrimRight(sc.Text(), "\r")
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		digest := hashreg.HexSum(algo, salter.Apply(word))
		if _, err := fmt.Fprintf(bw, "%s\t%s\n", digest, word); err != nil {
			return count, err
		}
		count++
	}
	if err := sc.Err(); err != nil {
		return count, err
	}
	return count, bw.Flush()
}

// Load reads a Build table into a digest->plaintext map.
func Load(r io.Reader) (map[string]string, error) {
	table := make(map[string]string)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		digest, plain, ok := strings.Cut(line, "\t")
		if !ok {
			continue // ignore malformed lines
		}
		digest = strings.ToLower(strings.TrimSpace(digest))
		if _, exists := table[digest]; !exists {
			table[digest] = plain
		}
	}
	return table, sc.Err()
}

// Lookup loads the table and resolves targets, returning the matches and the
// digests it couldn't find.
func Lookup(r io.Reader, targets []string) (map[string]string, []string, error) {
	table, err := Load(r)
	if err != nil {
		return nil, nil, err
	}
	cracked := make(map[string]string)
	var miss []string
	for _, t := range targets {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm == "" {
			continue
		}
		if pt, ok := table[norm]; ok {
			cracked[norm] = pt
		} else {
			miss = append(miss, norm)
		}
	}
	return cracked, miss, nil
}
