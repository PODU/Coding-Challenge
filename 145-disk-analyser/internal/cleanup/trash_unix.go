//go:build !windows

package cleanup

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func devOf(path string) (uint64, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no device information")
	}
	return uint64(st.Dev), nil
}

// mountPoint walks up from path while the device stays the same.
func mountPoint(path string) (string, error) {
	dev, err := devOf(path)
	if err != nil {
		return "", err
	}
	cur := path
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return cur, nil
		}
		d, err := devOf(parent)
		if err != nil || d != dev {
			return cur, nil
		}
		cur = parent
	}
}

// sameDevice reports whether a and b (or b's nearest existing ancestor) are
// on one filesystem, so a rename between them can work.
func sameDevice(a, b string) bool {
	da, err := devOf(a)
	if err != nil {
		return false
	}
	for {
		if db, err := devOf(b); err == nil {
			return da == db
		}
		parent := filepath.Dir(b)
		if parent == b {
			return false
		}
		b = parent
	}
}
