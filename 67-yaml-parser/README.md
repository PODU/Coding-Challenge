# yamlcc — a hand-written YAML parser (Coding Challenge #67)

A from-scratch YAML parser written in Go. It reads a YAML document (or a
multi-document stream) from a file or stdin, parses it into a typed in-memory
value tree, **validates** it (process exit `0`/`1`), and can emit the result as
**JSON**. No third-party YAML library is used for the core scanning/parsing —
only the Go standard library.

## Build & test

```sh
go build ./cmd/yaml      # produces the `yaml` CLI
go test ./...            # unit + golden + error + CLI tests
gofmt -l . && go vet ./...
```

Requires Go 1.24+. Zero runtime dependencies.

## Usage

```
yaml [flags] [file]

  file            Path to a YAML file. If omitted or "-", read stdin.

Flags:
  -json           Parse and print the data structure as JSON to stdout.
  -validate       Validate only: print "OK" or an error (this is the default
                  mode when neither -json nor -ndjson is given).
  -ndjson         For multi-document input, print one compact JSON value per line.
  -indent N       JSON indent width (default 2; 0 = compact single line).
  -strict         Treat tolerated constructs (directives) as errors.

Exit codes:
  0   Valid YAML (and successful emit if -json).
  1   Invalid YAML / parse error (message on stderr).
  2   Usage error (bad flags, unreadable file).
```

### Examples

```sh
yaml input.yaml                 # validate -> prints "OK", exit 0
yaml -json input.yaml           # YAML -> pretty JSON on stdout
cat input.yaml | yaml -json -   # read from stdin
yaml -json -indent 0 input.yaml # compact JSON
yaml -ndjson stream.yaml        # one JSON value per document, per line
```

### Library API

```go
import yaml "yamlcc"

docs, err := yaml.Parse(src)        // one yaml.Value per document
doc,  err := yaml.ParseOne(src)     // errors if the stream has >1 document
docs, err := yaml.ParseStrict(src, true)
```

## Supported subset

### Structure
- **Block mappings** — `key: value`, nested by indentation.
- **Block sequences** — `- item`, including sequences of mappings, mappings of
  sequences, and compact nested sequences (`- - a`).
- **Indentation** uses **spaces only**. A TAB used as structural indentation is
  a hard error.
- A block sequence may be written at the **same indentation** as its parent key:
  ```yaml
  fruits:
  - apple
  - banana
  ```
- **Flow sequences** `[a, b, c]` and **flow mappings** `{k: v}`, which may nest,
  may span multiple lines, and may carry a trailing comma (`[1, 2,]`).
- **Empty collections** `[]` / `{}`, and an empty value (`key:`) which becomes
  `null`.
- **Multi-document streams**: `---` starts a document (optionally with an inline
  root, `--- 42`), `...` ends one. Output is a bare value for a single document
  or a JSON array for several (one-per-line with `-ndjson`).

### Scalars & implicit typing (plain, unquoted scalars only)
Resolution order: **null → bool → int → float → string**.

| Type   | Examples                                                        |
|--------|-----------------------------------------------------------------|
| null   | `null`, `Null`, `NULL`, `~`, empty                              |
| bool   | `true`/`false`, `yes`/`no`, `on`/`off` (YAML 1.1, case-insensitive) |
| int    | `42`, `-7`, `+3`, hex `0x12d4`, octal `023332` / `0o23332`, bin `0b1010` |
| float  | `3.14`, `-0.5`, `6.8e-3`, and specials `.inf`, `-.inf`, `.nan`  |
| string | anything else                                                   |

Notable, documented resolution rules:
- `off`, `no`, `yes`, `on` resolve to **booleans** (this surprises some users;
  the challenge explicitly names `off`).
- A leading-zero number is **octal**, so `023332` is `9946` but `08` is *not* a
  valid octal and therefore resolves to the **string** `"08"`.
- The bare words `inf` / `nan` (without a leading `.`) are **strings**; only the
  dotted forms are special floats.

### Strings & escapes
- **Single-quoted**: literal; the only escape is `''` → `'`.
- **Double-quoted**: C-style escapes are decoded —
  `\n \t \r \" \\ \/ \0 \a \b \f \v \e`, plus `\xHH`, `\uHHHH`, `\UHHHHHHHH`.
  An invalid escape or invalid hex is an error.
- **Quoted scalars are always strings** — no type inference. So `"true"` is the
  string `true` and `"123"` is the string `123`.
- **Block scalars**:
  - Literal `|` — newlines preserved.
  - Folded `>` — single line breaks fold to spaces; blank lines and
    more-indented lines are preserved.
  - Chomping indicators: clip (default), strip `-`, keep `+`. An explicit indent
    indicator digit is supported (e.g. `|2`).

### Comments
- `#` begins a comment to end of line. A `#` that starts a comment must be at the
  start of a line or preceded by whitespace, so `http://x#y` is **not** a
  comment. A `#` inside quotes or a block scalar is literal.

## JSON output mapping
- `null` → JSON `null`; booleans/ints/floats/strings map directly.
- YAML map keys are emitted as JSON string keys, **sorted** for deterministic
  output.
- The special floats have no JSON representation and are emitted as strings:
  `.inf` → `"Infinity"`, `-.inf` → `"-Infinity"`, `.nan` → `"NaN"`.

## Tolerated (accepted with limited semantics)
- A leading `%YAML` / `%TAG` directive line is skipped (and rejected under
  `-strict`).

## Not supported — reported as a clear, located error (never a silent misparse)
- TABs as structural indentation.
- Anchors / aliases (`&a` / `*a`) — reported as `unsupported`.
- Explicit / custom tags (`!!str`, `!foo`) — reported as `unsupported`.
- Explicit complex keys (`? key`) and merge keys (`<<`).
- Multi-line plain (unquoted) scalars.

## Project layout
```
cmd/yaml/          CLI: flags, file/stdin, exit codes, JSON output
internal/scanner/  lexical layer: line splitting, quoted-scalar decoding
internal/parser/   syntactic layer: block + flow grammar, block scalars, streams
internal/value/    Value model (Kind/Value/KV) and the plain-scalar Resolver
internal/emit/     Value tree -> deterministic JSON
testdata/valid/    golden fixtures: *.yaml paired with *.json
testdata/invalid/  error fixtures: *.yaml expected to fail validation
yaml.go            public API (Parse / ParseOne / ParseStrict)
```

## Tests
- **Golden tests** parse every `testdata/valid/*.yaml` and diff the JSON against
  the paired `*.json` (regenerate with `go test . -run TestGolden -update`).
- **Error tests** assert each `testdata/invalid/*.yaml` fails with a located
  message containing the expected substring.
- **Unit tests** cover the scalar resolver truth-table, the quoted-scalar escape
  decoder, indentation/line splitting, flow nesting and block scalars.
- **CLI tests** exercise the exit-code matrix, stdin, `-json`/`-ndjson`/`-indent`
  and `-strict`.
