//go:build !linux && !darwin && !freebsd && !windows

package scan

import "errors"

func FreeSpace(string) (int64, int64, error) {
	return 0, 0, errors.New("free space is not available on this platform")
}
