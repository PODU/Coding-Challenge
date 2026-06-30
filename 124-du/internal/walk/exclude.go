package walk

import "path/filepath"

// Excluder matches --exclude globs against an entry's name or full path.
// Excluded directories are not walked at all.
type Excluder struct {
	patterns []string
}

// NewExcluder returns an Excluder for the given globs.
func NewExcluder(patterns []string) *Excluder {
	return &Excluder{patterns: patterns}
}

// Match reports whether any pattern matches name or path.
func (e *Excluder) Match(name, path string) bool {
	for _, p := range e.patterns {
		if ok, err := filepath.Match(p, name); err == nil && ok {
			return true
		}
		if ok, err := filepath.Match(p, path); err == nil && ok {
			return true
		}
	}
	return false
}
