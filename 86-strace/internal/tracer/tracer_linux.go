//go:build linux

package tracer

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"mystrace/internal/cli"
)

// linuxTracer traces with ptrace(2).
type linuxTracer struct {
	cfg cli.Config
}

// New returns a Tracer for cfg.
func New(cfg cli.Config) (Tracer, error) {
	return &linuxTracer{cfg: cfg}, nil
}

// ptraceMem reads tracee memory one word at a time via PTRACE_PEEKDATA.
type ptraceMem struct{ pid int }

func (m ptraceMem) ReadString(addr uint64, max int) (string, bool) {
	if addr == 0 || max <= 0 {
		return "", false
	}
	out := make([]byte, 0, max)
	word := make([]byte, 8)
	for len(out) < max {
		n, err := syscall.PtracePeekData(m.pid, uintptr(addr)+uintptr(len(out)), word)
		if err != nil || n == 0 {
			if len(out) == 0 {
				return "", false
			}
			return string(out), true
		}
		for i := 0; i < n; i++ {
			if word[i] == 0 {
				return string(out), true
			}
			out = append(out, word[i])
			if len(out) >= max {
				return string(out), true
			}
		}
	}
	return string(out), true
}

// noReturn reports whether nr is exit or exit_group. Those never return, so
// they are reported on entry.
func noReturn(nr int) bool { return nr == 60 || nr == 231 }

func buildCall(nr int, r *syscall.PtraceRegs, hasRet bool, ret int64, pid int, startNs, durNs int64) RawCall {
	return RawCall{
		Nr:      nr,
		Args:    [6]uint64{r.Rdi, r.Rsi, r.Rdx, r.R10, r.R8, r.R9},
		Ret:     ret,
		HasRet:  hasRet,
		Mem:     ptraceMem{pid: pid},
		StartNs: startNs,
		DurNs:   durNs,
	}
}

func (t *linuxTracer) Run(handler func(RawCall)) (int, error) {
	// The kernel only accepts ptrace requests from the thread that attached.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var pid int
	if t.cfg.HasPID {
		pid = t.cfg.PID
		if err := syscall.PtraceAttach(pid); err != nil {
			return 1, fmt.Errorf("attach to PID %d: %w", pid, err)
		}
		defer func() { _ = syscall.PtraceDetach(pid) }()
		var ws syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
			return 1, fmt.Errorf("wait for PID %d: %w", pid, err)
		}
	} else {
		cmd := exec.Command(t.cfg.Command[0], t.cfg.Command[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Ptrace: true}
		if err := cmd.Start(); err != nil {
			return 1, fmt.Errorf("launch %s: %w", t.cfg.Command[0], err)
		}
		pid = cmd.Process.Pid
		// The child stops on its initial exec because of PTRACE_TRACEME.
		var ws syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
			return 1, fmt.Errorf("wait for child: %w", err)
		}
	}

	// Makes syscall stops show up as SIGTRAP|0x80, distinct from real SIGTRAPs.
	_ = syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACESYSGOOD)

	entry := true
	var entryRegs syscall.PtraceRegs
	var startNs int64
	deliver := 0

	for {
		if err := syscall.PtraceSyscall(pid, deliver); err != nil {
			return 0, nil
		}
		deliver = 0

		var ws syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
			return 1, fmt.Errorf("wait: %w", err)
		}

		switch {
		case ws.Exited():
			return ws.ExitStatus(), nil
		case ws.Signaled():
			return 128 + int(ws.Signal()), nil
		case ws.Stopped():
			st := ws.StopSignal()
			if st != syscall.SIGTRAP|0x80 {
				// A real signal: pass it on when we resume.
				deliver = int(st)
				continue
			}
		}

		var regs syscall.PtraceRegs
		if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
			return 1, fmt.Errorf("getregs: %w", err)
		}
		nr := int(regs.Orig_rax)

		if entry {
			entryRegs = regs
			startNs = time.Now().UnixNano()
			if noReturn(nr) {
				handler(buildCall(nr, &regs, false, 0, pid, startNs, 0))
			}
			entry = false
		} else {
			durNs := time.Now().UnixNano() - startNs
			handler(buildCall(nr, &entryRegs, true, int64(regs.Rax), pid, startNs, durNs))
			entry = true
		}
	}
}
