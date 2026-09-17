package exec

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"runtime"
	"strings"
	"time"
)

// Timing is one measurement of one process execution. All durations are in
// seconds, which is the unit every downstream statistic is computed in.
type Timing struct {
	Real     float64
	User     float64
	System   float64
	ExitCode int
}

// Streams describes where the benchmarked process's standard streams go.
// A nil writer means the stream is discarded.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes command under shell (or directly, when the shell is disabled)
// and returns its wall-clock and processor times.
//
// Only a failure to *start* the process is returned as an error; a process that
// runs to completion with a non-zero status is a successful measurement whose
// ExitCode the caller decides what to do with.
func Run(shell Shell, command string, env []string, s Streams) (Timing, error) {
	cmd, err := build(shell, command)
	if err != nil {
		return Timing{}, err
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdin = s.Stdin
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr

	start := time.Now()
	err = cmd.Run()
	elapsed := time.Since(start)

	st := cmd.ProcessState
	if st == nil {
		return Timing{}, fmt.Errorf("could not run command %q: %w", command, err)
	}
	var exitErr *osexec.ExitError
	if err != nil && !asExitError(err, &exitErr) {
		return Timing{}, fmt.Errorf("could not run command %q: %w", command, err)
	}
	return Timing{
		Real:     elapsed.Seconds(),
		User:     st.UserTime().Seconds(),
		System:   st.SystemTime().Seconds(),
		ExitCode: st.ExitCode(),
	}, nil
}

func asExitError(err error, target **osexec.ExitError) bool {
	ee, ok := err.(*osexec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

// build assembles the os/exec command. With a shell it passes the command
// through verbatim; without one it splits the command into words itself.
func build(shell Shell, command string) (*osexec.Cmd, error) {
	if !shell.Enabled() {
		words, err := Split(command)
		if err != nil {
			return nil, err
		}
		if len(words) == 0 {
			return nil, fmt.Errorf("empty command")
		}
		return osexec.Command(words[0], words[1:]...), nil
	}

	argv := append(append([]string(nil), shell.Argv...), command)
	cmd := osexec.Command(argv[0], argv[1:]...)
	if runtime.GOOS == "windows" {
		// Go quotes each argument with MSVC rules, which cmd.exe does not
		// follow. Hand cmd.exe the exact line we mean instead.
		setRawCommandLine(cmd, windowsCommandLine(shell, command))
	}
	return cmd, nil
}

// windowsCommandLine builds the literal line passed to CreateProcess. With
// `/s /c`, cmd.exe strips the outermost pair of quotes and runs the remainder
// as-is, which is exactly the pass-through we want.
func windowsCommandLine(shell Shell, command string) string {
	var b strings.Builder
	for _, a := range shell.Argv {
		b.WriteString(a)
		b.WriteByte(' ')
	}
	b.WriteByte('"')
	b.WriteString(command)
	b.WriteByte('"')
	return b.String()
}

// Discard is a Streams value that throws away all output and supplies no input.
var Discard = Streams{Stdin: nil, Stdout: nil, Stderr: nil}

// Inherit is a Streams value that connects the child to our own terminal.
func Inherit() Streams {
	return Streams{Stdout: os.Stdout, Stderr: os.Stderr}
}
