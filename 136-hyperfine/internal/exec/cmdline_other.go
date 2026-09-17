//go:build !windows

package exec

import osexec "os/exec"

// setRawCommandLine is a no-op outside Windows: execve takes an argument
// vector, so there is no command line to rewrite.
func setRawCommandLine(cmd *osexec.Cmd, line string) {}
