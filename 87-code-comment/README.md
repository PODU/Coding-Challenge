# ccomment — Code Comment Remover (Coding Challenge #87)

`ccomment` strips comments from source code **while leaving the program's
behavior unchanged**. It is a deterministic, offline tool (no LLM, no network):
comment removal is purely lexical.

A naive regex/`sed` approach corrupts code, because comment delimiters (`#`,
`//`, `/* */`) routinely appear **inside string and character literals** where
they are *not* comments. `ccomment` instead runs a small **comment-aware
lexical scanner per language** that tracks whether it is in code, a string, a
char/rune literal, or a comment — so only true comments are removed and
everything else is preserved byte-for-byte.

## Features

- **Python** — `#` line comments. Understands `'…'`, `"…"`, and triple-quoted
  `'''…'''` / `"""…"""` strings (docstrings are strings, never comments).
- **JavaScript / TypeScript** — `//` and `/* */`, plus template literals
  (`` `…${expr}…` ``) and a heuristic regex-vs-division disambiguator.
- **C / C++** — `//` and `/* */`, with backslash line-continuation inside `//`
  comments and char literals.
- **Go** — `//` and `/* */`, raw strings (`` `…` ``), and rune literals, **plus**:
  - **Doc-comment preservation** (Step 4): a comment immediately preceding a
    top-level `package`/`func`/`type`/`const`/`var` (no blank line between) is
    kept. A blank line breaks the association and the comment is removed.
  - Optional `go/ast` oracle used in tests to validate the heuristic.
- **Directive / tool-config preservation** (Step 5): `# noqa`, `# type:`,
  Python shebang, `//go:generate`, `//go:build`, `// +build`, `// eslint-*`,
  `// NOLINT`, etc. are kept because tooling depends on them.
- **Literal-safe**: `"http://x"`, `'#'`, `"/* not a comment */"`,
  `` `a // b` `` (Go raw), `/\/\//` (JS regex) are all preserved intact.
- **Robust**: unterminated block comment → error (exit 2), never silently eats
  the file. Idempotent on comment-free input. CRLF / final-newline / BOM
  preserved.

## Install / build

Requires Go 1.24+.

```sh
cd 87-code-comment
go build ./...
go install ./cmd/ccomment      # or: go run ./cmd/ccomment [flags] [file]
```

## Usage

```
ccomment [flags] [file]

  (no file, or "-")   read source from stdin (then --lang is required)
  file                read from path; language inferred from extension
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--lang` | (infer) | `python\|js\|ts\|c\|cpp\|go` — overrides extension inference |
| `-w` | false | rewrite the file in place (atomic temp+rename) |
| `--keep-doc` | true | keep Go doc comments |
| `--keep-pragma` | true | keep tool-config/directive comments |
| `--blank-lines` | `drop` | full-line comment mode: `drop` (delete line) or `blank` (keep empty line, preserves line numbers) |
| `--minify` | false | also collapse runs of blank lines |
| `--stats` | false | print removed/kept/byte counts to stderr |
| `--check` | false | exit 1 if any comment *would* be removed (CI guard); writes nothing |
| `-o` | — | write output to a path instead of stdout |

Exit codes: `0` success · `1` would-remove (only with `--check`) · `2`
usage / IO / unterminated-block error.

### Examples

```sh
# stdin filter
echo 'x = 1  # gone' | ccomment --lang python          # -> x = 1

# strip a file to stdout (language inferred from .go)
ccomment main.go

# rewrite in place
ccomment -w app.js

# CI guard: fail if any comment would be stripped
ccomment --check --lang go ./pkg/foo.go
```

## Library API

```go
import "ccomment"

st, _ := ccomment.For("go")          // or ccomment.ForFile("x.go")
out, stats, err := st.Strip(src, ccomment.Options{
    KeepDoc: true, KeepPragma: true, BlankLines: ccomment.Drop,
})
```

`Scan(src)` returns the raw comment `Span`s (pure, no policy); `Strip` applies
the keep rules and whitespace/line policy. Splitting the two keeps each backend
tiny and makes preserve-rules and minification orthogonal.

## Testing

```sh
go test ./...                         # table + golden tests, fully offline
go test ./ -run TestGolden -update    # regenerate testdata/<lang>/*.out goldens
go test ./ -fuzz FuzzIdentity         # property: keep-all == identity, never panics
go vet ./... && gofmt -l .
```

Test coverage includes per-language golden source→output pairs, the critical
string-vs-comment disambiguation suite, Go doc-comment preservation cross-checked
against `go/ast`, directive preservation, unterminated-block errors, CRLF /
empty / no-final-newline robustness, and a fuzz harness asserting the scanner
never panics and reproduces code spans verbatim.

## Known limitations

- JavaScript regex-vs-division detection is heuristic (a full JS parse is out of
  scope); the common cases are covered by tests.
- Doc-comment detection uses a gofmt-style heuristic by default; the `go/ast`
  oracle is used to validate it in tests.
</content>
</invoke>
