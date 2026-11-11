//go:build darwin

package cleanup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MacTrash renames items into ~/.Trash, or into the volume's
// .Trashes/<uid> for other volumes, which is where Finder looks.
type MacTrash struct {
	Home string // ~/.Trash
	UID  int
}

func NewTrasher() Trasher {
	home, _ := os.UserHomeDir()
	return &MacTrash{Home: filepath.Join(home, ".Trash"), UID: os.Getuid()}
}

func (t *MacTrash) Where() string { return "the Trash" }

func (t *MacTrash) Trash(path string) error {
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
		dir = filepath.Join(top, ".Trashes", strconv.Itoa(t.UID))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	base := filepath.Base(abs)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i < 10000; i++ {
		name := base
		if i > 1 {
			name = stem + " " + strconv.Itoa(i) + ext // Finder's own convention
		}
		dst := filepath.Join(dir, name)
		if _, err := os.Lstat(dst); errors.Is(err, fs.ErrNotExist) {
			return os.Rename(abs, dst)
		}
	}
	return errors.New("no free name in the Trash")
}
