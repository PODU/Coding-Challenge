// Command bf is a Brainfuck interpreter. Programs are compiled to a small
// optimised bytecode and run on a VM.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitCompile = 2 // also usage errors
	exitIO      = 3
)

type config struct {
	cells   int
	eof     EOFMode
	wrapPtr bool
	opt     OptLevel
	naive   bool
	dump    bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bf", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr, fs) }

	cells := fs.Int("cells", 30000, "tape size in cells")
	eofStr := fs.String("eof", "0", "EOF policy for ',': 0 | neg1 | unchanged")
	wrapPtr := fs.Bool("wrap-ptr", false, "make the tape circular instead of erroring on out-of-bounds")
	optStr := fs.String("opt", "full", "optimization level: none | basic | full")
	naive := fs.Bool("naive", false, "use the naive tree-walking interpreter instead of the VM")
	dump := fs.Bool("dump", false, "print compiled bytecode and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitCompile
	}

	eofMode, err := ParseEOFMode(*eofStr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCompile
	}
	optLevel, err := ParseOptLevel(*optStr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCompile
	}
	if *cells < 1 {
		fmt.Fprintln(stderr, "compile error: --cells must be >= 1")
		return exitCompile
	}

	cfg := config{
		cells:   *cells,
		eof:     eofMode,
		wrapPtr: *wrapPtr,
		opt:     optLevel,
		naive:   *naive,
		dump:    *dump,
	}

	rest := fs.Args()

	if len(rest) == 1 && rest[0] == "repl" {
		return runREPL(cfg, stdin, stdout, stderr)
	}

	var src []byte
	switch {
	case len(rest) == 0, len(rest) == 1 && rest[0] == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, (&IOError{Msg: "reading stdin", Err: err}).Error())
			return exitIO
		}
		src = b
	case len(rest) == 1:
		b, err := os.ReadFile(rest[0])
		if err != nil {
			fmt.Fprintln(stderr, (&IOError{Msg: "reading " + rest[0], Err: err}).Error())
			return exitIO
		}
		src = b
	default:
		fmt.Fprintln(stderr, "usage error: expected at most one program argument")
		printUsage(stderr, fs)
		return exitCompile
	}

	return execProgram(cfg, src, stdin, stdout, stderr)
}

// execProgram compiles (unless naive) and runs a single program, mapping errors
// to exit codes. When cfg.dump is set it prints bytecode and returns.
func execProgram(cfg config, src []byte, stdin io.Reader, stdout, stderr io.Writer) int {
	if cfg.dump {
		prog, err := Compile(src, cfg.opt)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitCompile
		}
		DumpBytecode(stdout, prog)
		return exitOK
	}

	tape := NewTape(cfg.cells, cfg.wrapPtr)

	var runErr error
	if cfg.naive {
		it := NewInterpreter(tape, stdin, stdout, cfg.eof)
		runErr = it.Run(src)
	} else {
		prog, err := Compile(src, cfg.opt)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitCompile
		}
		vm := NewVM(prog, tape, stdin, stdout, cfg.eof)
		runErr = vm.Run()
	}

	return mapError(runErr, stderr)
}

// mapError prints err (if any) to stderr and returns the matching exit code.
func mapError(err error, stderr io.Writer) int {
	if err == nil {
		return exitOK
	}
	fmt.Fprintln(stderr, err)
	var ce *CompileError
	var re *RuntimeError
	var ioe *IOError
	switch {
	case errors.As(err, &ce):
		return exitCompile
	case errors.As(err, &re):
		return exitRuntime
	case errors.As(err, &ioe):
		return exitIO
	default:
		return exitRuntime
	}
}

// runREPL evaluates Brainfuck a line at a time, sharing one persistent tape so
// that state carries across lines. Output from '.' is printed inline.
func runREPL(cfg config, stdin io.Reader, stdout, stderr io.Writer) int {
	tape := NewTape(cfg.cells, cfg.wrapPtr)
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	fmt.Fprintln(stderr, "bf REPL — enter Brainfuck, one program per line. Ctrl-D to exit.")
	for {
		fmt.Fprint(stderr, "bf> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Bytes()
		prog, err := Compile(line, cfg.opt)
		if err != nil {
			fmt.Fprintln(stderr, err)
			continue
		}
		vm := NewVM(prog, tape, stdin, stdout, cfg.eof)
		if err := vm.Run(); err != nil {
			fmt.Fprintln(stderr, err)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(stderr, (&IOError{Msg: "reading repl input", Err: err}).Error())
		return exitIO
	}
	fmt.Fprintln(stderr)
	return exitOK
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(w, `bf — a Brainfuck interpreter

Usage:
  bf [flags] [program.bf]   Run a program from a file
  bf [flags] -              Read the program from stdin, then run
  cat prog.bf | bf -        Same, via a pipe
  bf [flags] repl           Interactive REPL (state persists across lines)

Flags:
`)
	fs.PrintDefaults()
	fmt.Fprint(w, `
Program input for ',' comes from stdin; output from '.' goes to stdout.
Exit codes: 0 success, 1 runtime error, 2 compile/usage error, 3 I/O error.
`)
}
