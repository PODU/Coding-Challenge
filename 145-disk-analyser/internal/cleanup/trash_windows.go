//go:build windows

package cleanup

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

const (
	foDelete           = 3
	fofSilent          = 0x0004
	fofNoConfirmation  = 0x0010
	fofAllowUndo       = 0x0040
	fofNoErrorUI       = 0x0400
	fofWantNukeWarning = 0x4000
)

// shFileOpStruct is SHFILEOPSTRUCTW with the natural alignment of 64-bit
// Windows (the header only packs it on 32-bit).
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// RecycleBin sends items to the Recycle Bin with the shell's own delete
// operation and FOF_ALLOWUNDO, so they can be restored from Explorer.
// FOF_WANTNUKEWARNING makes the shell ask first rather than silently
// deleting when a drive has no Recycle Bin.
type RecycleBin struct{}

func NewTrasher() Trasher { return RecycleBin{} }

func (RecycleBin) Where() string { return "the Recycle Bin" }

func (RecycleBin) Trash(path string) error {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return errors.New("Recycle Bin support needs 64-bit Windows")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	from, err := syscall.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0) // the list is double-NUL terminated
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarning,
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		return fmt.Errorf("shell error 0x%X", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("cancelled")
	}
	return nil
}
