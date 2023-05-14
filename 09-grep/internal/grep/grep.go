// Package grep runs a search over files or stdin and prints the results.
package grep

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mygrep/internal/cli"
	"mygrep/internal/matcher"
)

// maxLine raises bufio.Scanner's 64 KiB default so minified files and wide
// CSVs don't fail with "token too long".
const maxLine = 1 << 20

const stdinName = "(standard input)"

// Run performs the search and returns grep's exit status: 0 if any line was
// selected, 1 if none were, 2 on any error.
func Run(cfg *cli.Config, in io.Reader, out, errOut io.Writer) int {
	m, err := matcher.New(cfg.Pattern, cfg.IgnoreCase, cfg.WordMatch)
	if err != nil {
		fmt.Fprintf(errOut, "mygrep: invalid pattern %q: %v\n", cfg.Pattern, err)
		return 2
	}

	w := bufio.NewWriter(out)
	defer w.Flush()

	st := &state{
		cfg:      cfg,
		m:        m,
		out:      w,
		errOut:   errOut,
		showName: cfg.Recursive || len(cfg.Paths) > 1,
	}

	if len(cfg.Paths) == 0 {
		st.search(stdinName, in)
	} else {
		for _, p := range cfg.Paths {
			st.resolve(p, in)
		}
	}

	w.Flush()
	switch {
	case st.errored:
		return 2
	case st.anyMatch:
		return 0
	default:
		return 1
	}
}

type state struct {
	cfg      *cli.Config
	m        matcher.Matcher
	out      *bufio.Writer
	errOut   io.Writer
	showName bool
	anyMatch bool
	errored  bool
}

// resolve searches one path argument, which may be "-", a file or a directory.
func (st *state) resolve(p string, stdin io.Reader) {
	if p == "-" {
		st.search(stdinName, stdin)
		return
	}

	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(st.errOut, "mygrep: %s: No such file or directory\n", p)
		} else {
			fmt.Fprintf(st.errOut, "mygrep: %s: %v\n", p, err)
		}
		st.errored = true
		return
	}

	if info.IsDir() {
		if !st.cfg.Recursive {
			fmt.Fprintf(st.errOut, "mygrep: %s: Is a directory\n", p)
			st.errored = true
			return
		}
		st.walk(p)
		return
	}

	f, err := os.Open(p)
	if err != nil {
		fmt.Fprintf(st.errOut, "mygrep: %s: %v\n", p, err)
		st.errored = true
		return
	}
	defer f.Close()
	st.search(p, f)
}

// walk searches every regular file under root. WalkDir does not follow
// symlinked directories, so symlink loops can't trap us.
func (st *state) walk(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(st.errOut, "mygrep: %s: %v\n", path, err)
			st.errored = true
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		f, ferr := os.Open(path)
		if ferr != nil {
			fmt.Fprintf(st.errOut, "mygrep: %s: %v\n", path, ferr)
			st.errored = true
			return nil
		}
		defer f.Close()
		// Use forward slashes so output is the same on Windows.
		st.search(filepath.ToSlash(path), f)
		return nil
	})
}

// search prints the selected lines of r, or just their count under -c.
func (st *state) search(name string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)

	count := 0
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSuffix(sc.Text(), "\r")
		hit := st.m.Matches(line)
		if st.cfg.Invert {
			hit = !hit
		}
		if !hit {
			continue
		}
		count++
		st.anyMatch = true
		if !st.cfg.Count {
			st.writeLine(name, lineNo, line)
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(st.errOut, "mygrep: %s: %v\n", name, err)
		st.errored = true
	}

	if st.cfg.Count {
		if st.showName {
			fmt.Fprintf(st.out, "%s:%d\n", name, count)
		} else {
			fmt.Fprintf(st.out, "%d\n", count)
		}
	}
}

func (st *state) writeLine(name string, lineNo int, line string) {
	if st.showName {
		st.out.WriteString(name)
		st.out.WriteByte(':')
	}
	if st.cfg.LineNumbers {
		fmt.Fprintf(st.out, "%d:", lineNo)
	}
	st.out.WriteString(line)
	st.out.WriteByte('\n')
}
