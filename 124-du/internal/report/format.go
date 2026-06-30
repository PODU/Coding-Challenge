package report

import (
	"fmt"

	"du/internal/walk"
)

// FormatSize formats size in the unit chosen in opts, rounding up: a partly
// used block still counts as a block. Inode counts are printed as is.
func FormatSize(size int64, opts walk.Options) string {
	if opts.Inodes {
		return fmt.Sprintf("%d", size)
	}
	switch opts.Unit {
	case walk.UnitHuman:
		return Human(size)
	case walk.UnitK:
		return fmt.Sprintf("%d", ceilDiv(size, 1024))
	case walk.UnitM:
		return fmt.Sprintf("%d", ceilDiv(size, 1024*1024))
	case walk.UnitCustom:
		bs := opts.BlockSize
		if bs <= 0 {
			bs = 1
		}
		return fmt.Sprintf("%d", ceilDiv(size, bs))
	default: // UnitBlock
		return fmt.Sprintf("%d", ceilDiv(size, 512))
	}
}

func ceilDiv(n, d int64) int64 {
	if d <= 0 {
		return n
	}
	if n <= 0 {
		return 0
	}
	return (n + d - 1) / d
}

// Human formats n like GNU du -h: a plain number below 1024, otherwise a
// K/M/G... suffix with one decimal under 10 (1.5K) and none above (15K).
func Human(n int64) string {
	if n < 0 {
		n = 0
	}
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	units := []string{"K", "M", "G", "T", "P", "E"}
	f := float64(n)
	i := -1
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if f < 9.95 {
		return fmt.Sprintf("%.1f%s", f, units[i])
	}
	return fmt.Sprintf("%.0f%s", f, units[i])
}
