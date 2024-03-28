package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mytar/internal/archive"
	"mytar/internal/ustar"
)

// Create writes a USTAR archive of the given input paths to w. Directories are
// walked recursively. Verbose prints each member name to logw as it is added.
func Create(w io.Writer, inputs []string, verbose bool, logw io.Writer) error {
	if len(inputs) == 0 {
		return fmt.Errorf("create: no input files given")
	}
	aw := archive.NewWriter(w)
	for _, in := range inputs {
		root := filepath.Clean(in)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return addMember(aw, path, info, verbose, logw)
		})
		if err != nil {
			return err
		}
	}
	return aw.Close()
}

func addMember(aw *archive.Writer, path string, info os.FileInfo, verbose bool, logw io.Writer) error {
	name := toArchiveName(path, info.IsDir())

	h := &ustar.Header{
		Name:    name,
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
		Uname:   "root",
		Gname:   "root",
	}
	fillOwner(h, info)

	switch {
	case info.IsDir():
		h.Typeflag = ustar.TypeDir
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("create: readlink %s: %w", path, err)
		}
		h.Typeflag = ustar.TypeSymlink
		h.Linkname = filepath.ToSlash(target)
	case info.Mode().IsRegular():
		h.Typeflag = ustar.TypeReg
		h.Size = info.Size()
	default:
		// Sockets, devices and FIFOs are not archived.
		return nil
	}

	if verbose && logw != nil {
		fmt.Fprintln(logw, name)
	}

	// Names too long for USTAR go in a GNU 'L' record; the real header then
	// gets a truncated copy that readers ignore.
	if !ustar.FitsUSTARName(name) {
		if err := aw.WriteGNULongName(name); err != nil {
			return err
		}
		h.Name = truncateName(name)
	}

	if err := aw.WriteHeader(h); err != nil {
		return fmt.Errorf("create: header for %s: %w", name, err)
	}

	if h.Typeflag == ustar.TypeReg && h.Size > 0 {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("create: open %s: %w", path, err)
		}
		defer f.Close()
		n, err := io.Copy(aw, f)
		if err != nil {
			return fmt.Errorf("create: copy %s: %w", path, err)
		}
		if n != h.Size {
			return fmt.Errorf("create: %s changed size during archiving (%d != %d)", path, n, h.Size)
		}
	}
	return nil
}

// truncateName cuts name to 100 bytes, keeping a directory's trailing slash.
func truncateName(name string) string {
	if len(name) <= 100 {
		return name
	}
	s := name[:100]
	if strings.HasSuffix(name, "/") && !strings.HasSuffix(s, "/") {
		s = s[:99] + "/"
	}
	return s
}
