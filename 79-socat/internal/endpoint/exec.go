package endpoint

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// execEndpoint is a child process: writes go to its stdin, reads come from
// its stdout, and its stderr is passed through to ours.
type execEndpoint struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	once    sync.Once
	waitErr error
}

// newExec splits command on whitespace and runs it directly, without a shell.
func newExec(command string) (*execEndpoint, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil, fmt.Errorf("EXEC: empty command")
	}
	return newExecArgv(fields)
}

// newExecArgv is used directly by tests, whose program paths may contain
// spaces.
func newExecArgv(argv []string) (*execEndpoint, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("EXEC: empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("EXEC %q: stdin pipe: %w", argv[0], err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("EXEC %q: stdout pipe: %w", argv[0], err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("EXEC %q: start: %w", argv[0], err)
	}
	return &execEndpoint{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (e *execEndpoint) Read(p []byte) (int, error)  { return e.stdout.Read(p) }
func (e *execEndpoint) Write(p []byte) (int, error) { return e.stdin.Write(p) }

// Close sends EOF to the child and waits for it to exit. It is safe to call
// more than once.
func (e *execEndpoint) Close() error {
	e.once.Do(func() {
		_ = e.stdin.Close()
		e.waitErr = e.cmd.Wait()
		_ = e.stdout.Close()
	})
	return e.waitErr
}

// CloseWrite sends EOF to the child while we keep reading its output.
func (e *execEndpoint) CloseWrite() error { return e.stdin.Close() }

func (e *execEndpoint) CloseRead() error { return e.stdout.Close() }
