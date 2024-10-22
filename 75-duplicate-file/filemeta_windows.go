//go:build windows

package main

import "os"

// devIno returns zeros on Windows, where os.FileInfo has no device or inode
// numbers. Hardlinks then just get compared byte by byte like any other file.
func devIno(info os.FileInfo) (uint64, uint64) {
	return 0, 0
}
