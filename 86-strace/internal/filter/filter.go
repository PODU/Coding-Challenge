// Package filter implements -e, which picks the syscalls to trace.
package filter

import (
	"fmt"
	"sort"
	"strings"

	"mystrace/internal/syscalls"
)

// classes are the names usable in -e trace=CLASS. Only syscalls in our table
// are listed.
var classes = map[string][]string{
	"file": {
		"open", "openat", "stat", "lstat", "newfstatat", "access", "faccessat",
		"readlink", "unlink", "mkdir", "chdir", "getcwd", "statfs", "execve",
		"rename",
	},
	"process": {
		"fork", "vfork", "clone", "execve", "exit", "exit_group", "wait4",
		"kill", "getpid",
	},
	"network": {"socket", "connect", "accept", "bind", "listen", "sendto", "recvfrom"},
	"memory":  {"mmap", "munmap", "mprotect", "brk", "madvise"},
	"signal":  {"rt_sigaction", "rt_sigprocmask", "kill", "rt_sigreturn"},
	"desc": {
		"read", "write", "close", "open", "openat", "dup", "dup2", "fcntl",
		"ioctl", "lseek", "pread64", "pwrite64", "readv", "writev",
		"getdents64", "getdents", "pipe",
	},
}

// Filter decides which syscalls are shown.
type Filter struct {
	all    bool // no -e given
	negate bool // trace everything except names
	names  map[string]bool
}

// Parse builds a Filter from the -e values. With none, everything is traced.
// Each value can look like:
//
//	trace=open,read,write
//	open,read,write
//	trace=!close
//	trace=file
func Parse(exprs []string) (*Filter, error) {
	f := &Filter{names: map[string]bool{}}
	if len(exprs) == 0 {
		f.all = true
		return f, nil
	}
	for _, expr := range exprs {
		body := strings.TrimPrefix(expr, "trace=")
		if body == "" {
			return nil, fmt.Errorf("empty -e expression %q", expr)
		}
		for _, tok := range strings.Split(body, ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			if strings.HasPrefix(tok, "!") {
				f.negate = true
				tok = tok[1:]
			}
			if members, ok := classes[tok]; ok {
				for _, m := range members {
					f.names[m] = true
				}
				continue
			}
			if syscalls.IsKnownName(tok) {
				f.names[tok] = true
				continue
			}
			return nil, fmt.Errorf("unknown syscall or class %q (valid classes: %s)", tok, validClasses())
		}
	}
	if len(f.names) == 0 {
		f.all = true
	}
	return f, nil
}

func validClasses() string {
	names := make([]string, 0, len(classes))
	for k := range classes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ShouldTrace reports whether calls to name should be shown.
func (f *Filter) ShouldTrace(name string) bool {
	if f.all {
		return true
	}
	if f.negate {
		return !f.names[name]
	}
	return f.names[name]
}
