# tail — Coding Challenge #141

`cctail`, a reimplementation of Unix `tail` in Go. It prints the end of files,
follows them as they grow, survives truncation and log rotation, and wakes on
OS file-change notifications (inotify, kqueue, ReadDirectoryChangesW). If none
of those is available it polls instead.

Source: https://codingchallenges.substack.com/p/coding-challenge-141-tail

## Build & test

```sh
go build -o cctail .     # cctail.exe on Windows
go vet ./...
go test ./...
```

Requires Go 1.24+. Standard library only, no third-party dependencies.

## Usage

```
cctail [OPTION]... [FILE]...

  -n, --lines=[+]NUM   last NUM lines (default 10); +NUM = from line NUM
  -c, --bytes=[+]NUM   last NUM bytes;              +NUM = from byte NUM
  -f, --follow[=HOW]   keep printing appended data; HOW = descriptor | name
  -F                   --follow=name --retry (survives log rotation)
      --retry          keep retrying files that are missing
  -q, --quiet          never print '==> FILE <==' headers
  -v, --verbose        always print headers
  -s, --sleep-interval=N  poll interval with -f (default 1s; safety net
                          when notifications are active)
      --pid=PID        with -f, exit once process PID has died
      --poll           with -f, disable OS notifications
  -NUM                 obsolete form of -n NUM
```

NUM accepts suffixes `b` (512), `kB`/`K`, `MB`/`M`, `GB`/`G`.
No FILE, or `-`, means standard input.

Exit status: `0` OK; `1` when a file could not be read or on a usage error;
`130` when following is stopped by Ctrl+C or SIGTERM.

### Examples (the challenge's steps)

```sh
seq 1 15 > test.log; seq 100 115 > test2.log; printf 'abcdefghij' > bytes.log

cctail test.log                 # 6..15                         (step 1)
cctail -n 5 test.log            # 11..15                        (step 2)
cctail -n +5 test.log           # 5..15                         (step 3)
cctail -c 4 bytes.log           # ghij, no newline              (step 4)
cctail -n 10 big.log            # instant on a 5M-line file     (step 5)
seq 1 100 | cctail -n 3         # 98..100                       (step 6)
cctail test.log test2.log       # with ==> headers <==          (step 7)
cctail -f test.log              # follow appends                (steps 8-9)
cctail -F test.log              # follow across mv / recreate   (steps 10-11)
cctail -f test.log test2.log    # header when active file changes (step 12)
cctail -q -f test.log test2.log # interleaved, no headers       (step 13)
```

Diagnostics go to stderr, as in GNU tail:

```
cctail: test.log: file truncated
cctail: 'test.log' has become inaccessible: No such file or directory
cctail: 'test.log' has appeared; following new file
cctail: 'test.log' has been replaced; following new file
```

## Verification

- Every non-follow example above, plus edge cases (`-n 0`, `-n +0`, missing
  trailing newline, empty files, `-c +N`, stdin, `< file` redirection,
  `-NUM`), was compared byte-for-byte with GNU coreutils `tail`. All match.
- Tailing 3 lines of a 39 MB, 5,000,000-line file takes about 20 ms. A test
  checks that finding the last 10 lines of a 6 MB file reads no more than one
  64 KiB chunk.
- `go test`: 17 tests. They cover the backward scan against a reference
  implementation at chunk boundaries, the stream ring buffers, option
  parsing, and live `-f` scenarios on real temp files: append (with the
  watcher and with `--poll`), truncation, `-F` rotation, `-f` staying on the
  renamed file, a missing file appearing later, multi-file headers, `-q`,
  `--pid`, `-f` on a pipe, and prompt exit on cancellation. The watcher test
  uses a 5 s poll interval and requires delivery in under 1 s, so it fails if
  the notification path stops working.
- The runtime tests ran on Windows (ReadDirectoryChangesW). The Linux
  (inotify) and BSD/macOS (kqueue) backends compile and pass `go vet` for
  each GOOS, but have not been run on those systems here.
