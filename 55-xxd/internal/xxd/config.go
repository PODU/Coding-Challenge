// Package xxd implements hex dumping and its reverse, independent of the CLI.
package xxd

import (
	"errors"
	"fmt"
)

// Config controls a Dump or Reverse. The zero value is not valid; start from
// NewConfig.
type Config struct {
	Cols  int // bytes per line
	Group int // bytes per hex group; 0 means no grouping

	Length int64 // max bytes to read, or -1 for no limit
	Seek   int64 // starting offset; printed offsets start here too

	LittleEndian bool // -e: reverse bytes within each group
	Plain        bool // -p: bare hex, no offset or ASCII column
	Reverse      bool // -r
}

// Default bytes per line, normally and with -p.
const (
	DefaultCols      = 16
	DefaultPlainCols = 30
)

// NewConfig returns a Config populated with xxd's defaults.
func NewConfig() Config {
	return Config{
		Cols:   DefaultCols,
		Group:  2,
		Length: -1,
	}
}

// Group sizes xxd accepts together with -e.
var validEndianGroups = map[int]bool{1: true, 2: true, 4: true, 8: true, 16: true}

// Validate rejects option combinations the real xxd also rejects.
func (c Config) Validate() error {
	if c.Cols <= 0 {
		return fmt.Errorf("invalid number of columns: %d (must be > 0)", c.Cols)
	}
	if c.Group < 0 {
		return fmt.Errorf("invalid group size: %d (must be >= 0)", c.Group)
	}
	if c.Length < -1 {
		return fmt.Errorf("invalid length: %d", c.Length)
	}
	if c.Seek < 0 {
		return errors.New("negative seek offsets are not supported")
	}
	if c.LittleEndian {
		if c.Plain {
			return errors.New("-e is incompatible with -p")
		}
		if c.Reverse {
			return errors.New("-e is incompatible with -r")
		}
		if c.Group != 0 && !validEndianGroups[c.Group] {
			return errors.New("number of octets per group must be a power of 2 with -e")
		}
	}
	return nil
}

// effectiveGroup treats "no grouping" as one group spanning the line.
func (c Config) effectiveGroup() int {
	if c.Group <= 0 || c.Group > c.Cols {
		return c.Cols
	}
	return c.Group
}
