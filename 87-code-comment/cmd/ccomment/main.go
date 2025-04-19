// Command ccomment strips comments from source code.
//
// Usage:
//
//	ccomment [flags] [file]
//
// With no file (or "-") it reads from stdin and writes to stdout.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ccomment"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ccomment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		lang       = fs.String("lang", "", "language: python|js|ts|c|cpp|go (overrides extension)")
		write      = fs.Bool("w", false, "rewrite the file in place")
		keepDoc    = fs.Bool("keep-doc", true, "keep Go doc comments")
		keepPragma = fs.Bool("keep-pragma", true, "keep tool-config/directive comments")
		goAST      = fs.Bool("go-ast", false, "Go: use go/parser oracle for doc detection (reserved)")
		blankLines = fs.String("blank-lines", "drop", "full-line comment mode: drop|blank")
		minify     = fs.Bool("minify", false, "collapse runs of blank lines")
		stats      = fs.Bool("stats", false, "print removed/kept counts to stderr")
		check      = fs.Bool("check", false, "exit non-zero if any comment would be removed")
		outPath    = fs.String("o", "", "write output to path instead of stdout")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: ccomment [flags] [file]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var mode ccomment.Mode
	switch *blankLines {
	case "drop":
		mode = ccomment.Drop
	case "blank":
		mode = ccomment.Blank
	default:
		fmt.Fprintf(stderr, "ccomment: invalid --blank-lines %q (want drop|blank)\n", *blankLines)
		return 2
	}

	files := fs.Args()
	var file string
	switch len(files) {
	case 0:
		file = "-"
	case 1:
		file = files[0]
	default:
		fmt.Fprintln(stderr, "ccomment: at most one file argument")
		return 2
	}

	if *write && (file == "-" || file == "") {
		fmt.Fprintln(stderr, "ccomment: -w requires a file argument")
		return 2
	}

	// Resolve language.
	var st ccomment.Stripper
	var ok bool
	if *lang != "" {
		st, ok = ccomment.For(*lang)
		if !ok {
			fmt.Fprintf(stderr, "ccomment: unknown language %q\n", *lang)
			return 2
		}
	} else if file != "-" {
		st, ok = ccomment.ForFile(file)
		if !ok {
			fmt.Fprintf(stderr, "ccomment: cannot infer language from %q; use --lang\n", file)
			return 2
		}
	} else {
		fmt.Fprintln(stderr, "ccomment: --lang is required when reading from stdin")
		return 2
	}

	// Read input.
	var src []byte
	var err error
	if file == "-" {
		src, err = io.ReadAll(stdin)
	} else {
		src, err = os.ReadFile(file)
	}
	if err != nil {
		fmt.Fprintf(stderr, "ccomment: %v\n", err)
		return 2
	}

	opt := ccomment.Options{
		KeepDoc:    *keepDoc,
		KeepPragma: *keepPragma,
		BlankLines: mode,
		Minify:     *minify,
		GoAST:      *goAST,
	}

	out, s, err := st.Strip(src, opt)
	if err != nil {
		var ue *ccomment.UnterminatedError
		if errors.As(err, &ue) {
			fmt.Fprintf(stderr, "ccomment: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "ccomment: %v\n", err)
		return 2
	}

	if *stats {
		fmt.Fprintf(stderr, "ccomment: removed=%d kept=%d bytes=%d\n",
			s.CommentsRemoved, s.CommentsKept, s.BytesRemoved)
	}

	if *check {
		if s.CommentsRemoved > 0 {
			return 1
		}
		return 0
	}

	// Write output.
	switch {
	case *write:
		if err := atomicWrite(file, out); err != nil {
			fmt.Fprintf(stderr, "ccomment: %v\n", err)
			return 2
		}
	case *outPath != "":
		if err := os.WriteFile(*outPath, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "ccomment: %v\n", err)
			return 2
		}
	default:
		if _, err := stdout.Write(out); err != nil {
			fmt.Fprintf(stderr, "ccomment: %v\n", err)
			return 2
		}
	}
	return 0
}

// atomicWrite writes data to a temp file in the same directory then renames it
// over path, so a crash never leaves a half-written source file.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccomment-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
