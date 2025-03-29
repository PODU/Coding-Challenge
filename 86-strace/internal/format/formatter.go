// Package format writes trace lines in strace's style.
package format

import (
	"fmt"
	"strings"
)

// Line renders one syscall as "name(arg0, arg1, ...) = ret".
func Line(name string, args []string, ret string) string {
	return fmt.Sprintf("%s(%s) = %s", name, strings.Join(args, ", "), ret)
}

// ExitLine renders "+++ exited with N +++".
func ExitLine(code int) string {
	return fmt.Sprintf("+++ exited with %d +++", code)
}

// SignalLine renders "--- SIGNAME ---".
func SignalLine(sig string) string {
	return fmt.Sprintf("--- %s ---", sig)
}
