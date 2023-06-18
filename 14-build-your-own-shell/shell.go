package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// Shell is the interpreter state. The I/O streams are fields so tests can use
// buffers.
type Shell struct {
	Cwd        string
	PrevDir    string
	LastStatus int
	History    *History

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	fgMu   sync.Mutex
	fgCmds []*exec.Cmd
}

// NewShell returns a Shell using the process's standard streams. The caller
// loads the history.
func NewShell() *Shell {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return &Shell{
		Cwd:     cwd,
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		History: NewHistory(historyPath()),
	}
}

// Execute runs one command line. quit is true after the exit builtin.
func (sh *Shell) Execute(line string) (status int, quit bool) {
	tokens, err := Tokenize(line)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "ccsh: %v\n", err)
		sh.LastStatus = 2
		return 2, false
	}

	pl, err := Parse(tokens)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "ccsh: %v\n", err)
		sh.LastStatus = 2
		return 2, false
	}

	if len(pl.Commands) == 0 {
		return sh.LastStatus, false
	}

	// A lone builtin runs in-process so that cd and friends affect the shell.
	// Parse never returns a command with an empty Argv.
	if len(pl.Commands) == 1 {
		argv := pl.Commands[0].Argv
		if b, ok := builtins[argv[0]]; ok {
			st, quit := b(argv, sh)
			sh.LastStatus = st
			return st, quit
		}
	}

	st := sh.RunPipeline(pl)
	sh.LastStatus = st
	return st, false
}

// setForeground records the running children so Ctrl-C can reach them.
func (sh *Shell) setForeground(cmds []*exec.Cmd) {
	sh.fgMu.Lock()
	sh.fgCmds = cmds
	sh.fgMu.Unlock()
}

func (sh *Shell) clearForeground() {
	sh.fgMu.Lock()
	sh.fgCmds = nil
	sh.fgMu.Unlock()
}

// InterruptForeground kills the running children. On Unix the terminal already
// sends them SIGINT; Windows has no process groups, so there this is what
// actually stops them.
func (sh *Shell) InterruptForeground() {
	sh.fgMu.Lock()
	defer sh.fgMu.Unlock()
	for _, c := range sh.fgCmds {
		if c != nil && c.Process != nil {
			_ = c.Process.Kill()
		}
	}
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}
