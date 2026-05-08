# Coding Challenge #118 — `comm`

A production-ready clone of the Unix `comm` utility, written in Go. It compares
two **already-sorted** files line by line and prints three tab-separated
columns: lines only in FILE1, lines only in FILE2, and lines common to both.

## Build

```sh
go build -o comm .
```

This produces a single static binary (`comm`, or `comm.exe` on Windows). You can
also run without building:

```sh
go run . FILE1 FILE2
```

## Usage

```
Usage: comm [OPTION]... FILE1 FILE2

Compare sorted files FILE1 and FILE2 line by line.
With no options, produce three tab-separated columns: lines only in FILE1,
lines only in FILE2, and lines common to both.

Options:
  -1                      suppress column 1 (lines unique to FILE1)
  -2                      suppress column 2 (lines unique to FILE2)
  -3                      suppress column 3 (lines common to both)
                          (flags may be combined, e.g. -12, -123)
  -i                      case-insensitive line comparison
      --check-order       check that the input is correctly sorted, even
                          if all input lines are pairable
      --nocheck-order     do not check that the input is correctly sorted
      --output-delimiter=STR  separate columns with STR instead of a tab
      --total             output a summary of line counts
      --help              display this help and exit

FILE may be '-' to read standard input (at most one input).
Exit status: 0 on success, 1 on error.
```

### Examples

```sh
comm a.txt b.txt          # full three-column diff
comm -12 a.txt b.txt      # only common lines (suppress columns 1 and 2)
comm -3 a.txt b.txt       # only the differences
sort x | comm - b.txt     # compare a pipeline against b.txt
comm --total a.txt b.txt  # diff plus a count summary
```

Suppression flags combine and the survivors shift left. With `-1`, column 2
prints with no leading tab and column 3 with one leading tab.

## How it works

The core is a single-pass **merge walk** over the two sorted streams. At each
step it compares the current line from each side and emits it into the correct
column:

- `a < b` → column 1 (advance A)
- `a > b` → column 2 (advance B)
- `a == b` → column 3, advance **both**

Because both inputs are assumed sorted, the comparison is `O(n + m)` with `O(1)`
memory — only the two current lines are ever held, so arbitrarily large files
stream through in constant space.

Lines are compared **byte by byte**, the way `LC_ALL=C sort` orders them — not by
locale collation. With `-i`, ASCII letters are folded to lower case for the
comparison while the output preserves the original case (the line from FILE1 is
the one printed in column 3).

### Layout

```
118-comm/
  main.go                wiring + exit code
  internal/comm/
    config.go            parsed-options struct
    cli.go               hand-rolled argument parser + usage text
    source.go            lineSource: streaming reader + order check
    compare.go           byte / case-insensitive comparison
    walk.go              mergeWalk + columnWriter + counts
    run.go               Run(): orchestration and error → exit-code mapping
    comm_test.go         table-driven golden tests
    testdata/            sorted fixtures
```

The testable entry point is `comm.Run(args []string, stdin io.Reader, out, err io.Writer) int`,
which returns the process exit code. `main` is a thin wrapper around it.

## Order checking

- **Default:** detect out-of-order input, warn once on stderr, and keep going
  (matching GNU `comm`). Exit 0.
- **`--check-order`:** abort on the first out-of-order pair with a diagnostic.
  Exit 1.
- **`--nocheck-order`:** never check; silent. Exit 0.

## Tests

```sh
go test ./...
```

Golden tests assert byte-for-byte output over sorted fixtures in `testdata/`:
three-column baseline; each suppression flag and combinations (`-12`, `-13`,
`-23`, `-123`, and `-1 -2` as separate args); `-i`; equal files; disjoint files;
duplicate lines (positional pairing); empty files; `--total`; `--output-delimiter`;
stdin via `-`; a final line without a trailing newline; and order checking
(unsorted input under default / `--check-order` / `--nocheck-order`).

## Out of scope

`-z` / `--zero-terminated` (NUL records), headers, multi-file input, and
compressed inputs are intentionally not implemented ( CRLF
inputs are split on `\n`; a trailing `\r` is treated as part of the line bytes
and compared as-is, matching `comm`.
