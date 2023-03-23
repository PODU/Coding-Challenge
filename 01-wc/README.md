# ccwc — a `wc` clone in Go

Coding Challenge #1: a faithful subset of the Unix `wc` (word count) utility.
Counts **lines**, **words**, **characters**, and **bytes** for one or more
files, or from standard input. Output is byte-for-byte compatible with **GNU
`wc` (coreutils 8.32)** — verified by differential tests that diff `ccwc`'s
output against the system `wc` on every flag combination.

## Features

- `-c` byte count, `-l` line count, `-w` word count, `-m` character (UTF-8) count.
- Default (no flags) prints lines, words, bytes.
- Reads standard input when no file is given, or when the file is `-`.
- Multiple files print one row per file plus a `total` row.
- Combined/bundled flags (`-lw`, `-cl`, …); column order is always canonical:
  **lines, words, chars, bytes**.
- GNU-compatible right-aligned column widths and `total` alignment.
- Single streaming pass with a fixed 64 KiB buffer: constant memory on
  multi-gigabyte inputs; all counts are `int64`.
- UTF-8 aware `-m`; invalid/truncated byte sequences count one char each and
  never panic.

## Requirements

- Go 1.21 or newer.

## Layout

```
01-wc/
├── go.mod                       # module ccwc
├── cmd/ccwc/
│   ├── main.go                  # entrypoint: arg wiring, files/stdin, exit codes
│   └── main_test.go             # integration + differential tests vs system wc
├── internal/
│   ├── cli/
│   │   ├── args.go              # flag parsing -> Options (bundled flags, --, -)
│   │   ├── format.go            # GNU column-width logic + row rendering
│   │   └── cli_test.go
│   └── counter/
│       ├── counter.go           # single-pass Count(reader) -> Counts
│       └── counter_test.go      # unit tests for the counting core
└── scripts/acceptance.sh        # quick manual gate vs system wc
```

## Build

```sh
cd 01-wc
go build ./...                 # compile everything
go build -o ccwc ./cmd/ccwc    # produce the ./ccwc binary (ccwc.exe on Windows)
```

## Run / usage

```
Usage: ccwc [OPTION]... [FILE]...

Options:
  -c    print the byte count
  -l    print the line (newline) count
  -w    print the word count
  -m    print the character count (UTF-8 aware)
  -h    display help and exit

With no count option, -l -w -c (lines, words, bytes) are printed.
```

Examples:

```sh
$ ccwc -c test.txt
  342190 test.txt

$ ccwc -l test.txt
    7145 test.txt

$ ccwc test.txt
    7145   58164  342190 test.txt

$ cat test.txt | ccwc -l
7145

$ ccwc a.txt b.txt
 2  5 24 a.txt
 1  1  2 b.txt
 3  6 26 total
```

> Note on padding: `ccwc` matches **GNU** `wc`'s width rules. A single count of a
> single file is printed with no padding (`342190 test.txt`); multiple counts or
> multiple files are right-aligned to the width of the largest value; stdin with
> multiple counts uses GNU's fixed width of 7. The challenge's samples (e.g.
> `  342190 test.txt`) reflect **BSD/macOS** `wc`, which always pads to width 7 —
> the only difference is column padding, not the counts.

## Test

```sh
go test ./...            # unit + integration + differential tests
go test ./... -v         # verbose
go vet ./...             # static checks
gofmt -l .               # formatting (empty output = clean)
```

The differential tests (`TestDifferential*` in `cmd/ccwc/main_test.go`) compare
`ccwc` against the system `wc` across many fixtures and flag sets. They are
**skipped automatically** if no `wc` is found on `PATH`, so the suite stays green
on machines without coreutils.

A quick manual gate is also provided:

```sh
sh scripts/acceptance.sh        # diffs ccwc vs wc on generated fixtures
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | success |
| `1`  | a file could not be opened/read (processing continues for the rest) |
| `2`  | usage error (unknown flag / bad arguments) |

Usage errors exit `2`. (Some GNU `wc` builds exit `1` for a
bad option; `ccwc` follows the spec.)

## Toolchain status

Built and tested with **Go 1.24.5** on Windows. `go build ./...`, `go vet ./...`,
`gofmt -l .`, and `go test ./...` all pass; the differential suite was run
against **GNU coreutils `wc` 8.32** with byte-identical results.
