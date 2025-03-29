# mystrace — Coding Challenge #86

A working subset of [`strace(1)`](https://man7.org/linux/man-pages/man1/strace.1.html)
written in Go. It launches a command (or attaches to a running PID), intercepts
every Linux system call via `ptrace(2)`, decodes the syscall name, arguments,
and return value, and prints `strace`-style trace lines:

```
openat(AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC) = 3
read(3, "\177ELF\2\1\1\0\0\0\0\0\0\0\0\0"..., 832) = 832
write(1, "hello\n", 6) = 6
exit_group(0) = ?
+++ exited with 0 +++
```

## Platform support: live tracing is Linux-only

`ptrace(2)` is a Linux kernel facility. The live tracing event loop therefore
**only runs on Linux** (`linux/amd64`, the primary ABI). It lives in
`internal/tracer/tracer_linux.go` behind a `//go:build linux` constraint.

On every other OS (Windows, macOS) a stub (`internal/tracer/tracer_stub.go`)
takes its place: the program still **builds and runs**, but invoking live
tracing prints `live tracing requires Linux` and exits non-zero.

Crucially, all the **portable** logic is OS-independent and fully unit-tested on
any host, including the Windows build machine:

- syscall number → name table (`internal/syscalls/table_amd64.go`)
- argument, flag, errno, and string decoders (`internal/syscalls`)
- the trace-line formatter (`internal/format`)
- the `-e` filter parser and matcher (`internal/filter`)
- the `-c` summary aggregator and table renderer (`internal/summary`)
- the CLI parser (`internal/cli`)

This is achieved with a single portability seam: the Linux loop emits plain
`tracer.RawCall` data structs, and the portable layers consume those structs.
Tests feed hand-built `RawCall` fixtures and a fake `MemReader`, so the decode /
format / filter / summary paths are exercised without a live process.

## Build & test

```bash
# Portable: builds everywhere, runs the full portable test suite (Windows/macOS/Linux)
go build ./...
go test ./...

# Cross-compile the real Linux tracer from any host (verifies the ptrace path compiles)
GOOS=linux GOARCH=amd64 go build -o mystrace ./cmd/mystrace
```

> The race detector (`go test -race`) needs a 64-bit C toolchain. It is not a
> requirement for the portable suite; plain `go test ./...` is the gate.

## Usage

```
mystrace [options] command [args...]
mystrace [options] -p PID

Options:
  -p PID     Attach to and trace a running process (exclusive with a command).
  -e EXPR    Filter syscalls. Forms:
               -e trace=open,read,write
               -e open,read,write        (shorthand)
               -e trace=!close           (negation: all except close)
               -e trace=file             (class: file/process/network/memory/signal/desc)
             Repeatable; sets are unioned.
  -c         Print only a summary table (calls/errors/time) at exit.
  -o FILE    Write trace output to FILE instead of stderr.
  -s SIZE    Max printed string length before truncation (default 32).
  -f         (seam) Follow forks/clones — documented, not yet implemented.
  -h         Show help.
```

The exit status mirrors the tracee's exit status (or `128+signum` when the
tracee is killed by a signal). Usage/setup errors exit non-zero with a
diagnostic on stderr. The trace itself is written to stderr by default, matching
`strace`.

## Run (on Linux)

```bash
./mystrace /bin/echo hello
./mystrace -c /bin/ls
./mystrace -e trace=open,openat,read /bin/cat /etc/hostname
./mystrace -p 12345          # attach to a running process; detaches cleanly on exit
```

Example `-c` summary:

```
% time     seconds  usecs/call     calls    errors syscall
------ ----------- ----------- --------- --------- ----------------
 41.20    0.000123          12        10         2 openat
 ...
------ ----------- ----------- --------- --------- ----------------
100.00    0.000298                    57         3 total
```

## Scope

Ships a curated, extensible x86-64 syscall table covering the common
file/process/memory/IO path. Unknown syscall numbers print as
`syscall_0x<nr>(0x.., ...)` with raw hex arguments and still count under `-c`.
Multi-arch decoding beyond amd64, `-f` fork-following, and full timestamp modes
are documented seams (
