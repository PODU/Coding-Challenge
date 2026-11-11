//go:build !windows

package scan

import (
	"os"
	"syscall"
)

// fillStat reads allocated blocks, inode and link count. st_blocks is always
// in 512-byte units, whatever the filesystem block size.
func fillStat(n *Node, _ string, fi os.FileInfo) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		n.Usage = n.Apparent
		return
	}
	n.Usage = int64(st.Blocks) * 512
	n.ID = FileID{Dev: uint64(st.Dev), Ino: uint64(st.Ino)}
	n.Links = uint32(st.Nlink)
}

// Directory sizes count towards apparent size, as with du --apparent-size.
func dirApparent(fi os.FileInfo) int64 { return fi.Size() }

func isReparsePoint(os.FileInfo) bool { return false }
