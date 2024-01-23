//go:build windows

package scanner

import "syscall"

// wsaeconnrefused is Winsock's WSAECONNREFUSED, which package syscall doesn't
// define.
const wsaeconnrefused = syscall.Errno(10061)

// connRefusedErrnos are the errnos for a refused connection (an RST). Windows
// dials report WSAECONNREFUSED, not syscall.ECONNREFUSED.
var connRefusedErrnos = []syscall.Errno{wsaeconnrefused, syscall.ECONNREFUSED}
