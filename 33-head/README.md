# cchead — Coding Challenge #33 (`head`)

A production-ready clone of the Unix `head` utility, written in Go. It prints
the beginning portion of one or more files (or standard input) — by default the
first 10 lines — and faithfully reproduces GNU/BSD `head` semantics including
byte counts, negative (all-but-last) counts, and multi-file headers.

Reference: [John Crickett's Coding Challenge #33](https://codingchallenges.substack.com/p/coding-challenge-33-head).

## Build

```sh
cd 33-head
go build -o cchead .      # produces ./cchead (cchead.exe on Windows)
```

Standard library only — no third-party dependencies.

## Usage

```
Usage: cchead [OPTION]... [FILE]...

Print the first part of FILEs to standard output. With no FILE, or when
FILE is '-', read standard input.

Options:
  -n, --lines=[-]N    print the first N lines (default 10);
                      with leading '-', print all but the last N lines
  -c, --bytes=[-]N    print the first N bytes;
                      with leading '-', print all but the last N bytes
  -q, --quiet, --silent   never print '==> FILE <==' headers
  -v, --verbose           always print headers
  -h, --help          display this help and exit
      --version       output version information and exit
```

Accepted flag spellings: `-n5`, `-n 5`, `-n=5`, `--lines=5`, `--lines 5` (and
the same for `-c`). Bare `-` means standard input; `--` ends option parsing.
Counts accept an optional `+`/`-` sign and the size suffixes `b` (×512),
`k` (×1024), `m` (×1024²).

## Examples

```sh
cchead test.txt                # first 10 lines of test.txt
cchead -n1 test.txt            # first line
cchead -n 5 a.txt b.txt        # first 5 lines of each, with ==> headers
cchead -c 31 test.txt          # first 31 bytes
cchead -n -3 test.txt          # all but the last 3 lines
cat test.txt | cchead -n 2     # first 2 lines from stdin
```

## Behavior notes

- **Default**: 10 lines when neither `-n` nor `-c` is given.
- **`-n` and `-c`** select line- vs byte-mode; if both appear, the **last one
  wins** (GNU behavior). Only one mode is ever active.
- **Negative counts** (`-n -K` / `-c -K`) print all but the last K lines/bytes.
  K greater than or equal to the total prints nothing. These cases buffer only a
  bounded window (K lines or K bytes), never the whole file.
- **Streaming**: positive counts stream and stop early — a multi-GB file read
  with `-n 5` does not load into memory.
- **Headers**: `==> NAME <==` is printed when more than one file is given (or
  with `-v`), suppressed with `-q`. A blank line separates consecutive sections.
  Standard input shows as `==> standard input <==`.
- **Byte counting** counts bytes, not runes: `-c` may split a multibyte UTF-8
  character mid-sequence, exactly like real `head -c`. Output is binary-safe.
- **Missing trailing newline**: the final partial line is emitted byte-for-byte
  with no newline added. CRLF endings are preserved (`\r` is data; a line ends
  at `\n`).

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | One or more files could not be opened/read (others still processed). |
| 2 | Usage error (bad flag, missing/invalid count). |

Diagnostics go to **stderr** as `cchead: ...`; normal output goes to stdout.

## Project layout

```
33-head/
  go.mod          module definition
  main.go         arg parsing, file iteration, headers, exit codes
  head.go         core streaming copy functions
  head_test.go    table-driven unit + end-to-end tests
  testdata/       fixtures (multiline, short, empty, nonewline, utf8, crlf, binary)
```

The core is split from IO: `run(args, stdin, out, err) int` is the testable
entry point, and the `copyFirst*`/`copyAllButLast*` functions operate on plain
`io.Reader`/`io.Writer`.

## Testing

```sh
go test ./...            # unit + end-to-end tests
go vet ./...             # static checks
gofmt -l .               # formatting (no output = clean)
```

> The design mentions `go test -race`. The race detector requires cgo and a
> working 64-bit C toolchain; on a stock Windows install without one, run the
> tests without `-race` (as above). On Linux/macOS, `go test -race ./...` works
> out of the box.
