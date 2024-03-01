package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// Exit codes, as specified by POSIX.
const (
	exitOK          = 0
	exitChildFailed = 123 // a command exited 1-125
	exitChild255    = 124 // a command exited 255
	exitChildSignal = 125 // a command was killed by a signal
	exitCannotRun   = 126 // command found but could not be executed
	exitNotFound    = 127 // command not found
)

// ExecConfig controls how jobs are run.
type ExecConfig struct {
	MaxProcs int  // -P; <= 0 means as many as is reasonable
	Verbose  bool // -t

	// Default to os.Stdout and os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// RunJobs runs the jobs, up to MaxProcs at a time, and returns the exit code
// for the worst result. After a fatal result (124-127) no new jobs are
// started, but running ones are allowed to finish.
func RunJobs(jobs []Job, cfg ExecConfig) int {
	if len(jobs) == 0 {
		return exitOK
	}

	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	maxProcs := cfg.MaxProcs
	if maxProcs <= 0 {
		// "As many as possible", within reason.
		maxProcs = runtime.NumCPU() * 8
	}
	if maxProcs > len(jobs) {
		maxProcs = len(jobs)
	}

	var (
		mu        sync.Mutex
		finalCode = exitOK
		stop      = false
	)
	record := func(code int, fatal bool) {
		mu.Lock()
		if rank(code) > rank(finalCode) {
			finalCode = code
		}
		if fatal {
			stop = true
		}
		mu.Unlock()
	}
	stopped := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return stop
	}

	sem := make(chan struct{}, maxProcs)
	var wg sync.WaitGroup
	for _, job := range jobs {
		if stopped() {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			defer func() { <-sem }()
			record(runOne(j, cfg.Verbose, stdout, stderr))
		}(job)
	}
	wg.Wait()
	return finalCode
}

func runOne(j Job, verbose bool, stdout, stderr io.Writer) (code int, fatal bool) {
	if len(j.Argv) == 0 {
		return exitOK, false
	}
	if verbose {
		fmt.Fprintln(stderr, formatArgv(j.Argv))
	}
	cmd := exec.Command(j.Argv[0], j.Argv[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return classify(cmd.Run())
}

// classify maps the result of running a command to an exit code, and whether
// xargs should stop starting new commands.
func classify(err error) (int, bool) {
	if err == nil {
		return exitOK, false
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := ee.ProcessState.ExitCode()
		switch {
		case code == -1:
			// Killed by a signal.
			return exitChildSignal, true
		case code == 255:
			return exitChild255, true
		default:
			return exitChildFailed, false
		}
	}
	// It never started. Anything other than a permission problem (missing
	// binary or otherwise) is reported as not found.
	if errors.Is(err, fs.ErrPermission) {
		return exitCannotRun, true
	}
	return exitNotFound, true
}

// rank orders exit codes by severity.
func rank(code int) int {
	switch code {
	case exitOK:
		return 0
	case exitChildFailed:
		return 1
	case exitChild255:
		return 2
	case exitChildSignal:
		return 3
	case exitCannotRun:
		return 4
	case exitNotFound:
		return 5
	default:
		return 1
	}
}

// formatArgv renders a command for -t, quoting empty arguments and ones with
// blanks.
func formatArgv(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t\n") {
			parts[i] = "'" + a + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}
