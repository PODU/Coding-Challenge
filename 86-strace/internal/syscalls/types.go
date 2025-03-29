// Package syscalls turns raw x86-64 syscall numbers and registers into
// readable names, arguments and results.
package syscalls

// ArgType says how to display an argument.
type ArgType int

const (
	ArgNone   ArgType = iota
	ArgInt            // signed decimal
	ArgLong           // signed decimal (64-bit)
	ArgPtr            // hex pointer (0x...), NULL when zero
	ArgStr            // C string read from tracee memory
	ArgFd             // file descriptor (decimal)
	ArgOpenFl         // open(2) flags: O_RDONLY|O_CLOEXEC|...
	ArgProt           // mmap/mprotect prot: PROT_READ|PROT_WRITE|...
	ArgMap            // mmap flags: MAP_PRIVATE|MAP_ANONYMOUS|...
	ArgMode           // file mode, octal (0644)
	ArgSignal         // signal number -> SIGxxx
	ArgAtFd           // dirfd that may be AT_FDCWD
)

// Syscall is one entry in the syscall table.
type Syscall struct {
	Name string
	Args []ArgType
	Ret  ArgType // ArgPtr prints the result in hex (mmap, brk); default is decimal
}

// Decoded is a call ready to print.
type Decoded struct {
	Name string
	Args []string
	Ret  string
}
