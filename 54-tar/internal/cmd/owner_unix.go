//go:build unix

package cmd

import (
	"os"
	"syscall"

	"mytar/internal/ustar"
)

// fillOwner populates UID/GID from the underlying stat on POSIX systems.
func fillOwner(h *ustar.Header, info os.FileInfo) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		h.UID = int(st.Uid)
		h.GID = int(st.Gid)
	}
}
