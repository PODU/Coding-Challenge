# ccjq — a `jq` clone in Go

`ccjq` is a command-line JSON processor implementing a realistic subset of
[`jq`](https://jqlang.github.io/jq/). It is the solution to John Crickett's
[Coding Challenge #34](https://codingchallenges.substack.com/p/coding-challenge-34-jq).

It reads JSON from a file or `stdin`, applies a **filter program**, and writes the
resulting JSON value(s) to `stdout`. Output is pretty-printed by default; flags
switch to compact or raw.

```
bytes ─▶ [JSON decoder] ─▶ value ─▶ [filter evaluator] ─▶ stream of values ─▶ [encoder] ─▶ bytes
                                         ▲
                       filter source ─▶ [lexer] ─▶ [parser] ─▶ AST
```

The guiding model, inherited from jq: **a filter is a function from one input
value to a _stream_ of zero or more output values.** Identity, field access,
`.[]`, pipe, comma and array construction are all instances of that one idea.

## Build & run

```bash
cd 34-jq
go build -o ccjq ./cmd/ccjq      # produces a single static binary
go test ./...                    # run the full test suite

echo '[1,2,3]' | ./ccjq '.[0]'   # -> 1
./ccjq '.users[] | select(.age > 30) | .name' users.json
```

Requires Go 1.21+. No third-party dependencies.

## CLI

```
ccjq [OPTIONS] [FILTER] [FILES...]
```

- `FILTER` is optional and defaults to identity `.` (so bare `ccjq` pretty-prints).
- With no `FILES`, input is read from `stdin`. Multiple files are concatenated
  into one input stream. Multiple top-level JSON values are processed in turn.

| Flag | Long | Meaning |
|------|------|---------|
| `-r` | `--raw-output` | Strings printed without quotes; other values unchanged. |
| `-j` | `--join-output` | Like `-r` but no trailing newline between outputs. |
| `-c` | `--compact-output` | Single-line, no insignificant whitespace. |
| `-n` | `--null-input` | Don't read input; run the filter once with `null`. |
| `-s` | `--slurp` | Read the whole input stream into one array, run once. |
| `-e` | `--exit-status` | Exit status reflects the last output value (see below). |
| `-S` | `--sort-keys` | Sort object keys in the output. |
| `-a` | `--ascii-output` | Escape non-ASCII as `\uXXXX`. |
|      | `--tab` | Indent with tabs (pretty mode). |
|      | `--indent N` | Indent with N spaces (0–8). |
| `-h` | `--help` | Usage. |
| `-V` | `--version` | Version string. |

### Exit codes (jq-compatible)

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | With `-e`, the last output was `false` or `null`. |
| 2 | Usage error, bad option, file not found, or invalid input JSON. |
| 3 | Compile error (the filter failed to parse). |
| 4 | With `-e`, no output was produced. |
| 5 | Uncaught runtime error during evaluation. |

## Supported filter language

### Identity & recursion
- `.` — identity
- `..` — recursive descent (every value in the tree)

### Field / index access
- `.foo`, `.foo.bar` — field access (chains)
- `.["foo"]`, `."foo"` — quoted field access (keys with spaces/symbols)
- `.[2]` — array index; negative indices count from the end (`.[-1]`); out of
  range → `null`
- `.[]` — iterate: every element of an array / every value of an object
- `.[1:3]`, `.[:3]`, `.[2:]` — array/string slices (half-open, negatives allowed,
  clamped to bounds)
- `?` suffix — optional: suppress a type error and emit nothing instead
  (`.foo?`, `.[0]?`, `.[]?`)

### Composition & construction
- `f | g` — pipe
- `f, g` — comma (stream concatenation)
- `( f )` — grouping
- `[ f ]` — array construction (collect the whole stream into one array)
- `{ a: f, "b": g, c, (expr): v }` — object construction with bare-identifier,
  string-literal, computed `(expr)` and shorthand `{user}` ≡ `{user: .user}`
  keys. Multi-valued value filters produce the Cartesian product (as in jq).

### Literals
- `null`, `true`, `false`, numbers, strings with escapes incl. `\uXXXX`
- String interpolation `"\(expr)"` and `@base64 "\(x)"`-style format application

### Operators
- Arithmetic `+ - * / %` with jq type semantics:
  - numbers: ordinary arithmetic (`/` and `%` by zero raise a catchable error)
  - `+`: number add, string concat, array concat, object merge (right wins),
    `null` is the identity
  - `-`: number subtract, array difference
  - `*`: number multiply, recursive object merge
  - `/`: number divide, string split (`"a,b" / ","`)
- Comparisons `== != < <= > >=` over the total order
  `null < false < true < numbers < strings < arrays < objects`
- `and`, `or`, `not` (`not` is a 0-arg builtin); `false`/`null` are falsy
- `//` — alternative: first non-`null`/non-`false` output of the left, else the right

### Control flow
- `if C then A elif C2 then B else D end` (`elif`/`else` optional; no `else` ⇒
  identity on non-match)
- `try f catch g` and the `f?` shorthand for `try f`

### Builtin functions
Invoked `name` or `name(arg; arg)` — jq separates call arguments with **`;`**,
while `,` inside an argument is the comma operator.

- **Structure/type:** `length`, `utf8bytelength`, `keys`, `keys_unsorted`,
  `has(k)`, `in(o)`, `contains(x)`, `inside(x)`, `type`, `empty`, `error`,
  `error(msg)`, `not`
- **Type filters:** `values`, `nulls`, `booleans`, `numbers`, `strings`,
  `arrays`, `objects`, `iterables`, `scalars`
- **Arrays/objects:** `add`, `any`, `any(f)`, `all`, `all(f)`, `flatten`,
  `flatten(d)`, `range(n)`, `range(a;b)`, `range(a;b;s)`, `min`, `max`,
  `min_by(f)`, `max_by(f)`, `sort`, `sort_by(f)`, `unique`, `unique_by(f)`,
  `group_by(f)`, `reverse`, `to_entries`, `from_entries`, `with_entries(f)`,
  `floor`
- **Strings:** `tostring`, `tonumber`, `ascii_downcase`, `ascii_upcase`,
  `ltrimstr(s)`, `rtrimstr(s)`, `startswith(s)`, `endswith(s)`, `split(s)`,
  `join(s)`, `test(re)`
- **Formats:** `@base64`, `@base64d`, `@json`, `@text`, `@csv`, `@tsv`
- **Higher-order:** `map(f)`, `map_values(f)`, `select(cond)`, `recurse`,
  `recurse(f)`, `paths`, `getpath(p)`, `setpath(p; v)`, `del(path)`, `walk(f)`
- **I/O-ish:** `tojson`, `fromjson`

## Examples

```bash
ccjq                  < data.json        # pretty-print (implicit identity)
ccjq '.[0]'           commits.json       # first array element
ccjq '.[0] | .commit.message' commits.json
ccjq '[.quotes[].quote]' quotes.json     # collect into an array
ccjq -r '.name'       < user.json        # raw string output
ccjq -c '.'           < data.json        # compact
ccjq 'group_by(.dept) | map({dept: .[0].dept, n: length})' staff.json
```

## Architecture

```
34-jq/
├── cmd/ccjq/main.go        # CLI: flag parsing, wiring, exit codes
├── internal/jq/
│   ├── value.go            # JSON value model + insertion-ordered Object
│   ├── order.go            # total ordering / equality / truthiness
│   ├── decode.go           # streaming, order-preserving JSON decoder
│   ├── encode.go           # pretty / compact / raw / sort / ascii encoder
│   ├── lexer.go            # filter-source tokeniser (incl. string interpolation)
│   ├── ast.go              # AST node types
│   ├── parser.go           # recursive-descent + Pratt precedence-climbing parser
│   ├── eval.go             # tree-walking, callback-based stream evaluator
│   ├── arith.go            # operators, comparisons, @-formats
│   ├── path.go             # getpath / setpath / del / paths
│   ├── builtins.go         # builtin function registry
│   └── regex.go            # test(re) via Go's regexp
└── testdata/               # JSON fixtures
```

The evaluator uses a callback (`func(Value) error`) to model jq's value stream,
so filters such as `empty` and `first-output` builtins short-circuit without
allocating intermediate slices.

## Tests

- **Golden matrix** (`internal/jq/eval_test.go`): ~150 `filter → output` cases
  covering every supported feature, plus runtime-error and compile-error cases.
- **Differential** (`internal/jq/differential_test.go`): runs the same
  `(filter, input)` pairs through real `jq` and asserts byte-equality (modulo
  Windows CRLF). Automatically **skipped** when `jq` is not on `PATH`.
- **Unit** (`internal/jq/unit_test.go`): ordering, encoder modes, number
  formatting, lexer, ordered decode.
- **CLI/E2E** (`cmd/ccjq/main_test.go`): flags, output modes and every exit code.

```bash
go test ./...          # all of the above
go vet ./...
gofmt -l .             # should print nothing
```

## Deliberately out of scope (v1)

These real-jq features are excluded by design (clean extension points remain):

- User-defined functions (`def f: …;`) and `as $x` bindings / destructuring.
- Reductions: `reduce`, `foreach`, `label`/`break`.
- Update-assignment operators (`|=`, `+=`, `*=`, `=`) and general path expressions
  beyond `del` / `getpath` / `setpath`.
- `input`/`inputs`, `$ENV`/`env`, `$__loc__`, modules (`import`/`include`),
  `--arg`/`--argjson`.
- `match`/`capture`/`sub`/`gsub` and PCRE regex flags (only basic `test(re)` via
  Go's RE2 engine).

### Number formatting note

Numbers are evaluated as IEEE-754 doubles. Integer values round-trip exactly
(`10`, `-5`, `2.5`, `3.14`). jq 1.7+ canonicalises some non-integer / very large
literals differently (e.g. `1e10` → `1E+10`); `ccjq` uses Go's shortest
round-trippable formatting instead. This only affects exotic numeric literals,
not the common integer/decimal cases.
