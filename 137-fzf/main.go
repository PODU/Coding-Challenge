// ccfzf is a fuzzy finder: it reads items from standard input, lets you narrow
// them down interactively, and prints what you picked.
//
// Coding Challenge #137.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ccfzf/internal/cli"
	"ccfzf/internal/matcher"
	"ccfzf/internal/pattern"
	"ccfzf/internal/term"
	"ccfzf/internal/ui"
)

const version = "1.0.0"

// Exit codes, as documented for fzf.
const (
	exitOK          = 0
	exitNoMatch     = 1
	exitError       = 2
	exitInterrupted = 130
)

func main() { os.Exit(run()) }

func run() int {
	opts, err := cli.ParseWithDefaults(os.Getenv("FZF_DEFAULT_OPTS"), os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccfzf: %s\n", err)
		return exitError
	}
	if opts.Help {
		fmt.Print(cli.Usage)
		return exitOK
	}
	if opts.Version {
		fmt.Printf("ccfzf %s\n", version)
		return exitOK
	}

	input, cleanup, err := openInput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccfzf: %s\n", err)
		return exitError
	}
	defer cleanup()

	if opts.HasFilter {
		return runFilter(opts, input)
	}
	return runInteractive(opts, input)
}

// openInput returns the item source: standard input when something is piped
// in, and otherwise $FZF_DEFAULT_COMMAND or a walk of the current directory.
func openInput() (io.Reader, func(), error) {
	stat, err := os.Stdin.Stat()
	piped := err == nil && stat.Mode()&os.ModeCharDevice == 0
	if piped {
		return os.Stdin, func() {}, nil
	}
	if cmdline := os.Getenv("FZF_DEFAULT_COMMAND"); cmdline != "" {
		return runCommand(cmdline)
	}
	return walkCurrentDir(), func() {}, nil
}

func runCommand(cmdline string) (io.Reader, func(), error) {
	name, args := shellCommand(cmdline)
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("FZF_DEFAULT_COMMAND: %w", err)
	}
	return out, func() {
		out.Close()
		cmd.Wait()
	}, nil
}

func shellCommand(cmdline string) (string, []string) {
	if os.PathSeparator == '\\' {
		return "cmd", []string{"/c", cmdline}
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-c", cmdline}
}

// walkCurrentDir is the fallback source: every file under the working
// directory, skipping hidden directories the way the usual find command in
// FZF_DEFAULT_COMMAND does.
func walkCurrentDir() io.Reader {
	pr, pw := io.Pipe()
	go func() {
		w := pw
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if path != "." && strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return nil
			}
			_, werr := io.WriteString(w, filepath.ToSlash(path)+"\n")
			return werr
		})
		pw.CloseWithError(err)
	}()
	return pr
}

// runFilter is --filter: no interface, just print what matches.
func runFilter(opts *cli.Options, input io.Reader) int {
	store := matcher.NewStore()
	if err := matcher.Read(input, opts.Read0, opts.ItemBuilder(), store, nil); err != nil {
		fmt.Fprintf(os.Stderr, "ccfzf: %s\n", err)
		return exitError
	}
	pat := pattern.Build(opts.Filter, opts.PatternOptions())
	matches, _ := matcher.Run(store, pat, 0, store.Count(), matchOptions(opts), nil)

	out := newWriter(opts)
	defer out.Flush()
	if opts.PrintQuery {
		out.line(opts.Filter)
	}
	for _, m := range matches {
		out.line(m.Item.Raw)
	}
	if len(matches) == 0 {
		return exitNoMatch
	}
	return exitOK
}

func matchOptions(opts *cli.Options) matcher.Options {
	return matcher.Options{Tiebreak: opts.Tiebreak, Sort: opts.Sort, Tac: opts.Tac}
}

func runInteractive(opts *cli.Options, input io.Reader) int {
	store := matcher.NewStore()

	// -1 and -0 have to know the final match count, so the input is consumed
	// up front; otherwise reading happens in the background and the interface
	// appears instantly.
	if opts.Select1 || opts.Exit0 {
		if err := matcher.Read(input, opts.Read0, opts.ItemBuilder(), store, nil); err != nil {
			fmt.Fprintf(os.Stderr, "ccfzf: %s\n", err)
			return exitError
		}
		pat := pattern.Build(opts.Query, opts.PatternOptions())
		matches, _ := matcher.Run(store, pat, 0, store.Count(), matchOptions(opts), nil)
		if len(matches) == 0 && opts.Exit0 {
			return finish(opts, ui.Result{Query: opts.Query}, exitNoMatch)
		}
		if len(matches) == 1 && opts.Select1 {
			res := ui.Result{Query: opts.Query, Selected: []string{matches[0].Item.Raw}}
			return finish(opts, res, exitOK)
		}
	} else {
		go func() {
			matcher.Read(input, opts.Read0, opts.ItemBuilder(), store, nil)
		}()
	}

	tty, err := term.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccfzf: cannot open terminal: %s\n", err)
		return exitError
	}
	defer tty.Close()

	res, err := ui.New(opts, store, tty).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccfzf: %s\n", err)
		return exitError
	}
	if res.Aborted {
		return finish(opts, res, exitInterrupted)
	}
	if len(res.Selected) == 0 {
		return finish(opts, res, exitNoMatch)
	}
	return finish(opts, res, exitOK)
}

// finish writes the output lines fzf promises: the query first when asked for,
// then the key that accepted when --expect is in play, then the selection.
func finish(opts *cli.Options, res ui.Result, code int) int {
	out := newWriter(opts)
	defer out.Flush()
	if opts.PrintQuery {
		out.line(res.Query)
	}
	if len(opts.Expect) > 0 {
		out.line(res.Key)
	}
	for _, s := range res.Selected {
		out.line(s)
	}
	return code
}

type writer struct {
	w     *os.File
	delim string
	buf   strings.Builder
}

func newWriter(opts *cli.Options) *writer {
	delim := "\n"
	if opts.Print0 {
		delim = "\x00"
	}
	return &writer{w: os.Stdout, delim: delim}
}

func (w *writer) line(s string) {
	w.buf.WriteString(s)
	w.buf.WriteString(w.delim)
}

func (w *writer) Flush() { io.WriteString(w.w, w.buf.String()) }
