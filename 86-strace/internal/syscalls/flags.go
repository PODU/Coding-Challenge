package syscalls

import (
	"fmt"
	"strings"
)

// flagDef maps a bitmask to its symbolic name. Tables are ordered so that
// composite masks are matched before the single bits they contain.
type flagDef struct {
	mask uint64
	name string
}

// renderFlags ORs together the names of all set bits, appending any leftover
// bits as a hex remainder, matching strace's behaviour.
func renderFlags(val uint64, defs []flagDef) string {
	if val == 0 {
		return "0"
	}
	var parts []string
	rem := val
	for _, d := range defs {
		if d.mask == 0 {
			continue
		}
		if rem&d.mask == d.mask {
			parts = append(parts, d.name)
			rem &^= d.mask
		}
	}
	if rem != 0 {
		parts = append(parts, fmt.Sprintf("%#x", rem))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%#x", val)
	}
	return strings.Join(parts, "|")
}

// open(2) access modes occupy the low two bits and are always rendered.
const oAccMode = 0x3

var openFlagDefs = []flagDef{
	{0x40, "O_CREAT"},
	{0x80, "O_EXCL"},
	{0x100, "O_NOCTTY"},
	{0x200, "O_TRUNC"},
	{0x400, "O_APPEND"},
	{0x800, "O_NONBLOCK"},
	{0x1000, "O_DSYNC"},
	{0x4000, "O_DIRECT"},
	{0x8000, "O_LARGEFILE"},
	{0x10000, "O_DIRECTORY"},
	{0x20000, "O_NOFOLLOW"},
	{0x40000, "O_NOATIME"},
	{0x80000, "O_CLOEXEC"},
}

func renderOpenFlags(val uint64) string {
	var acc string
	switch val & oAccMode {
	case 0:
		acc = "O_RDONLY"
	case 1:
		acc = "O_WRONLY"
	case 2:
		acc = "O_RDWR"
	default:
		acc = fmt.Sprintf("%#x", val&oAccMode)
	}
	rest := val &^ oAccMode
	if rest == 0 {
		return acc
	}
	return acc + "|" + renderFlags(rest, openFlagDefs)
}

var protFlagDefs = []flagDef{
	{0x1, "PROT_READ"},
	{0x2, "PROT_WRITE"},
	{0x4, "PROT_EXEC"},
}

func renderProt(val uint64) string {
	if val == 0 {
		return "PROT_NONE"
	}
	return renderFlags(val, protFlagDefs)
}

var mapFlagDefs = []flagDef{
	{0x1, "MAP_SHARED"},
	{0x2, "MAP_PRIVATE"},
	{0x10, "MAP_FIXED"},
	{0x20, "MAP_ANONYMOUS"},
	{0x100, "MAP_GROWSDOWN"},
	{0x800, "MAP_DENYWRITE"},
	{0x1000, "MAP_EXECUTABLE"},
	{0x2000, "MAP_LOCKED"},
	{0x4000, "MAP_NORESERVE"},
	{0x8000, "MAP_POPULATE"},
	{0x20000, "MAP_STACK"},
}

func renderMap(val uint64) string {
	return renderFlags(val, mapFlagDefs)
}

// atFDCWD is the dirfd value meaning "relative to the current directory".
const atFDCWD = -100

func renderAtFd(val uint64) string {
	if int32(uint32(val)) == atFDCWD {
		return "AT_FDCWD"
	}
	return fmt.Sprintf("%d", int32(uint32(val)))
}

var signalNames = map[int]string{
	1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 5: "SIGTRAP",
	6: "SIGABRT", 7: "SIGBUS", 8: "SIGFPE", 9: "SIGKILL", 10: "SIGUSR1",
	11: "SIGSEGV", 12: "SIGUSR2", 13: "SIGPIPE", 14: "SIGALRM", 15: "SIGTERM",
	17: "SIGCHLD", 18: "SIGCONT", 19: "SIGSTOP", 20: "SIGTSTP", 21: "SIGTTIN",
	22: "SIGTTOU", 23: "SIGURG", 24: "SIGXCPU", 25: "SIGXFSZ", 26: "SIGVTALRM",
	27: "SIGPROF", 28: "SIGWINCH", 29: "SIGIO", 30: "SIGPWR", 31: "SIGSYS",
}

// SignalName returns the SIGxxx name for a signal number, or "" if unknown.
func SignalName(sig int) string { return signalNames[sig] }

func renderSignal(val uint64) string {
	if n, ok := signalNames[int(val)]; ok {
		return n
	}
	return fmt.Sprintf("%d", int(val))
}
