package scheduler

import (
	"context"
	"errors"
	"os/exec"
	"runtime"

	"crontab/internal/job"
)

// Result is the outcome of one run of a job.
type Result struct {
	ExitCode int
	Output   string
	Err      error
}

// Runner runs a job's command. Implementations must not panic.
type Runner interface {
	Run(ctx context.Context, j job.Job) Result
}

// ExecRunner runs commands through sh -c, or cmd /C on Windows.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, j job.Job) Result {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", j.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", j.Command)
	}

	out, err := cmd.CombinedOutput()
	res := Result{Output: string(out), Err: err}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
	}
	return res
}
