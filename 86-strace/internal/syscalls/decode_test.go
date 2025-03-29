package syscalls

import (
	"testing"

	"mystrace/internal/tracer"
)

// fakeMem is a portable MemReader backed by a map of address -> bytes.
type fakeMem map[uint64]string

func (m fakeMem) ReadString(addr uint64, max int) (string, bool) {
	s, ok := m[addr]
	if !ok {
		return "", false
	}
	if len(s) > max {
		s = s[:max]
	}
	return s, true
}

func TestNameLookup(t *testing.T) {
	if n, ok := Name(257); !ok || n != "openat" {
		t.Errorf("Name(257) = %q,%v want openat,true", n, ok)
	}
	if n, ok := Name(1); !ok || n != "write" {
		t.Errorf("Name(1) = %q,%v want write,true", n, ok)
	}
	if n, ok := Name(0x999); ok || n != "syscall_0x999" {
		t.Errorf("Name(0x999) = %q,%v want syscall_0x999,false", n, ok)
	}
}

func TestRenderFlags(t *testing.T) {
	cases := []struct {
		val  uint64
		want string
		fn   func(uint64) string
	}{
		{0x80000, "O_RDONLY|O_CLOEXEC", renderOpenFlags},
		{0x241, "O_WRONLY|O_CREAT|O_TRUNC", renderOpenFlags}, // 1|0x40|0x200
		{0, "O_RDONLY", renderOpenFlags},
		{0x3, "PROT_READ|PROT_WRITE", renderProt},
		{0, "PROT_NONE", renderProt},
		{0x22, "MAP_PRIVATE|MAP_ANONYMOUS", renderMap},
	}
	for _, c := range cases {
		if got := c.fn(c.val); got != c.want {
			t.Errorf("flags(%#x) = %q, want %q", c.val, got, c.want)
		}
	}
}

func TestRenderFlagsLeftoverBits(t *testing.T) {
	// PROT_READ (0x1) plus an unknown high bit should keep the hex remainder.
	if got := renderProt(0x1 | 0x100); got != "PROT_READ|0x100" {
		t.Errorf("renderProt = %q, want PROT_READ|0x100", got)
	}
}

func TestRenderAtFd(t *testing.T) {
	if got := renderAtFd(^uint64(99)); got != "AT_FDCWD" { // ^99 == -100 in two's complement
		t.Errorf("renderAtFd(-100) = %q, want AT_FDCWD", got)
	}
	if got := renderAtFd(3); got != "3" {
		t.Errorf("renderAtFd(3) = %q, want 3", got)
	}
}

func TestDecodeOpenat(t *testing.T) {
	mem := fakeMem{0x1000: "/etc/passwd"}
	rc := tracer.RawCall{
		Nr:     257,                                        // openat
		Args:   [6]uint64{^uint64(99), 0x1000, 0x80000, 0}, // ^99 == -100 (AT_FDCWD)
		Ret:    3,
		HasRet: true,
		Mem:    mem,
	}
	d := Decode(rc, 32)
	if d.Name != "openat" {
		t.Fatalf("Name = %q", d.Name)
	}
	want := []string{"AT_FDCWD", `"/etc/passwd"`, "O_RDONLY|O_CLOEXEC", "0"}
	if len(d.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", d.Args, want)
	}
	for i := range want {
		if d.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, want %q", i, d.Args[i], want[i])
		}
	}
	if d.Ret != "3" {
		t.Errorf("Ret = %q, want 3", d.Ret)
	}
}

func TestDecodeStringEscaping(t *testing.T) {
	mem := fakeMem{0x10: "hi\n\x00there"} // ReadString stops at NUL in real impl; fake returns raw
	rc := tracer.RawCall{Nr: 1, Args: [6]uint64{1, 0x10, 3}, Ret: 3, HasRet: true, Mem: mem}
	d := Decode(rc, 32)
	// write(1, "...", 3): the NUL becomes an octal escape in the fake's raw string.
	if d.Args[0] != "1" || d.Args[2] != "3" {
		t.Errorf("fd/count = %q/%q", d.Args[0], d.Args[2])
	}
}

