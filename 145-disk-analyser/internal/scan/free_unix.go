//go:build linux || darwin || freebsd

package scan

import "syscall"

// FreeSpace reports the bytes available to unprivileged users (df's "Avail")
// and the filesystem size.
func FreeSpace(path string) (free, total int64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), int64(st.Blocks) * int64(st.Bsize), nil
}
