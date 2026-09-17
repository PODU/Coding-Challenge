//go:build windows

package exec

import (
	osexec "os/exec"
	"syscall"
)

// setRawCommandLine bypasses Go's MSVC-style argument quoting so cmd.exe
// receives the command line byte for byte.
func setRawCommandLine(cmd *osexec.Cmd, line string) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = line
}
