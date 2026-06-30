//go:build !windows

package platform

import (
	"os"
	"syscall"
)

// Stat returns the metadata for info. st_blocks counts 512-byte units
// regardless of the filesystem block size.
func Stat(info os.FileInfo) Meta {
	m := Meta{Size: info.Size(), Nlink: 1, Allocated: roundUp(info.Size(), 512)}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		m.Allocated = int64(st.Blocks) * 512
		m.Dev = int64(st.Dev)
		m.Ino = int64(st.Ino)
		m.Nlink = int64(st.Nlink)
	}
	return m
}
