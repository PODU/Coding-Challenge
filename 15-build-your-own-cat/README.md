# ccat — Build Your Own `cat`

A production-ready clone of the Unix `cat` utility, written in Go with the
standard library only. It reads one or more files (or standard input) and
concatenates their contents to standard output, with optional line numbering.

This is [Coding Challenge #15](https://codingchallenges.substack.com/p/coding-challenge-15-build-your-own).

## Build

```sh
go build -o ccat .
```

This produces a single static binary (`ccat`, or `ccat.exe` on Windows) with no
third-party dependencies.

## Usage

```
Usage: ccat [OPTIONS] [FILE...]

Concatenate FILE(s) to standard output.
With no FILE, or when FILE is '-', read standard input.

Options:
  -n          number all output lines
  -b          number non-blank output lines (overrides -n)
  --          treat all following arguments as file names
  -h, --help  show this help and exit
```

### Examples

```sh
ccat test.txt                # print a file
ccat - < test.txt            # read from stdin via dash
head -n1 test.txt | ccat     # read from stdin via a pipe (no args)
ccat a.txt b.txt c.txt       # concatenate files in order
ccat -n test.txt             # number every line
ccat -b test.txt             # number only non-blank lines
ccat -n a.txt - b.txt        # mix files and stdin, continuous numbering
```

## Behavior

- **No arguments** or a `-` operand reads from standard input.
- Line numbering uses a **continuous counter across all inputs** (it does not
  reset per file), formatted as `cat` does: a 6-column right-justified number, a
  tab, then the line.
- `-b` **overrides** `-n`; blank lines are passed through unnumbered.
- Output is **byte-faithful**: content is never re-encoded, and a final line
  without a trailing newline is emitted as-is. CRLF endings are preserved.
- Errors on one operand (missing file, permission denied, a directory) are
  reported to **stderr** and do **not** abort processing of the remaining
  operands; the exit code becomes `1`.

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | All operands read successfully. |
| `1`  | An operand could not be read, or an invalid option was given. |

## Testing

```sh
go test ./...
```

The test suite (`cat_test.go`) is table-driven and covers golden outputs for
every challenge step (single file, concatenation, `-n`, `-b`, `-b` overriding
`-n`), stdin handling, continuous numbering, no-trailing-newline and empty-file
edge cases, argument parsing (combined flags, `--` terminator, unknown flag),
and the report-but-continue error path (missing file and directory arguments).

## Project layout

```
15-build-your-own-cat/
  go.mod
  main.go        # entry point: wires os streams into run()
  cat.go         # run(), parseArgs(), cat(), processReader()
  cat_test.go    # table-driven tests
  README.md
```

The core is the testable `run(args []string, stdin io.Reader, out, err io.Writer) int`
function, which decouples all logic from `os` globals so it can be exercised
with in-memory buffers.
