# cut — Coding Challenge #4

A from-scratch reimplementation of the Unix `cut` utility in Go. It extracts
selected portions of each input line — by **byte** (`-b`), **character** (`-c`),
or **delimited field** (`-f`) — and writes them to standard output. It is a
streaming, constant-memory filter designed to compose cleanly in shell
pipelines.

## Features

- Modes `-f` / `-c` / `-b` (mutually exclusive; exactly one required).
- 1-indexed LIST grammar with ranges: `N`, `N-M`, `N-`, `-M`, comma- **or**
  whitespace-separated (`-f1,2`, `-f"1 2"`). Overlapping/out-of-order ranges are
  merged and emitted once, in increasing position order (matching GNU `cut`).
- `-d` input delimiter (default TAB), single character.
- `-s` / `--only-delimited`: in field mode, drop lines with no delimiter.
- `--complement`: invert the selection.
- `--output-delimiter=STRING`: rejoin output with a custom string.
- `-n`: BSD compatibility no-op (with `-b`).
- Multiple files plus stdin via `-`; with no file, reads stdin.
- `--help` / `--version`.
- Streaming line-by-line I/O (constant memory), UTF-8 aware (`-c`), raw bytes
  (`-b`), trailing-newline preservation, graceful handling of invalid UTF-8.

## Layout

```
cmd/cut/main.go          entrypoint + run(): wiring, file iteration, exit codes
internal/cli/cli.go      argument parsing -> validated cut.Config
internal/cut/list.go     LIST parsing, Range, Selector (Selected/complement)
internal/cut/cutter.go   Config, Cutter, streaming ProcessStream
internal/cut/fields.go   field-mode line transform
internal/cut/bytescars.go byte- and char-mode line transforms
internal/cut/*_test.go    table-driven unit tests
cmd/cut/main_test.go     integration tests (run examples, stdin, multi-file)
testdata/                sample.tsv, fourchords.csv
```

## Build, run, test

Requires Go 1.24+.

```sh
# Build the binary
go build -o cut ./cmd/cut        # produces ./cut (cut.exe on Windows)

# Or run directly
go run ./cmd/cut -f2 testdata/sample.tsv

# Test, vet, format check, coverage
go test ./...
go vet ./...
gofmt -l .
go test -cover ./internal/...
```

## Examples

```sh
# Field 2 of a TSV (default delimiter is TAB)
cut -f2 testdata/sample.tsv

# Field 2 of a CSV
cut -f2 -d, testdata/fourchords.csv

# Multiple fields
cut -f1,2 testdata/sample.tsv
cut -d, -f"1 2" testdata/fourchords.csv

# stdin pipeline
tail -n5 testdata/fourchords.csv | cut -d, -f"1 2"
cut -f2 -d, testdata/fourchords.csv | uniq | wc -l

# Characters and bytes (1-indexed ranges)
cut -c1-5 testdata/sample.tsv
echo "abcdefg" | cut -b1,3-4

# Complement and custom output delimiter
cut -d, -f1 --complement testdata/fourchords.csv
echo "a-b-c" | cut -d- -f2 --output-delimiter=:
echo "abcdefg" | cut -b1-2,4-5 --output-delimiter=:   # -> ab:de
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (also `--help` / `--version`). |
| 1 | Runtime error (e.g. a file could not be opened); remaining files are still processed. |
| 2 | Usage / argument error. |

## Notes & limitations

- **Delimiter** is a single byte. Multi-byte (UTF-8) delimiters are out of
  scope and rejected with `the delimiter must be a single character`. Empty
  `-d ''` is an error.
- **CRLF**: `\r` is treated as an ordinary byte/character, matching Unix `cut`.
  Pipe through `dos2unix` first if you need CRLF stripping.
- **Invalid UTF-8** under `-c` decodes to the Unicode replacement rune; it never
  crashes.
- **`--output-delimiter` with `-b`/`-c`** only changes output when the selection
  leaves gaps between ranges (typically via `--complement`); otherwise selected
  positions are concatenated, as in GNU `cut`.
```
