# ccyq — a `jq` for YAML (Coding Challenge #46)

`ccyq` reads a YAML document (or a multi-document stream) from a file or stdin,
evaluates a small **path/filter expression language** against it, and emits the
result as YAML (default) or JSON. It doubles as a YAML <-> JSON converter and
supports in-place editing.

YAML parsing/serialization is delegated to [`gopkg.in/yaml.v3`][yaml] (which
round-trips comments, key order, anchors, and scalar styles). The **query
engine** — lexer, parser, AST, and tree-walking evaluator — is written from
scratch and is the heart of the project.

[yaml]: https://pkg.go.dev/gopkg.in/yaml.v3

## Build & test

```sh
cd 46-yq
go mod tidy
go build -o ccyq.exe ./cmd/ccyq
go test ./...
```

## Usage

```
ccyq [options] '<filter>' [file]

  <filter>   path/filter expression (default '.', the identity filter)
  file       input file; if omitted, read from stdin

Options:
  -o, --output <yaml|json>   output format            (default yaml)
  -p, --input  <yaml|json>   input format             (default yaml)
  -r, --raw                  raw output for string scalars (no quotes)
  -c, --compact              compact JSON (one doc per line)
  -i, --inplace              edit the input file in place (requires file)
      --no-doc-sep           omit '---' between multiple output YAML docs
  -h, --help                 show this help
  -v, --version              show version
```

### Examples

```sh
ccyq '.' quotes.yaml                  # pretty-print, formatting preserved
ccyq '.quotes[1].quote' quotes.yaml   # drill into a field
ccyq '.quotes[] | .author' quotes.yaml
ccyq '[.quotes[].quote]' quotes.yaml  # collect into a sequence
ccyq -o json '.quotes[0]' quotes.yaml # YAML -> JSON
ccyq -p json -o yaml '.' data.json    # JSON -> YAML
cat quotes.yaml | ccyq '.quotes | .[0]'
ccyq -i '.version' config.yaml        # in-place edit
```

## Supported filter subset

| Filter | Meaning |
|--------|---------|
| `.` | Identity — returns the input unchanged, preserving formatting. |
| `.foo` | Object access — the value of key `foo`. |
| `.["foo"]` / `["foo"]` | Bracketed object access (allows non-identifier keys). |
| `.foo?` | Optional access — yields nothing instead of erroring on a missing key or wrong type. |
| `.[N]` | Array index (zero-based). Negative indices count from the end (`.[-1]` = last). Out-of-range yields `null`. |
| `.[]` | Iterate — streams every element of a sequence or every value of a mapping. |
| `.a.b.c`, `.a[0].b` | Chained paths (mixed dot/bracket). |
| `A \| B` | Pipe — feeds each output of `A` as input to `B`. |
| `[ E ]` | Collection — gathers the whole stream produced by `E` into one sequence. |

### Semantics & edge cases

- **Stream model.** Like `jq`, every filter maps one input value to a stream of
  zero or more output values. Multiple results are emitted as multiple YAML
  documents (separated by `---`, unless `--no-doc-sep`) or multiple JSON values.
- **Missing key.** `.nope` is an error (exit 1); suffix with `?` for an empty
  result instead.
- **Out-of-range index.** `.[99]` yields `null` (matches `jq`).
- **Wrong-type access.** Indexing a scalar, or treating a sequence as an object,
  is an error unless suffixed with `?`.
- **Multi-document streams.** The filter runs against every `---`-separated
  document; outputs are re-joined with `---`.
- **Anchors & aliases.** On identity/whole-document output, anchors and aliases
  are preserved. When a *subtree* containing an alias is extracted, the alias is
  emitted as resolved content so the output is self-contained valid YAML.
- **Comments.** Preserved on YAML output via `yaml.v3` head/line/foot comments.
  JSON output necessarily drops comments.
- **Type fidelity.** YAML tags (`!!int`, `!!bool`, `!!null`, `!!str`) are
  respected, so `42` stays a number and `"42"` stays a string through JSON
  conversion.
- **Raw output (`-r`).** String scalars are printed without surrounding quotes;
  non-string values are unaffected.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. |
| `1` | Runtime/evaluation error (e.g. missing key without `?`). |
| `2` | Usage/parse error (bad flags, malformed filter, `-i` without a file). |
| `3` | I/O error (file not found, read/write failure). |

## Out of scope

Arithmetic/boolean/comparison operators, built-in functions (`length`, `keys`,
`map`, `select`, …), object construction `{a: .b}`, variables, recursive descent
`..`, slices `.[2:4]`, and assignment/update (`|=`, `=`) are intentionally not
implemented in this iteration. The `Filter` interface and node-stream contract
leave room to add them —

## Project layout

```
46-yq/
  cmd/ccyq/main.go          CLI driver, flags, exit codes
  query/                    from-scratch filter engine
    lexer.go  parser.go  ast.go  eval.go
    query_test.go
  yamlio/                   loader & emitter (yaml.v3 / encoding/json)
    load.go  emit.go
    yamlio_test.go
  testdata/                 fixtures + golden outputs
```

### Caveats

- JSON **object key order** is not preserved on `-p json` input (Go decodes JSON
  objects into unordered maps); keys are emitted sorted for determinism. YAML
  input preserves key order.
- JSON output drops YAML comments and anchors (JSON has no equivalent).
