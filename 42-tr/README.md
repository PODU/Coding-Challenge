# tr — Coding Challenge #42

A streaming, constant-memory reimplementation of the core, portable behaviour
of GNU `tr` in Go. It copies standard input to standard output while
**translating**, **deleting**, and/or **squeezing** characters selected by one
or two SET operands.

There are no file arguments — `tr` is a pure `stdin → stdout` pipe filter.

## Build

```sh
cd 42-tr
go build -o tr .       # produces ./tr (tr.exe on Windows)
```

No third-party dependencies — standard library only.

## Usage

```
Usage: tr [OPTION]... SET1 [SET2]

Options:
  -c, -C, --complement      use the complement of SET1
  -d, --delete              delete characters in SET1, do not translate
  -s, --squeeze-repeats     replace each run of a repeated character that is
                            listed in the last SET with a single occurrence
  -t, --truncate-set1       first truncate SET1 to the length of SET2
  -u                        unbuffered output (flush after every write)
  -h, --help                display help and exit
  -v, --version             output version information and exit
```

Flags may be combined (`-ds`, `-cs`, `-cd`). `--` ends flag parsing so a SET
may begin with `-`.

### Operations

| Invocation            | Behaviour                                              |
|-----------------------|-------------------------------------------------------|
| `tr SET1 SET2`        | Translate each SET1 char to the SET2 char at the same index |
| `tr -d SET1`          | Delete every char in SET1                             |
| `tr -s SET1`          | Squeeze runs of repeated chars in SET1 to one         |
| `tr -s SET1 SET2`     | Translate SET1→SET2, then squeeze runs of SET2        |
| `tr -d -s SET1 SET2`  | Delete SET1 chars, then squeeze runs of SET2          |

The pipeline order matches GNU `tr`: **delete → translate → squeeze**.

## SET syntax

- **Literals** — any byte not part of another construct.
- **Ranges** — `a-z`, `0-9a-f` (inclusive, ascending). A `-` at the start/end
  or with no valid right endpoint is a literal `-`. A reversed range is an error.
- **POSIX classes** — `[:alnum:]`, `[:alpha:]`, `[:digit:]`, `[:lower:]`,
  `[:upper:]`, `[:space:]`, `[:blank:]`, `[:punct:]`, `[:cntrl:]`, `[:print:]`,
  `[:graph:]`, `[:xdigit:]`. In SET2 only `[:upper:]`/`[:lower:]` are allowed
  (for case mapping).
- **Equivalence classes** — `[=c=]` (just `c` in the C/POSIX locale).
- **Repeat (SET2 only)** — `[c*n]` repeats `c` exactly `n` times; `[c*]` (or
  `[c*0]`) pads `c` to make SET2 as long as SET1.
- **Escapes** — `\\ \n \t \r \f \v \a \b` and octal `\NNN` (1–3 digits).

### SET1 / SET2 length reconciliation (translate mode)

- `len(SET2) > len(SET1)` — surplus SET2 chars are ignored.
- `len(SET2) < len(SET1)` — SET2 is padded by repeating its **last char**.
- With `-t` — SET1 is truncated to `len(SET2)` instead (no padding).
- SET2 must be non-empty in translate mode.

## Examples

```sh
echo Hello | tr a-z A-Z              # HELLO
echo hi! | tr '[:lower:]' '[:upper:]' # HI!
echo cab | tr abc x                   # xxx
echo He1lo | tr -d a-z                # H1
printf 'a   b' | tr -s ' '            # a b
echo aabb | tr -s a-z A-Z             # AB
echo 'a, b' | tr -cs '[:alpha:]' '\n' # a / b on separate lines
echo aabbb | tr -ds a b               # b
echo A | tr '\101' x                  # x
```

## Design

```
42-tr/
  main.go             CLI: arg parsing, mode validation, wiring, stream loop
  internal/set/set.go SET lexer/expander: escapes, ranges, classes, [c*n]
  internal/tr/tr.go   table builder + single-pass byte-stream engine
  main_test.go        behavioural / golden tests (full CLI path)
  internal/set/*_test.go, internal/tr/*_test.go  unit tests
```

The engine builds fixed 256-entry lookup tables once (`delSet`, `trMap`,
`sqzSet`), then makes one pass over the input through `bufio.Reader` /
`bufio.Writer`. Memory is O(1) in input size, so arbitrarily large streams pass
through without buffering. Processing is byte-oriented (C locale), so binary and
non-UTF-8 input is never corrupted.

## Test

```sh
go test ./...
go vet ./...
```
