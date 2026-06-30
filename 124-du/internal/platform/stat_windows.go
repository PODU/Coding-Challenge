//go:build windows

package platform

import "os"

// clusterSize is the usual NTFS cluster size. On Windows FileInfo.Sys() has no
// block count or file ID (that needs GetFileInformationByHandle), so we round
// the size up to a cluster and report Nlink 1, which turns off hardlink dedup.
const clusterSize = 4096

// Stat returns approximate metadata for info.
func Stat(info os.FileInfo) Meta {
	size := info.Size()
	return Meta{
		Size:      size,
		Allocated: roundUp(size, clusterSize),
		Nlink:     1,
	}
}
