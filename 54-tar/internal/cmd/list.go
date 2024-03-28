package cmd

import (
	"errors"
	"fmt"
	"io"

	"mytar/internal/archive"
	"mytar/internal/ustar"
)

// List prints member names from the archive read from r. With verbose, a long
// format line (mode, owner/group, size, mtime, name) is printed instead.
func List(r io.Reader, w io.Writer, verbose bool) error {
	ar := archive.NewReader(r)
	for {
		h, err := ar.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}
		if verbose {
			fmt.Fprintln(w, longFormat(h))
		} else {
			fmt.Fprintln(w, h.Name)
		}
	}
}

// longFormat renders a line like `tar -tv`.
func longFormat(h *ustar.Header) string {
	owner := fmt.Sprintf("%s/%s", orDefault(h.Uname, "root"), orDefault(h.Gname, "root"))
	mtime := h.ModTime.Format("2006-01-02 15:04")
	name := h.Name
	if h.Typeflag == ustar.TypeSymlink && h.Linkname != "" {
		name = fmt.Sprintf("%s -> %s", name, h.Linkname)
	}
	return fmt.Sprintf("%s %s %10d %s %s", modeString(h), owner, h.Size, mtime, name)
}

// modeString renders e.g. "-rwxr-xr-x".
func modeString(h *ustar.Header) string {
	var typ byte = '-'
	switch h.Typeflag {
	case ustar.TypeDir:
		typ = 'd'
	case ustar.TypeSymlink:
		typ = 'l'
	case ustar.TypeLink:
		typ = 'h'
	}
	perm := []byte("rwxrwxrwx")
	out := make([]byte, 10)
	out[0] = typ
	for i := 0; i < 9; i++ {
		if h.Mode&(1<<uint(8-i)) != 0 {
			out[i+1] = perm[i]
		} else {
			out[i+1] = '-'
		}
	}
	return string(out)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
