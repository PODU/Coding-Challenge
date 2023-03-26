# jsonparser

A command-line JSON validator built for John Crickett's
[Coding Challenge #2 — Build Your Own JSON Parser](https://codingchallenges.substack.com/p/coding-challenge-2).

It implements a **hand-written lexer** and a **recursive-descent parser** for
the JSON grammar defined by [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) —
no `encoding/json`, no regex, no parser generators. The stdlib is used only as a
test oracle.

The tool reads a JSON document (from a file or stdin), validates it, and reports
the verdict Unix-style:

- **Valid JSON** → prints `Valid JSON`, exits `0`.
- **Invalid JSON** → prints `Invalid JSON: <message> at line L, column C` to
  stderr, exits `1`.

## Requirements

- Go **1.22+** (developed and tested with Go 1.24).

## Build

```sh
go build -o jsonparser ./cmd/jsonparser     # produces a single static binary
# Windows: go build -o jsonparser.exe ./cmd/jsonparser
```

Or run without installing:

```sh
go run ./cmd/jsonparser file.json
```

## Usage

```
jsonparser [options] [file]

Arguments:
  file            Path to a JSON file. If omitted or "-", read from stdin.

Options:
  -q, --quiet     Suppress success/error text; communicate only via exit code.
  -v, --verbose   Print a one-line summary of the parsed value on success.
      --version   Print version and exit 0.
  -h, --help      Print usage and exit 0.

Exit codes:
  0   Valid JSON.
  1   Invalid JSON (syntax error) or I/O error (file not found, read error).
  2   Usage error (bad flags).
```

### Examples

```sh
# Validate a file
$ jsonparser testdata/step4/valid.json
Valid JSON

# Validate from stdin
$ echo '{"a":[1,2,{"b":true}]}' | jsonparser
Valid JSON

# A syntax error reports line and column
$ echo '{"a":}' | jsonparser
Invalid JSON: expected a JSON value, found '}' at line 1, column 6
$ echo $?
1

# Script-friendly: just the exit code, no output
$ jsonparser -q file.json; echo $?
0

# Verbose summary on success
$ echo '[1,2,3]' | jsonparser -v
Valid JSON
array with 3 element(s)
```

## Test

```sh
go test ./...                 # unit + fixture + conformance + CLI tests
go vet ./...                  # static checks
gofmt -l .                    # formatting (no output = clean)

# Native fuzzing: feeds random bytes, asserts no panic and that we never accept
# JSON the stdlib rejects.
go test -run=^$ -fuzz=FuzzParseAgainstStdlib -fuzztime=20s ./internal/parser
```

The suite covers:

- **Lexer unit tests** — every token type, all string escapes, surrogate pairs,
  the full number grammar and its boundaries, and `ILLEGAL` cases.
- **Parser unit tests** — table-driven valid/invalid documents, targeted
  error-message and error-position assertions, the depth guard, and AST shape.
- **Staged fixtures** — `testdata/step1` … `step4`, mirroring the challenge's
  staged delivery (one subtest per file).
- **json.org conformance** — the canonical `JSON_checker` suite
  (`pass1`–`pass3`, `fail1`–`fail33`) under `testdata/json_checker`.
- **CLI integration** — exercises `run()` for exit codes, `-q`/`-v`, file vs
  stdin, missing files, and bad flags.
- **Oracle cross-check + fuzzing** — compares verdicts against
  `encoding/json.Valid` from the Go standard library.

## Architecture

Classic two-phase pipeline: **Lexer (scanner) → Parser (recursive descent)**.

```
02-json-parser/
  cmd/jsonparser/main.go        CLI: arg/stdin handling, exit codes, output
  internal/lexer/
    token.go                    TokenType, Token{Type, Literal, Line, Col}
    lexer.go                    NextToken(); whitespace, string, number scanning
    lexer_test.go
  internal/parser/
    ast.go                      Value nodes (Object, Array, String, Number, Bool, Null)
    parser.go                   Parser: Parse() -> (Value, error); recursive descent
    error.go                    ParseError{Message, Line, Col}
    parser_test.go              unit tests
    fixtures_test.go            staged + json.org conformance fixtures
    fuzz_test.go                native fuzz + stdlib oracle
  testdata/
    step1/ step2/ step3/ step4/ staged challenge fixtures
    json_checker/               pass1-3, fail1-33
  README.md
```

### Grammar (RFC 8259)

```
json     := ws value ws EOF
value    := object | array | string | number | "true" | "false" | "null"
object   := '{' ws '}' | '{' members '}'
members  := pair (',' pair)*
pair     := ws string ws ':' value
array    := '[' ws ']' | '[' elements ']'
elements := value (',' value)*
```

### What it validates

- Objects, arrays, and all scalar types, nested arbitrarily.
- **Strings:** escapes `\" \\ \/ \b \f \n \r \t` and `\uXXXX`, including
  surrogate pairs. Rejects unescaped control characters, bad escapes, short
  `\u` sequences, lone surrogates, and unterminated strings.
- **Numbers:** the exact RFC grammar
  `-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`. Rejects leading zeros, a
  bare `.`, missing fraction/exponent digits, and a leading `+`.
- Insignificant whitespace (space, tab, LF, CR) is skipped.
- Trailing non-whitespace after the top-level value is rejected.

## Design notes & RFC-8259 divergences from json.org's old test suite

This implementation follows RFC 8259, which differs in two documented places
from the classic `JSON_checker` expectations. Both are whitelisted as **valid**
in the conformance test:

- **`fail1.json`** — a bare top-level string. RFC 8259 permits *any* value as the
  top-level JSON text, so `"...”`, `42`, `true`, and `null` are all valid.
- **`fail18.json`** — deeply nested arrays. RFC 8259 sets no depth limit, so this
  is valid JSON. (A configurable `WithMaxDepth` guard, default 10000, protects
  against stack overflow on pathological input but does not reject these
  fixtures.)

Other intentional choices:

- **Duplicate object keys** are accepted (RFC 8259 permits them); all members are
  preserved in source order.
- **Invalid UTF-8 bytes** and **lone/unpaired surrogate escapes** are rejected —
  stricter than Go's lenient `encoding/json`, matching RFC 8259's requirement
  that JSON text be valid UTF-8.

