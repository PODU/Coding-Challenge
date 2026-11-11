//go:build !windows && !darwin

package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// XDGTrash implements the freedesktop.org Trash specification, which file
// managers on Linux and the BSDs use, so items can be restored from them.
type XDGTrash struct {
	Home string // $XDG_DATA_HOME/Trash
	UID  int
	Now  func() time.Time
}

func NewTrasher() Trasher {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, _ := os.UserHomeDir()
		data = filepath.Join(home, ".local", "share")
	}
	return &XDGTrash{Home: filepath.Join(data, "Trash"), UID: os.Getuid(), Now: time.Now}
}

func (t *XDGTrash) Where() string { return "the Trash" }

// Trash renames path into a trash directory on the same filesystem: the
// home trash if possible, otherwise $topdir/.Trash-$uid. It never copies,
// so a failure leaves the item exactly where it was.
func (t *XDGTrash) Trash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := t.Home
	if !sameDevice(abs, t.Home) {
		top, err := mountPoint(abs)
		if err != nil {
			return err
		}
		dir = filepath.Join(top, ".Trash-"+strconv.Itoa(t.UID))
	}
	files, info := filepath.Join(dir, "files"), filepath.Join(dir, "info")
	for _, d := range []string{files, info} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	base := filepath.Base(abs)
	for i := 1; i < 10000; i++ {
		name := base
		if i > 1 {
			name = base + "." + strconv.Itoa(i)
		}
		// Claiming the .trashinfo with O_EXCL reserves the name.
		infoPath := filepath.Join(info, name+".trashinfo")
		f, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(files, name)); err == nil {
			f.Close()
			os.Remove(infoPath)
			continue
		}
		u := url.URL{Path: abs}
		_, err = fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", u.EscapedPath(), t.Now().Format("2006-01-02T15:04:05"))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(abs, filepath.Join(files, name))
		}
		if err != nil {
			os.Remove(infoPath)
			return err
		}
		return nil
	}
	return errors.New("no free name in the Trash")
}
