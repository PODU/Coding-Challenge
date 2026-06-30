package walk

import (
	"os"

	"du/internal/platform"
)

type devIno struct {
	dev, ino int64
}

// Sizer works out how much each file adds to its directory's total. A file
// with several hard links is only counted the first time its inode is seen,
// across the whole run.
//
// With --apparent-size directories count as 0, since a directory's own
// apparent size depends on the filesystem and means little here.
type Sizer struct {
	apparent bool
	inodes   bool
	seen     map[devIno]bool
}

// NewSizer returns a Sizer with nothing seen yet.
func NewSizer(apparent, inodes bool) *Sizer {
	return &Sizer{apparent: apparent, inodes: inodes, seen: make(map[devIno]bool)}
}

// Size returns what info adds to its parent's total.
func (s *Sizer) Size(info os.FileInfo) int64 {
	m := platform.Stat(info)

	if !info.IsDir() && m.Nlink > 1 {
		k := devIno{m.Dev, m.Ino}
		if s.seen[k] {
			return 0
		}
		s.seen[k] = true
	}

	switch {
	case s.inodes:
		return 1
	case s.apparent && info.IsDir():
		return 0
	case s.apparent:
		return m.Size
	default:
		return m.Allocated
	}
}
