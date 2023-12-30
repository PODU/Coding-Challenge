// Package tr builds the lookup tables for a tr invocation and runs input
// through them.
package tr

import (
	"bufio"
	"errors"
	"io"

	"github.com/podu2997/tr/internal/set"
)

type Config struct {
	Delete     bool
	Squeeze    bool
	Complement bool
	Truncate   bool
	Unbuffered bool
	Set1       *set.Set
	Set2       *set.Set // nil when only one SET was given
	HasSet2    bool
}

// Engine holds the lookup tables for one invocation.
type Engine struct {
	doDelete    bool
	doTranslate bool
	doSqueeze   bool
	unbuffered  bool
	delSet      [256]bool
	trMap       [256]byte
	sqzSet      [256]bool
}

func membership(bs []byte) [256]bool {
	var m [256]bool
	for _, b := range bs {
		m[b] = true
	}
	return m
}

func invert(m [256]bool) [256]bool {
	for i := range m {
		m[i] = !m[i]
	}
	return m
}

func complementBytes(bs []byte) []byte {
	m := membership(bs)
	var out []byte
	for i := 0; i < 256; i++ {
		if !m[i] {
			out = append(out, byte(i))
		}
	}
	return out
}

// Build checks cfg and builds the tables. As in GNU tr, each byte is deleted,
// then translated, then squeezed.
func Build(cfg Config) (*Engine, error) {
	e := &Engine{unbuffered: cfg.Unbuffered}
	for i := 0; i < 256; i++ {
		e.trMap[i] = byte(i)
	}

	set1bytes, err := set.Expand(cfg.Set1, -1)
	if err != nil {
		return nil, err
	}

	if cfg.Delete {
		e.doDelete = true
		m := membership(set1bytes)
		if cfg.Complement {
			m = invert(m)
		}
		e.delSet = m
	}

	doTranslate := !cfg.Delete && cfg.HasSet2
	var finalSet2 []byte
	if doTranslate {
		e.doTranslate = true
		if err := cfg.Set2.Set2ClassesOK(); err != nil {
			return nil, err
		}

		s1 := set1bytes
		if cfg.Complement {
			s1 = complementBytes(set1bytes)
		}

		set2bytes, err := set.Expand(cfg.Set2, len(s1))
		if err != nil {
			return nil, err
		}

		if cfg.Truncate {
			if len(s1) > len(set2bytes) {
				s1 = s1[:len(set2bytes)]
			}
		} else {
			if len(set2bytes) == 0 {
				return nil, errors.New("when not truncating set1, string2 must be non-empty")
			}
			// A short SET2 is padded with its last character.
			last := set2bytes[len(set2bytes)-1]
			for len(set2bytes) < len(s1) {
				set2bytes = append(set2bytes, last)
			}
		}
		if len(set2bytes) > len(s1) {
			set2bytes = set2bytes[:len(s1)]
		}

		// If a byte appears more than once in SET1, its first mapping wins.
		var seen [256]bool
		for i, c := range s1 {
			if !seen[c] {
				e.trMap[c] = set2bytes[i]
				seen[c] = true
			}
		}
		finalSet2 = set2bytes
	}

	if cfg.Squeeze {
		e.doSqueeze = true
		switch {
		case doTranslate:
			// Squeezing looks at the translated bytes, i.e. SET2.
			e.sqzSet = membership(finalSet2)
		case cfg.HasSet2:
			// -d -s: delete SET1, squeeze SET2.
			s2b, err := set.Expand(cfg.Set2, len(set1bytes))
			if err != nil {
				return nil, err
			}
			e.sqzSet = membership(s2b)
		default:
			m := membership(set1bytes)
			if cfg.Complement {
				m = invert(m)
			}
			e.sqzSet = m
		}
	}

	return e, nil
}

// Run copies r to w through the tables. lastEmitted lives outside the read
// loop so a run split across two reads still gets squeezed.
func (e *Engine) Run(r io.Reader, w io.Writer) error {
	br := bufio.NewReaderSize(r, 64*1024)
	bw := bufio.NewWriterSize(w, 64*1024)
	lastEmitted := -1
	buf := make([]byte, 64*1024)
	for {
		n, rerr := br.Read(buf)
		for i := 0; i < n; i++ {
			b := buf[i]
			if e.doDelete && e.delSet[b] {
				continue
			}
			if e.doTranslate {
				b = e.trMap[b]
			}
			if e.doSqueeze && e.sqzSet[b] && int(b) == lastEmitted {
				continue
			}
			if err := bw.WriteByte(b); err != nil {
				return err
			}
			lastEmitted = int(b)
			if e.unbuffered {
				if err := bw.Flush(); err != nil {
					return err
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = bw.Flush()
			return rerr
		}
	}
	return bw.Flush()
}
