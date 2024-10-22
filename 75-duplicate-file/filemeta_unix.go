//go:build !windows

package main

import (
	"os"
	"syscall"
)

// devIno returns the device and inode numbers, so hardlinks to the same file
// can be recognised.
func devIno(info os.FileInfo) (uint64, uint64) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev), uint64(st.Ino)
	}
	return 0, 0
}
