# mygrep — Coding Challenge #9

A production-ready clone of the Unix `grep` utility, written in Go.
It searches files, directories, or standard input line by line and prints the
lines that match a regular-expression pattern.

## Features

- **Regex matching** via Go's standard-library `regexp` (RE2): `\d`, `\w`,
  `\s` and their negations, anchors `^`/`$`, alternation, quantifiers, groups,
  and character classes — all linear-time, no catastrophic backtracking.
- **Flags:** `-r` recursive, `-i` ignore-case, `-v` invert, `-c` count,
  `-n` line numbers, `-w` whole-word. Short clusters (`-ivn`) and GNU long
  forms (`--ignore-case`, …) are both accepted.
- **Inputs:** one or more files, recursive directory walks, and standard input
  (no path, or `-`).
- **POSIX exit codes:** `0` = matched, `1` = no match, `2` = error.
- **Streaming:** `bufio.Scanner` with a 1 MiB line buffer handles huge files
  and very long lines with a tiny memory footprint. CRLF endings are tolerated.
- **Pluggable matcher:** the regex engine sits behind a `Matcher` interface, so
  the challenge's "hard mode" hand-written engine can be dropped in later.

## Layout

```
09-grep/
├── cmd/mygrep/main.go        # thin CLI entry point
├── internal/cli/             # argv -> Config parser
├── internal/matcher/         # Matcher interface + regexp implementation
├── internal/grep/            # input resolver + search engine + formatter
├── testdata/                 # sample data (test.txt, rockbands.txt, …)
├── go.mod
└── README.md
```

## Build

```bash
# from 09-grep/
go build -o mygrep ./cmd/mygrep      # single static binary
# or build everything:
go build ./...
```

## Run

```
mygrep [OPTIONS] PATTERN [FILE...]

  -r, --recursive      Recurse into directories.
  -i, --ignore-case    Case-insensitive matching.
  -v, --invert-match   Select non-matching lines.
  -c, --count          Print only a count of matching lines per file.
  -n, --line-number    Prefix each line of output with its line number.
  -w, --word-regexp    Match only whole words.
  -h, --help           Show usage and exit.
```

### Examples

```bash
mygrep "" testdata/test.txt              # every line (empty pattern)
mygrep Mozilla testdata/test.txt         # literal match
mygrep -i nirvana testdata/rockbands.txt # case-insensitive
mygrep -v error log.txt                  # lines WITHOUT "error"
mygrep -c '\d' testdata/symbols.txt      # how many lines contain a digit
mygrep -n '^The' testdata/test.txt       # numbered lines starting "The"
mygrep -w cat testdata/words.txt         # "cat" but not "category"
mygrep -r 1985 testdata                  # recursive, filename-prefixed
cat testdata/test.txt | mygrep -i band   # read from a pipe
mygrep -- -v file.txt                    # "--" ends options; pattern is "-v"
```

### Output format

- Single source, no flags: `<line>`
- Multiple files or `-r`: `path:<line>`
- With `-n`: `[path:]<n>:<line>`
- With `-c`: `[path:]<count>` (one line per source)

Filename prefixing turns on automatically when there is more than one input
source or when `-r` is used, matching real `grep`.

### Exit status

| Code | Meaning |
|------|---------|
| 0 | One or more lines matched. |
| 1 | No lines matched (clean run). |
| 2 | Error: invalid regex, unreadable file, or a directory without `-r`. |

An error such as a missing file marks the whole run as exit `2` even if other
inputs matched, while still searching the remaining inputs.

## Test

```bash
go test ./...            # unit + golden/integration tests
go test -cover ./...     # with coverage
go vet ./...
```

The suite covers the matcher (literals, empty pattern, `\d`/`\w`, anchors,
case-insensitivity, whole-word, invert), the CLI parser (short/long/clustered
flags, `--` separator, errors), and end-to-end behaviour of the search engine
and formatter (counts, line numbers, multi-file prefixing, recursion, CRLF,
no-trailing-newline, very long lines, missing files, directory-without-`-r`,
bad regex, and stdin).

## Notes & documented behaviour

- **Unicode:** RE2 is UTF-8 aware; `\w`/`\d` follow RE2 semantics.
- **CRLF:** a trailing `\r` is stripped per line so `$` and exact matches work
  on Windows-authored files.
- **Binary files:** currently treated as text (matching lines are printed); a
  `-I`/binary-skip flag is a listed future extension.
- **Symlink loops:** `filepath.WalkDir` does not follow symlinked directories,
  so recursive walks cannot loop.
- **Hard mode:** a hand-written NFA engine implementing `matcher.Matcher` can
  replace the default `regexMatcher` without touching the rest of the code.
```