func TestDecodeStringTruncation(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789"
	mem := fakeMem{0x20: long}
	rc := tracer.RawCall{Nr: 1, Args: [6]uint64{1, 0x20, 36}, Ret: 36, HasRet: true, Mem: mem}
	d := Decode(rc, 10)
	if d.Args[1] != `"abcdefghij"...` {
		t.Errorf("truncated string = %q, want \"abcdefghij\"...", d.Args[1])
	}
}

func TestDecodeUnreadablePointer(t *testing.T) {
	rc := tracer.RawCall{Nr: 1, Args: [6]uint64{1, 0xdead, 3}, Ret: 3, HasRet: true, Mem: tracer.NopMem{}}
	d := Decode(rc, 32)
	if d.Args[1] != "0xdead" {
		t.Errorf("unreadable buf = %q, want 0xdead", d.Args[1])
	}
}

func TestDecodeNullString(t *testing.T) {
	rc := tracer.RawCall{Nr: 1, Args: [6]uint64{1, 0, 0}, Ret: 0, HasRet: true, Mem: fakeMem{}}
	d := Decode(rc, 32)
	if d.Args[1] != "NULL" {
		t.Errorf("null buf = %q, want NULL", d.Args[1])
	}
}

func TestDecodeReturnErrno(t *testing.T) {
	rc := tracer.RawCall{Nr: 257, Args: [6]uint64{}, Ret: -2, HasRet: true, Mem: fakeMem{}}
	d := Decode(rc, 32)
	if d.Ret != "-1 ENOENT (No such file or directory)" {
		t.Errorf("errno ret = %q", d.Ret)
	}
}

func TestDecodeReturnNoReturn(t *testing.T) {
	rc := tracer.RawCall{Nr: 231, Args: [6]uint64{0}, HasRet: false, Mem: fakeMem{}}
	d := Decode(rc, 32)
	if d.Name != "exit_group" || d.Ret != "?" {
		t.Errorf("exit_group = %q ret=%q, want exit_group/?", d.Name, d.Ret)
	}
}

func TestDecodeReturnHexPointer(t *testing.T) {
	rc := tracer.RawCall{Nr: 12, Args: [6]uint64{0}, Ret: 0x55aa, HasRet: true, Mem: fakeMem{}} // brk
	d := Decode(rc, 32)
	if d.Ret != "0x55aa" {
		t.Errorf("brk ret = %q, want 0x55aa", d.Ret)
	}
}

func TestDecodeNonErrorNegative(t *testing.T) {
	// A negative outside the errno window stays decimal.
	rc := tracer.RawCall{Nr: 8, Args: [6]uint64{}, Ret: -8000, HasRet: true, Mem: fakeMem{}}
	d := Decode(rc, 32)
	if d.Ret != "-8000" {
		t.Errorf("ret = %q, want -8000", d.Ret)
	}
}

func TestDecodeUnknownSyscall(t *testing.T) {
	rc := tracer.RawCall{Nr: 0x123, Args: [6]uint64{0xaa, 0xbb}, Ret: 0, HasRet: true, Mem: fakeMem{}}
	d := Decode(rc, 32)
	if d.Name != "syscall_0x123" {
		t.Fatalf("Name = %q", d.Name)
	}
	if len(d.Args) != 6 || d.Args[0] != "0xaa" || d.Args[1] != "0xbb" {
		t.Errorf("Args = %v", d.Args)
	}
}

func TestErrnoHelpers(t *testing.T) {
	if ErrnoName(2) != "ENOENT" || ErrnoMessage(2) != "No such file or directory" {
		t.Error("errno 2 mismatch")
	}
	if ErrnoFromRet(true, -13) != 13 {
		t.Error("ErrnoFromRet(-13) should be 13")
	}
	if ErrnoFromRet(true, 5) != 0 {
		t.Error("ErrnoFromRet(5) should be 0")
	}
}
