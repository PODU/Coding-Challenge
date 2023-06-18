package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Exit statuses follow the usual POSIX shell conventions.
const (
	statusOK            = 0
	statusGeneralError  = 1
	statusNotExecutable = 126
	statusNotFound      = 127
	statusInterrupted   = 130
)

// RunPipeline runs every stage of pl, connecting neighbours with OS pipes and
// applying redirections. Like other shells it returns the last stage's status.
func (sh *Shell) RunPipeline(pl *Pipeline) int {
	n := len(pl.Commands)
	if n == 0 {
		return statusOK
	}

	cmds := make([]*exec.Cmd, n)
	// Pipe ends and redirect files the shell itself has to close.
	var toClose []io.Closer

	cleanup := func() {
		for _, c := range toClose {
			_ = c.Close()
		}
	}

	for i, c := range pl.Commands {
		cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
		cmd.Stderr = sh.Stderr
		cmds[i] = cmd
	}

	cmds[0].Stdin = sh.Stdin
	cmds[n-1].Stdout = sh.Stdout

	for i := 0; i < n-1; i++ {
		pr, pw, err := os.Pipe()
		if err != nil {
			fmt.Fprintf(sh.Stderr, "ccsh: pipe: %v\n", err)
			cleanup()
			return statusGeneralError
		}
		cmds[i].Stdout = pw
		cmds[i+1].Stdin = pr
		toClose = append(toClose, pr, pw)
	}

	// Redirections take precedence over pipes.
	for i, c := range pl.Commands {
		for _, r := range c.Redirs {
			f, err := openRedirect(r)
			if err != nil {
				fmt.Fprintf(sh.Stderr, "ccsh: %v\n", err)
				cleanup()
				return statusGeneralError
			}
			toClose = append(toClose, f)
			if r.Type == TokenRedirectIn {
				cmds[i].Stdin = f
			} else {
				cmds[i].Stdout = f
			}
		}
	}

	started := make([]*exec.Cmd, 0, n)
	for i, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			status := startErrorStatus(err)
			fmt.Fprintf(sh.Stderr, "ccsh: %s\n", startErrorMessage(pl.Commands[i].Argv[0], err))
			cleanup()
			for _, s := range started {
				_ = s.Process.Kill()
				_ = s.Wait()
			}
			return status
		}
		started = append(started, cmd)
	}

	// The children have their own copies of the pipe fds now. Ours must be
	// closed or the readers never see EOF.
	sh.setForeground(started)
	cleanup()

	status := statusOK
	for i, cmd := range started {
		err := cmd.Wait()
		if i == len(started)-1 {
			status = waitStatus(err)
		}
	}
	sh.clearForeground()

	return status
}

func openRedirect(r Redirect) (*os.File, error) {
	var f *os.File
	var err error
	switch r.Type {
	case TokenRedirectIn:
		f, err = os.Open(r.Target)
	case TokenRedirectAppend:
		f, err = os.OpenFile(r.Target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	default:
		f, err = os.OpenFile(r.Target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %v", r.Target, err)
	}
	return f, nil
}

func startErrorStatus(err error) int {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return statusNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return statusNotExecutable
	}
	return statusGeneralError
}

func startErrorMessage(name string, err error) string {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "command not found: " + name
	}
	if errors.Is(err, os.ErrPermission) {
		return name + ": permission denied"
	}
	return name + ": " + err.Error()
}

// waitStatus turns the error from cmd.Wait into an exit status.
func waitStatus(err error) int {
	if err == nil {
		return statusOK
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code
		}
		// -1 means the process was killed by a signal.
		return statusInterrupted
	}
	return statusGeneralError
}
