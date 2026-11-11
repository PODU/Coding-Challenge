//go:build windows

package scan

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGetCompressedFileSizeW = kernel32.NewProc("GetCompressedFileSizeW")
	procGetDiskFreeSpaceW      = kernel32.NewProc("GetDiskFreeSpaceW")
	procGetDiskFreeSpaceExW    = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumePathNameW     = kernel32.NewProc("GetVolumePathNameW")
	clusterSizes               sync.Map // volume root -> int64
)

const (
	fileReadAttributes       = 0x80
	fileFlagOpenReparsePoint = 0x00200000
	fileAttributeReparse     = 0x400
)

// fillStat measures a regular file. GetCompressedFileSize reports the bytes
// actually allocated, so sparse and compressed files come out small; that is
// rounded up to whole clusters like Explorer's "size on disk". The file index
// and link count need a handle, opened for attribute reading only.
func fillStat(n *Node, path string, fi os.FileInfo) {
	if n.Kind != File {
		return
	}
	n.Usage = n.Apparent
	if p, err := syscall.UTF16PtrFromString(path); err == nil {
		var high uint32
		low, _, callErr := procGetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
		if uint32(low) != 0xFFFFFFFF || callErr == syscall.Errno(0) {
			n.Usage = roundUp(int64(high)<<32|int64(uint32(low)), clusterSize(path))
		}
		h, err := syscall.CreateFile(p, fileReadAttributes,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
			syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|fileFlagOpenReparsePoint, 0)
		if err == nil {
			var info syscall.ByHandleFileInformation
			if syscall.GetFileInformationByHandle(h, &info) == nil {
				n.ID = FileID{Dev: uint64(info.VolumeSerialNumber), Ino: uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)}
				n.Links = info.NumberOfLinks
			}
			syscall.CloseHandle(h)
		}
	}
}

func roundUp(n, unit int64) int64 {
	if unit <= 0 || n == 0 {
		return n
	}
	return (n + unit - 1) / unit * unit
}

func volumeRoot(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err == nil {
		buf := make([]uint16, 1024)
		if r, _, _ := procGetVolumePathNameW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r != 0 {
			return syscall.UTF16ToString(buf)
		}
	}
	return filepath.VolumeName(path) + `\`
}

func clusterSize(path string) int64 {
	vol := filepath.VolumeName(path)
	if v, ok := clusterSizes.Load(vol); ok {
		return v.(int64)
	}
	var spc, bps, free, total uint32
	size := int64(4096)
	if p, err := syscall.UTF16PtrFromString(volumeRoot(path)); err == nil {
		r, _, _ := procGetDiskFreeSpaceW.Call(uintptr(unsafe.Pointer(p)),
			uintptr(unsafe.Pointer(&spc)), uintptr(unsafe.Pointer(&bps)),
			uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)))
		if r != 0 && spc*bps > 0 {
			size = int64(spc) * int64(bps)
		}
	}
	clusterSizes.Store(vol, size)
	return size
}

// Directories have no meaningful size on NTFS.
func dirApparent(os.FileInfo) int64 { return 0 }

// isReparsePoint catches junctions and mount points, which Go reports as
// irregular files rather than symlinks. They are never followed.
func isReparsePoint(fi os.FileInfo) bool {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return ok && d.FileAttributes&fileAttributeReparse != 0
}

// FreeSpace reports the bytes available to the current user and the volume
// size.
func FreeSpace(path string) (free, total int64, err error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var avail, tot, totFree uint64
	r, _, callErr := procGetDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&tot)), uintptr(unsafe.Pointer(&totFree)))
	if r == 0 {
		return 0, 0, callErr
	}
	return int64(avail), int64(tot), nil
}
