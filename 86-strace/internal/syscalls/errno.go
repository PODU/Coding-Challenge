package syscalls

type errnoEntry struct {
	name string
	msg  string
}

var errnoTable = map[int]errnoEntry{
	1:   {"EPERM", "Operation not permitted"},
	2:   {"ENOENT", "No such file or directory"},
	3:   {"ESRCH", "No such process"},
	4:   {"EINTR", "Interrupted system call"},
	5:   {"EIO", "Input/output error"},
	6:   {"ENXIO", "No such device or address"},
	7:   {"E2BIG", "Argument list too long"},
	8:   {"ENOEXEC", "Exec format error"},
	9:   {"EBADF", "Bad file descriptor"},
	10:  {"ECHILD", "No child processes"},
	11:  {"EAGAIN", "Resource temporarily unavailable"},
	12:  {"ENOMEM", "Cannot allocate memory"},
	13:  {"EACCES", "Permission denied"},
	14:  {"EFAULT", "Bad address"},
	16:  {"EBUSY", "Device or resource busy"},
	17:  {"EEXIST", "File exists"},
	18:  {"EXDEV", "Invalid cross-device link"},
	19:  {"ENODEV", "No such device"},
	20:  {"ENOTDIR", "Not a directory"},
	21:  {"EISDIR", "Is a directory"},
	22:  {"EINVAL", "Invalid argument"},
	23:  {"ENFILE", "Too many open files in system"},
	24:  {"EMFILE", "Too many open files"},
	25:  {"ENOTTY", "Inappropriate ioctl for device"},
	26:  {"ETXTBSY", "Text file busy"},
	27:  {"EFBIG", "File too large"},
	28:  {"ENOSPC", "No space left on device"},
	29:  {"ESPIPE", "Illegal seek"},
	30:  {"EROFS", "Read-only file system"},
	31:  {"EMLINK", "Too many links"},
	32:  {"EPIPE", "Broken pipe"},
	34:  {"ERANGE", "Numerical result out of range"},
	36:  {"ENAMETOOLONG", "File name too long"},
	38:  {"ENOSYS", "Function not implemented"},
	39:  {"ENOTEMPTY", "Directory not empty"},
	40:  {"ELOOP", "Too many levels of symbolic links"},
	98:  {"EADDRINUSE", "Address already in use"},
	103: {"ECONNABORTED", "Software caused connection abort"},
	104: {"ECONNRESET", "Connection reset by peer"},
	110: {"ETIMEDOUT", "Connection timed out"},
	111: {"ECONNREFUSED", "Connection refused"},
	115: {"EINPROGRESS", "Operation now in progress"},
}

// ErrnoName returns the name of errno n, e.g. "ENOENT", or "" if unknown.
func ErrnoName(n int) string {
	if e, ok := errnoTable[n]; ok {
		return e.name
	}
	return ""
}

// ErrnoMessage returns the strerror text for n, or "" if unknown.
func ErrnoMessage(n int) string {
	if e, ok := errnoTable[n]; ok {
		return e.msg
	}
	return ""
}

// ErrnoFromRet returns the errno for a failed call, or 0. The kernel returns
// errors as -errno, and only values from -4095 to -1 mean failure.
func ErrnoFromRet(hasRet bool, ret int64) int {
	if hasRet && ret < 0 && ret >= -4095 {
		return int(-ret)
	}
	return 0
}
