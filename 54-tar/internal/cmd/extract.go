package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"mytar/internal/archive"
	"mytar/internal/ustar"
)

// Extract unpacks the archive in r under dest, refusing any member that would
// land outside dest. Failures to restore mode or mtime are only warnings.
func Extract(r io.Reader, dest string, verbose bool, logw io.Writer) error {
	ar := archive.NewReader(r)
	for {
		h, err := ar.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}

		target, err := sanitizePath(dest, h.Name)
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}

		if verbose && logw != nil {
			fmt.Fprintln(logw, h.Name)
		}

		switch h.Typeflag {
		case ustar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(h.Mode)&os.ModePerm); err != nil {
				return fmt.Errorf("extract: mkdir %s: %w", target, err)
			}
		case ustar.TypeSymlink:
			if linkEscapes(dest, target, h.Linkname) {
				return fmt.Errorf("extract: refusing symlink %q -> %q: escapes destination", h.Name, h.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("extract: mkdir for symlink %s: %w", target, err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(filepath.FromSlash(h.Linkname), target); err != nil {
				return fmt.Errorf("extract: symlink %s: %w", target, err)
			}
		case ustar.TypeReg:
			if err := writeRegular(ar, target, h, logw); err != nil {
				return err
			}
		case ustar.TypeLink:
			// The link target is a member extracted earlier.
			src, err := sanitizePath(dest, h.Linkname)
			if err != nil {
				return fmt.Errorf("extract: hardlink target %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(src, target); err != nil {
				return fmt.Errorf("extract: hardlink %s: %w", target, err)
			}
		default:
			if logw != nil {
				fmt.Fprintf(logw, "mytar: skipping unsupported member %q (type %q)\n", h.Name, string(h.Typeflag))
			}
		}
	}
}

func writeRegular(ar *archive.Reader, target string, h *ustar.Header, logw io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("extract: mkdir for %s: %w", target, err)
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&os.ModePerm)
	if err != nil {
		return fmt.Errorf("extract: create %s: %w", target, err)
	}
	if _, err := io.Copy(f, ar); err != nil {
		f.Close()
		return fmt.Errorf("extract: write %s: %w", target, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("extract: close %s: %w", target, err)
	}
	restoreMeta(target, h, logw)
	return nil
}

func restoreMeta(target string, h *ustar.Header, logw io.Writer) {
	if err := os.Chmod(target, os.FileMode(h.Mode)&os.ModePerm); err != nil && logw != nil {
		fmt.Fprintf(logw, "mytar: warning: chmod %s: %v\n", target, err)
	}
	if !h.ModTime.IsZero() {
		if err := os.Chtimes(target, time.Now(), h.ModTime); err != nil && logw != nil {
			fmt.Fprintf(logw, "mytar: warning: chtimes %s: %v\n", target, err)
		}
	}
}
