// Package matcher holds the item list and runs queries against it.
package matcher

import (
	"bufio"
	"io"
	"strings"
	"sync"

	"ccfzf/internal/item"
)

// chunkSize keeps the item list in fixed-size blocks: appending never copies
// or reallocates the whole list, so a reader can keep filling it while the
// matcher walks what is already there.
const chunkSize = 4096

type Store struct {
	mu     sync.RWMutex
	chunks [][]item.Item
	count  int
	done   bool
}

func NewStore() *Store { return &Store{} }

func (s *Store) Append(it item.Item) {
	s.mu.Lock()
	if len(s.chunks) == 0 || len(s.chunks[len(s.chunks)-1]) == chunkSize {
		s.chunks = append(s.chunks, make([]item.Item, 0, chunkSize))
	}
	last := len(s.chunks) - 1
	s.chunks[last] = append(s.chunks[last], it)
	s.count++
	s.mu.Unlock()
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

func (s *Store) Done() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

func (s *Store) SetDone() {
	s.mu.Lock()
	s.done = true
	s.mu.Unlock()
}

// At returns item i. Items are never moved once appended, so the pointer stays
// valid for the lifetime of the program.
func (s *Store) At(i int) *item.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &s.chunks[i/chunkSize][i%chunkSize]
}

// Snapshot returns the chunks holding items [0,count) without copying them.
// The caller may read them without holding the lock because appends only ever
// touch the tail.
func (s *Store) snapshot() ([][]item.Item, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	chunks := make([][]item.Item, len(s.chunks))
	copy(chunks, s.chunks)
	return chunks, s.count
}

// Read fills the store from r, one item per line (or per NUL with --read0).
// It is meant to run in its own goroutine: the interface starts drawing
// immediately and the list grows underneath it.
func Read(r io.Reader, nulDelimited bool, b *item.Builder, s *Store, onProgress func()) error {
	delim := byte('\n')
	if nulDelimited {
		delim = 0
	}
	br := bufio.NewReaderSize(r, 128*1024)
	var index int32
	const progressEvery = 4096
	for {
		line, err := br.ReadString(delim)
		if len(line) > 0 {
			line = strings.TrimSuffix(line, string(delim))
			if !nulDelimited {
				line = strings.TrimSuffix(line, "\r")
			}
			if len(line) > 0 || err == nil {
				s.Append(*b.Build(index, line))
				index++
				if onProgress != nil && index%progressEvery == 0 {
					onProgress()
				}
			}
		}
		if err != nil {
			s.SetDone()
			if onProgress != nil {
				onProgress()
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
