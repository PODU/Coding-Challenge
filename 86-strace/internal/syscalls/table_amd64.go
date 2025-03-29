package syscalls

// table describes the x86-64 syscalls we know how to decode: the common file,
// process, memory and I/O calls. Anything else is printed as syscall_0x<nr>
// with raw hex arguments.
var table = map[int]Syscall{
	0:   {Name: "read", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	1:   {Name: "write", Args: []ArgType{ArgFd, ArgStr, ArgInt}},
	2:   {Name: "open", Args: []ArgType{ArgStr, ArgOpenFl, ArgMode}},
	3:   {Name: "close", Args: []ArgType{ArgFd}},
	4:   {Name: "stat", Args: []ArgType{ArgStr, ArgPtr}},
	5:   {Name: "fstat", Args: []ArgType{ArgFd, ArgPtr}},
	6:   {Name: "lstat", Args: []ArgType{ArgStr, ArgPtr}},
	7:   {Name: "poll", Args: []ArgType{ArgPtr, ArgInt, ArgInt}},
	8:   {Name: "lseek", Args: []ArgType{ArgFd, ArgLong, ArgInt}},
	9:   {Name: "mmap", Args: []ArgType{ArgPtr, ArgInt, ArgProt, ArgMap, ArgFd, ArgLong}, Ret: ArgPtr},
	10:  {Name: "mprotect", Args: []ArgType{ArgPtr, ArgInt, ArgProt}},
	11:  {Name: "munmap", Args: []ArgType{ArgPtr, ArgInt}},
	12:  {Name: "brk", Args: []ArgType{ArgPtr}, Ret: ArgPtr},
	13:  {Name: "rt_sigaction", Args: []ArgType{ArgSignal, ArgPtr, ArgPtr, ArgInt}},
	14:  {Name: "rt_sigprocmask", Args: []ArgType{ArgInt, ArgPtr, ArgPtr, ArgInt}},
	16:  {Name: "ioctl", Args: []ArgType{ArgFd, ArgInt, ArgPtr}},
	17:  {Name: "pread64", Args: []ArgType{ArgFd, ArgPtr, ArgInt, ArgLong}},
	18:  {Name: "pwrite64", Args: []ArgType{ArgFd, ArgStr, ArgInt, ArgLong}},
	19:  {Name: "readv", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	20:  {Name: "writev", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	21:  {Name: "access", Args: []ArgType{ArgStr, ArgMode}},
	22:  {Name: "pipe", Args: []ArgType{ArgPtr}},
	23:  {Name: "select", Args: []ArgType{ArgInt, ArgPtr, ArgPtr, ArgPtr, ArgPtr}},
	28:  {Name: "madvise", Args: []ArgType{ArgPtr, ArgInt, ArgInt}},
	32:  {Name: "dup", Args: []ArgType{ArgFd}},
	33:  {Name: "dup2", Args: []ArgType{ArgFd, ArgFd}},
	35:  {Name: "nanosleep", Args: []ArgType{ArgPtr, ArgPtr}},
	39:  {Name: "getpid", Args: []ArgType{}},
	41:  {Name: "socket", Args: []ArgType{ArgInt, ArgInt, ArgInt}},
	42:  {Name: "connect", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	56:  {Name: "clone", Args: []ArgType{ArgPtr, ArgPtr, ArgInt, ArgPtr}},
	57:  {Name: "fork", Args: []ArgType{}},
	58:  {Name: "vfork", Args: []ArgType{}},
	59:  {Name: "execve", Args: []ArgType{ArgStr, ArgPtr, ArgPtr}},
	60:  {Name: "exit", Args: []ArgType{ArgInt}},
	61:  {Name: "wait4", Args: []ArgType{ArgInt, ArgPtr, ArgInt, ArgPtr}},
	62:  {Name: "kill", Args: []ArgType{ArgInt, ArgSignal}},
	63:  {Name: "uname", Args: []ArgType{ArgPtr}},
	72:  {Name: "fcntl", Args: []ArgType{ArgFd, ArgInt, ArgLong}},
	78:  {Name: "getdents", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	79:  {Name: "getcwd", Args: []ArgType{ArgPtr, ArgInt}},
	80:  {Name: "chdir", Args: []ArgType{ArgStr}},
	83:  {Name: "mkdir", Args: []ArgType{ArgStr, ArgMode}},
	87:  {Name: "unlink", Args: []ArgType{ArgStr}},
	89:  {Name: "readlink", Args: []ArgType{ArgStr, ArgPtr, ArgInt}},
	96:  {Name: "gettimeofday", Args: []ArgType{ArgPtr, ArgPtr}},
	97:  {Name: "getrlimit", Args: []ArgType{ArgInt, ArgPtr}},
	99:  {Name: "sysinfo", Args: []ArgType{ArgPtr}},
	102: {Name: "getuid", Args: []ArgType{}},
	104: {Name: "getgid", Args: []ArgType{}},
	107: {Name: "geteuid", Args: []ArgType{}},
	108: {Name: "getegid", Args: []ArgType{}},
	137: {Name: "statfs", Args: []ArgType{ArgStr, ArgPtr}},
	158: {Name: "arch_prctl", Args: []ArgType{ArgInt, ArgPtr}},
	202: {Name: "futex", Args: []ArgType{ArgPtr, ArgInt, ArgInt, ArgPtr, ArgPtr, ArgInt}},
	217: {Name: "getdents64", Args: []ArgType{ArgFd, ArgPtr, ArgInt}},
	218: {Name: "set_tid_address", Args: []ArgType{ArgPtr}},
	228: {Name: "clock_gettime", Args: []ArgType{ArgInt, ArgPtr}},
	231: {Name: "exit_group", Args: []ArgType{ArgInt}},
	257: {Name: "openat", Args: []ArgType{ArgAtFd, ArgStr, ArgOpenFl, ArgMode}},
	262: {Name: "newfstatat", Args: []ArgType{ArgAtFd, ArgStr, ArgPtr, ArgInt}},
	269: {Name: "faccessat", Args: []ArgType{ArgAtFd, ArgStr, ArgMode}},
	273: {Name: "set_robust_list", Args: []ArgType{ArgPtr, ArgInt}},
	302: {Name: "prlimit64", Args: []ArgType{ArgInt, ArgInt, ArgPtr, ArgPtr}},
	318: {Name: "getrandom", Args: []ArgType{ArgPtr, ArgInt, ArgInt}},
	334: {Name: "rseq", Args: []ArgType{ArgPtr, ArgInt, ArgInt, ArgInt}},
}

// nameSet lets the -e filter check syscall names.
var nameSet = func() map[string]bool {
	m := make(map[string]bool, len(table))
	for _, s := range table {
		m[s.Name] = true
	}
	return m
}()

// IsKnownName reports whether name is in the table.
func IsKnownName(name string) bool { return nameSet[name] }
