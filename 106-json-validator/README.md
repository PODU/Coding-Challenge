# jsonschema-validate — JSON Schema Validator (Coding Challenge #106)

A command-line tool that validates a JSON **instance** document against a JSON
**Schema** (JSON Schema **Draft 7** subset) and reports **every** validation
error with a precise [RFC 6901 JSON Pointer](https://datatracker.ietf.org/doc/html/rfc6901)
location.

The validation engine is written **by hand** — no third-party JSON Schema
library is used. Only the Go standard library is used: `encoding/json` (with
`UseNumber()`) turns bytes into value trees, and `math/big` provides exact
numeric comparisons. Everything about schema *semantics* (keyword evaluation,
error collection, path tracking, `$ref` resolution) is implemented from first
principles.

## Build & run

```sh
go build -o jsonschema-validate ./cmd/jsonschema-validate

# validate a file
./jsonschema-validate --schema schema.json instance.json

# read the instance from stdin
cat instance.json | ./jsonschema-validate --schema schema.json -

# machine-readable output
./jsonschema-validate --schema schema.json --format json instance.json
```

### Flags

| Flag / arg            | Description                                              | Default |
|-----------------------|----------------------------------------------------------|---------|
| `--schema <path>`     | Path to the JSON Schema document. **Required.**          | —       |
| `<instance.json>`     | Positional instance path. `-` reads stdin.               | —       |
| `--format human\|json`| Output format.                                           | `human` |
| `--assert-format`     | Treat `format` as an assertion (else annotation-only).   | off     |
| `--strict`            | Warn (on stderr) about unknown keywords / unresolved `$ref`. | off |
| `--max-errors <n>`    | Print at most N errors (`0` = unlimited).                | `0`     |
| `--max-depth <n>`     | Recursion / `$ref` depth guard.                          | `256`   |
| `-h, --help`          | Usage.                                                   | —       |

### Output

- **human** (default): `OK` on success; otherwise one line per error:
  `<instance-pointer>: <message> (keyword: <kw>, schema: <schema-pointer>)`.
  The document root is shown as `(root)`.
- **json**: `{"valid": <bool>, "errors": [{"instancePath","schemaPath","keyword","message"}]}`.
  Errors are sorted deterministically by instance path, then keyword, so output
  is stable for golden testing.

### Exit codes

| Code | Meaning                                                       |
|------|---------------------------------------------------------------|
| `0`  | Instance is **valid**.                                        |
| `1`  | Instance is **invalid** (validation errors found).            |
| `2`  | Usage error (bad flags / missing required arg).               |
| `3`  | Input error (file not found, malformed JSON in schema/instance). |

Separating `1` (validation) from `3` (I/O / parse) lets scripts distinguish
"the data is wrong" from "the tooling/input is wrong".

## Supported keywords (Draft 7 subset)

| Category    | Keywords |
|-------------|----------|
| Core        | `$ref` (local pointer refs: `#`, `#/definitions/X`, any `#/<json-pointer>`), `definitions` |
| Type        | `type` (single name or array; `integer` = number with zero fractional part) |
| Generic     | `enum`, `const` (deep JSON-equality) |
| Numeric     | `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf` |
| String      | `minLength`, `maxLength` (Unicode code points), `pattern` (RE2, unanchored), `format` |
| Object      | `properties`, `required`, `additionalProperties`, `patternProperties`, `minProperties`, `maxProperties`, `propertyNames` |
| Array       | `items` (single schema **or** tuple), `additionalItems`, `minItems`, `maxItems`, `uniqueItems`, `contains` |
| Applicators | `allOf`, `anyOf`, `oneOf` (exactly one), `not`, `if` / `then` / `else` |
| Boolean     | `true` / `false` schemas (also at sub-positions, e.g. `additionalProperties: false`) |

`format` supports `email`, `date`, `date-time`, `ipv4`, `uri`, `uuid` and is an
**annotation by default** (no failure); pass `--assert-format` to make it an
assertion. Unrecognised `format` values never fail.

### Behaviour notes

- **No type coercion.** A JSON string `"5"` is **not** a number — JSON Schema's
  no-coercion rule is followed (form-validator users often expect coercion).
- **`integer` vs `number`.** `1.0` is a valid `integer` (zero fractional part);
  `1.5` is not. Checked on the exact literal via `math/big`, not float rounding.
- **Number precision.** Numbers are kept as `json.Number` (`UseNumber()`) and
  compared with `math/big.Rat`, so large integers and `multipleOf` with floats
  (e.g. `0.1`) are handled exactly.
- **`minLength`/`maxLength`** count Unicode code points (runes), not bytes.
- **`additionalProperties`** treats a key as "additional" only if covered by
  neither `properties` nor a matching `patternProperties` entry.
- **`oneOf`** reports distinctly when zero branches match vs. more than one.
- **Noise suppression.** Errors from a *passing* `anyOf`/`oneOf`/`not`/`if`
  branch never leak into the final report (sub-validation into a temporary
  collector).
- **`$ref` siblings.** Per Draft 7, when `$ref` is present its sibling keywords
  are ignored — only the referenced schema applies (differs from 2019-09).
- **`$ref` cycles** (e.g. a recursive tree schema) are broken via a visited-set
  keyed on `(ref target, instance location)` plus the `--max-depth` guard; no
  infinite loops, no panics.

### Out of scope (documented, not silently mis-handled)

Remote/URL `$ref` (http/file) — reported as an error; `$id` / base-URI
re-scoping; `$recursiveRef` / `$dynamicRef`; `dependencies` /
`dependentSchemas`; `contentEncoding` / `contentMediaType`; and
draft-2019-09 / 2020-12-only keywords. Unknown keywords are ignored per spec
(use `--strict` to get warnings).

## Project layout

```
106-json-validator/
├── go.mod
├── cmd/jsonschema-validate/main.go   # CLI: flags, IO, exit codes, formatters
├── internal/validator/
│   ├── loader.go            # parse JSON (UseNumber)
│   ├── schema.go            # type-name accessors
│   ├── validate.go          # recursive core + dispatch + Options/Result
│   ├── keywords_type.go     # type, enum, const
│   ├── keywords_number.go   # minimum/maximum/exclusive*/multipleOf
│   ├── keywords_string.go   # minLength/maxLength/pattern/format
│   ├── keywords_object.go   # properties/required/additional*/pattern*/counts/propertyNames
│   ├── keywords_array.go    # items/additionalItems/min-maxItems/uniqueItems/contains
│   ├── keywords_logic.go    # allOf/anyOf/oneOf/not/if-then-else
│   ├── ref.go               # local $ref resolver + cycle guard
│   ├── pointer.go           # RFC 6901 JSON Pointer
│   ├── equal.go             # JSON deep-equality
│   ├── num.go               # exact numeric helpers (math/big)
│   ├── format.go            # format checkers + pattern cache
│   └── errors.go            # ValidationError + collector + deterministic sort
└── testdata/golden/<keyword>/   # file-based golden valid/invalid pairs
```

## Tests

```sh
go test ./...           # unit + table-driven + file-based golden harness
go test -cover ./...    # coverage
go vet ./...
gofmt -l .              # cleanliness gate (no output == clean)
```

- **Table-driven golden tests** (`internal/validator/validator_test.go`) cover
  **every** supported keyword with valid + invalid instances and assert the
  exact `(instancePath, keyword)` of each error — including nested
  objects/arrays, combinators, `if/then/else`, recursive `$ref`, and
  multi-error collection.
- **File-based golden harness** (`golden_test.go`) walks
  `testdata/golden/<keyword>/`, validating `valid_*.json` (must pass) and
  `invalid_*.json` against a sibling `.golden` file of expected error keys.
- **Unit tests** cover Pointer rendering / RFC 6901 escaping, `deepEqual`,
  `$ref` resolution + cycle detection, and numeric edge cases.
- **CLI tests** (`cmd/.../main_test.go`) cover exit codes (valid / invalid /
  usage / input), stdin, `--format json`, `--max-errors`, and malformed input.

> Note: `go test -race` requires a working cgo/C toolchain. On a stock Windows
> Go install without 64-bit gcc the race detector can't link; the suite passes
> under plain `go test`.
