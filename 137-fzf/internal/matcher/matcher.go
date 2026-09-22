package matcher

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"ccfzf/internal/algo"
	"ccfzf/internal/cli"
	"ccfzf/internal/item"
	"ccfzf/internal/pattern"
)

// Match is an item that satisfied the query, with the result of matching it.
type Match struct {
	Item *item.Item
	Res  algo.Result
}

// Options are the ranking rules, fixed for the whole run.
type Options struct {
	Tiebreak []cli.Criterion
	Sort     bool
	Tac      bool
}

// Compare orders two matches. Higher score first, then the configured
// tiebreakers, then input order - so the ordering is total and stable even
// when items are matched out of order by different goroutines.
func (o Options) Compare(a, b Match) int {
	if !o.Sort {
		return o.byIndex(a, b)
	}
	if a.Res.Score != b.Res.Score {
		return b.Res.Score - a.Res.Score
	}
	for _, c := range o.Tiebreak {
		switch c {
		case cli.ByLength:
			if d := a.Item.Match().Len() - b.Item.Match().Len(); d != 0 {
				return d
			}
		case cli.ByBegin:
			if d := a.Res.Start - b.Res.Start; d != 0 {
				return d
			}
		case cli.ByEnd:
			// Prefer the match that sits closer to the end of the item.
			if d := b.Res.End - a.Res.End; d != 0 {
				return d
			}
		case cli.ByIndex:
			return o.byIndex(a, b)
		}
	}
	return o.byIndex(a, b)
}

func (o Options) byIndex(a, b Match) int {
	d := int(a.Item.Index) - int(b.Item.Index)
	if o.Tac {
		return -d
	}
	return d
}

// ForPattern adapts the ranking rules to a query: with an empty query every
// item scores zero, so the input order is the only meaningful one.
func (o Options) ForPattern(p *pattern.Pattern) Options {
	if p == nil || p.IsEmpty() {
		o.Sort = false
	}
	return o
}

// Run matches items [from,to) against p and returns the matches in ranked
// order. The work is split across all available cores; cancel is polled
// between blocks so a keystroke can abandon a search over a million items
// without waiting for it to finish.
func Run(s *Store, p *pattern.Pattern, from, to int, o Options, cancel *atomic.Bool) ([]Match, bool) {
	if to <= from {
		return nil, true
	}
	o = o.ForPattern(p)
	chunks, _ := s.snapshot()

	workers := runtime.NumCPU()
	if n := (to - from) / 2048; n < workers {
		workers = max(n, 1)
	}
	span := (to - from + workers - 1) / workers

	parts := make([][]Match, workers)
	var wg sync.WaitGroup
	for w := range workers {
		lo := from + w*span
		hi := min(lo+span, to)
		if lo >= hi {
			continue
		}
		wg.Add(1)
		go func(w, lo, hi int) {
			defer wg.Done()
			slab := algo.NewSlab()
			var out []Match
			for i := lo; i < hi; i++ {
				if i%2048 == 0 && cancel != nil && cancel.Load() {
					return
				}
				it := &chunks[i/chunkSize][i%chunkSize]
				res, _, ok := p.Match(it.Match(), false, slab)
				if ok {
					out = append(out, Match{Item: it, Res: res})
				}
			}
			parts[w] = out
		}(w, lo, hi)
	}
	wg.Wait()
	if cancel != nil && cancel.Load() {
		return nil, false
	}

	n := 0
	for _, part := range parts {
		n += len(part)
	}
	out := make([]Match, 0, n)
	for _, part := range parts {
		out = append(out, part...)
	}
	// Parts are already in input order; only a scored search needs a sort.
	if o.Sort {
		slices.SortStableFunc(out, o.Compare)
	} else if o.Tac {
		slices.Reverse(out)
	}
	return out, true
}

// MergeSorted merges two ranked lists, which is how results from newly read
// items join the list already on screen without re-matching everything.
func MergeSorted(a, b []Match, o Options) []Match {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]Match, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if o.Compare(a[i], b[j]) <= 0 {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	return append(append(out, a[i:]...), b[j:]...)
}

// Positions recomputes the matched character positions for one item. Only the
// handful of items actually on screen need them, so the search itself runs
// without tracking positions at all.
func Positions(p *pattern.Pattern, it *item.Item, slab *algo.Slab) []int {
	_, pos, ok := p.Match(it.Match(), true, slab)
	if !ok {
		return nil
	}
	if off := it.MatchOffset(); off > 0 {
		// Positions are relative to the --nth fields; shift them back onto the
		// text that is actually displayed.
		for i := range pos {
			pos[i] += off
		}
	} else if off < 0 {
		return nil // the match text is not a contiguous run of the display text
	}
	return pos
}
