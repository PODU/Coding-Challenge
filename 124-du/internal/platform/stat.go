// Package platform reads the file metadata du needs. Unix gets real block
// counts and inode numbers from syscall.Stat_t. Windows has neither in the
// standard library, so allocation is estimated and hardlinks are not detected.
package platform

// Meta describes a file's size on disk.
type Meta struct {
	Size      int64 // apparent size
	Allocated int64 // bytes allocated on disk
	Dev       int64 // Dev and Ino are 0 on Windows
	Ino       int64
	Nlink     int64
}

// roundUp rounds n up to a multiple of unit.
func roundUp(n, unit int64) int64 {
	if n <= 0 {
		return 0
	}
	if unit <= 0 {
		return n
	}
	return ((n + unit - 1) / unit) * unit
}
