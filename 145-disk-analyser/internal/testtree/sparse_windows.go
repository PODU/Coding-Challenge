//go:build windows

package testtree

import (
	"os"
	"syscall"
)

const fsctlSetSparse = 0x000900C4

// makeSparse marks the file sparse before extending it. Without the flag,
// NTFS allocates the whole length.
func makeSparse(path string, size int64) (bool, error) {
	f, err := os.Create(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var n uint32
	sparse := syscall.DeviceIoControl(syscall.Handle(f.Fd()), fsctlSetSparse, nil, 0, nil, 0, &n, nil) == nil
	return sparse, f.Truncate(size)
}
