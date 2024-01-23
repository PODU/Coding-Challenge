//go:build !windows

package scanner

import "syscall"

// connRefusedErrnos are the errnos for a refused connection (an RST).
var connRefusedErrnos = []syscall.Errno{syscall.ECONNREFUSED}
