# uniq — Coding Challenge #10

A drop-in clone of the Unix `uniq` filter, written in Go. It reads text input
(from a file or standard input), collapses **adjacent** duplicate lines, and
writes the result to a file or standard output.

The key semantic — same as the real tool — is that `uniq` only collapses
**consecutive** equal lines. It is not a global de-duplicator; `sort | uniq` is
the idiom for that.

## Features

- Default mode: collapse each maximal run of adjacent equal lines to one line.
- `-c`, `--count` — prefix each line with its run-length count, GNU-formatted as
  a count right-justified in a 7-wide field, a space, then the line (`%7d %s`).
- `-d`, `--repeated` — print only lines that occur more than once in a run.
- `-u`, `--unique` — print only lines that occur exactly once in a run.
- Flags combine and bundle: `-c -d` and `-cd` are equivalent.
- Long forms: `--count`, `--repeated`, `--unique`, `--help`.
- `--` terminates option parsing.
- Positional `INPUT` then `OUTPUT`; `-` or omission means stdin / stdout.
- O(longest-line) streaming via `bufio.Reader.ReadString` — handles arbitrarily
  large files and arbitrarily long lines (no 64 KB Scanner cap).
- Refuses to read and write the same file (streaming would corrupt it).

## Build

Requires Go 1.24+.

```sh
go build -o uniq ./...      # produces ./uniq (uniq.exe on Windows)
```

Or run without building:

```sh
go run . [OPTION]... [INPUT [OUTPUT]]
```

## Usage

```
Usage: uniq [OPTION]... [INPUT [OUTPUT]]

Filter adjacent matching lines from INPUT (or standard input),
writing to OUTPUT (or standard output).

With no options, matching adjacent lines are collapsed to the first occurrence.

Options:
  -c, --count       prefix lines with the number of occurrences
  -d, --repeated    only print duplicate lines, one for each group
  -u, --unique      only print unique lines
  -h, --help        display this help and exit

Arguments:
  INPUT    file to read; "-" or omitted means standard input
  OUTPUT   file to write; "-" or omitted means standard output
```

## Examples

Given `test.txt`:

```
line1
line1
line2
line3
line3
line3
line4
line2
```

Default — collapse adjacent runs (note the two `line2` runs are not merged
because they are not adjacent):

```sh
$ ./uniq test.txt
line1
line2
line3
line4
line2
```

Count occurrences per run (`-c`):

```sh
$ ./uniq -c test.txt
      2 line1
      1 line2
      3 line3
      1 line4
      1 line2
```

Only repeated runs (`-d`):

```sh
$ ./uniq -d test.txt
line1
line3
```

Only unique runs (`-u`):

```sh
$ ./uniq -u test.txt
line2
line4
line2
```

Combine / bundle flags — counts on repeated runs only (`-cd` == `-c -d`):

```sh
$ ./uniq -cd test.txt
      2 line1
      3 line3
```

Read from stdin (arg omitted or `-`) and write to a file:

```sh
$ cat test.txt | ./uniq -c - out.txt
```

`-d` and `-u` together produce no output (a run is either length 1 or >= 2):

```sh
$ ./uniq -du test.txt      # (empty)
```

## Test

```sh
go test ./...           # run the suite
go test -v ./...        # verbose
go vet ./...            # static checks
gofmt -l .              # formatting check (no output == clean)
```

The suite (`uniq_test.go`) covers:
table-driven tests over the pure core `run` (empty input, adjacent vs.
non-adjacent duplicates, each flag and combination, no-trailing-newline, blank
runs, whitespace significance, CRLF, and a >64 KB long-line regression guard),
`parseArgs` parsing (bundling, long forms, `--`, positionals, errors), and
integration tests over `realMain` (file/stdin input, file/stdout output, missing
file, unknown flag, `--help`, and the same-path refusal).

## Notes / caveats

- Lines are compared as raw byte sequences (locale-independent). CRLF input
  keeps the `\r` as part of the line content; it is not stripped.
- Leading/trailing whitespace is significant: ` a` and `a` are different lines.
- A final line without a trailing newline is still emitted, terminated with a
  single `\n`.
- The `-c` field width matches GNU `uniq`. Differential testing against the
  system `uniq` (Linux/macOS) is the strongest drop-in compatibility check.
