package main

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// lockedWriter serializes concurrent writes so child output can be captured
// safely when running jobs in parallel.
type lockedWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// echoJob builds a portable "echo <arg>" job for the host OS.
func echoJob(arg string) Job {
	if runtime.GOOS == "windows" {
		return Job{Argv: []string{"cmd", "/c", "echo", arg}}
	}
	return Job{Argv: []string{"echo", arg}}
}

// exitJob builds a portable job that exits with the given code.
func exitJob(code int) Job {
	if runtime.GOOS == "windows" {
		return Job{Argv: []string{"cmd", "/c", "exit", strconv.Itoa(code)}}
	}
	return Job{Argv: []string{"sh", "-c", "exit " + strconv.Itoa(code)}}
}

func TestExecSerial(t *testing.T) {
	out := &lockedWriter{}
	jobs := []Job{echoJob("alpha"), echoJob("beta"), echoJob("gamma")}
	code := RunJobs(jobs, ExecConfig{MaxProcs: 1, Stdout: out, Stderr: out})
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	got := out.String()
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q missing %q", got, want)
		}
	}
}

func TestExecParallel(t *testing.T) {
	out := &lockedWriter{}
	var jobs []Job
	for i := 0; i < 12; i++ {
		jobs = append(jobs, echoJob("item"+strconv.Itoa(i)))
	}
	code := RunJobs(jobs, ExecConfig{MaxProcs: 4, Stdout: out, Stderr: out})
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	got := out.String()
	for i := 0; i < 12; i++ {
		if !strings.Contains(got, "item"+strconv.Itoa(i)) {
			t.Fatalf("parallel output missing item%d; got %q", i, got)
		}
	}
}

func TestExecExitCodeMapping(t *testing.T) {
	out := &lockedWriter{}
	tests := []struct {
		name string
		jobs []Job
		want int
	}{
		{"child exit 3 -> 123", []Job{exitJob(3)}, exitChildFailed},
		{"child exit 1 -> 123", []Job{exitJob(1)}, exitChildFailed},
		{"child exit 255 -> 124", []Job{exitJob(255)}, exitChild255},
		{"mixed worst wins", []Job{exitJob(1), exitJob(255)}, exitChild255},
		{"all success -> 0", []Job{echoJob("ok")}, exitOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := RunJobs(tt.jobs, ExecConfig{MaxProcs: 1, Stdout: out, Stderr: out})
			if code != tt.want {
				t.Fatalf("got %d, want %d", code, tt.want)
			}
		})
	}
}

func TestExecCommandNotFound(t *testing.T) {
	out := &lockedWriter{}
	jobs := []Job{{Argv: []string{"this-binary-does-not-exist-xyz123"}}}
	code := RunJobs(jobs, ExecConfig{MaxProcs: 1, Stdout: out, Stderr: out})
	if code != exitNotFound {
		t.Fatalf("expected exit 127, got %d", code)
	}
}

func TestExecKilledBySignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signals not available on Windows")
	}
	out := &lockedWriter{}
	jobs := []Job{{Argv: []string{"sh", "-c", "kill -TERM $$"}}}
	code := RunJobs(jobs, ExecConfig{MaxProcs: 1, Stdout: out, Stderr: out})
	if code != exitChildSignal {
		t.Fatalf("expected exit 125, got %d", code)
	}
}

func TestExecEmptyJobs(t *testing.T) {
	if code := RunJobs(nil, ExecConfig{}); code != exitOK {
		t.Fatalf("expected exit 0 for no jobs, got %d", code)
	}
}

func TestExecVerbose(t *testing.T) {
	out := &lockedWriter{}
	errOut := &lockedWriter{}
	jobs := []Job{echoJob("hello")}
	RunJobs(jobs, ExecConfig{MaxProcs: 1, Verbose: true, Stdout: out, Stderr: errOut})
	if !strings.Contains(errOut.String(), "hello") {
		t.Fatalf("verbose stderr should contain the command, got %q", errOut.String())
	}
}
