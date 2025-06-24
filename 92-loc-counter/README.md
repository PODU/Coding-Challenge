# loc — a lines-of-code counter (cloc clone)

> [John Crickett's Coding Challenge #92](https://codingchallenges.substack.com/p/coding-challenge-92-loc-counter)

`loc` recursively walks one or more paths, detects each file's programming
language by extension, and classifies every physical line as **blank**,
**comment**, or **code**. It aggregates the counts per file and per language and
prints a summary table (or JSON), with grand totals. It is written in Go with no
third-party dependencies.

The core is a small, table-driven per-line state machine that correctly handles
the tricky cases: single- and multi-line (block) comments, comment markers that
appear *inside string literals* (which must **not** count as comments), strings
inside comments, mixed code+comment lines, nested block comments, raw strings,
and Unicode.

---

## Build & run

```sh
cd 92-loc-counter
go build -o loc ./cmd/loc      # single static, dependency-free binary

./loc .                        # per-language summary for the current directory
./loc --by-file ./src          # per-file breakdown
./loc --json --cocomo .        # JSON output + a COCOMO estimate
./loc --files .                # just list the discovered files
go test ./...                  # run the full test suite
```

Example:

```
-------------------------------------------------
 Language    Files  Lines  Blanks  Comments  Code
-------------------------------------------------
 Go             16   1616     133       143  1340
 Markdown        2    415      68       222   125
 C               2     13       1         5     7
-------------------------------------------------
 Total          20   2044     202       370  1472
-------------------------------------------------
```

---

## CLI

```
loc [flags] [path ...]

Positional:
  path...            One or more files or directories. Default: "." (cwd).

Flags:
  --by-file          Per-file breakdown instead of the per-language summary.
  --files            Just list discovered files and exit.
  --json             Emit JSON instead of the text table.
  --sort <key>       code|lines|comments|blanks|name  (default: code, desc).
  --include-unknown  Count files with unrecognized extensions as "(unknown)".
  --no-ignore        Do not skip .git/node_modules/vendor/etc.
  --exclude <glob>   Repeatable; skip paths matching the glob.
  --lang <name>      Repeatable; restrict to the named language(s).
  --cocomo           Append a COCOMO organic-model estimate.
  --avg-wage <n>     Annual wage for the COCOMO cost (default 56286).
  --threads <n>      Worker count (default: GOMAXPROCS).
  --version          Print version and exit.

Exit codes: 0 ok; 1 usage error; 2 a path did not exist / was unreadable.
```

---

## How classification works

Each file is read line by line. A `scanState` is carried **across** lines so a
block comment or raw string can span many lines. For each physical line the
scanner walks rune-by-rune and tracks two flags, `sawCode` and `sawComment`:

```
blank   := line is empty or all whitespace        (blank always wins)
code    := !blank && sawCode                       (code wins on mixed lines)
comment := !blank && sawComment && !sawCode
```

This matches the physical-line semantics of `cloc`/`scc`: a line with any code
token counts as **code**, even if it also has a trailing comment. A line is
**comment** only when it is comment-only.

At each position in normal (code) state the scanner picks the **longest**
matching marker among string openers, block-comment openers, and line-comment
markers. Longest-match is what lets Python's `"""` (modelled as a block) win over
`"` (a string), and what keeps a `//` inside a `"…"` string from being read as a
comment.

### Edge cases handled

- `//` / `#` inside a string literal (e.g. a URL) → code, not comment.
- Quotes inside a block comment → inert, stays comment.
- Mixed `i++; /* x */` → counts as one **code** line.
- Block comment spanning many lines, and a block left unterminated at EOF.
- Nested block comments (Rust-style `/* /* */ */`).
- Raw strings (Go backticks) that ignore escapes and span lines.
- Backslash-escaped quotes inside strings (`"a\"b"`).
- CRLF / LF / missing trailing newline; Unicode / multibyte content.
- Plain text (every non-blank line is code) and Markdown (fenced ``` blocks are
  code, prose is comment).

---

## Adding a language

Language support is **data**, not code. Append one struct literal to
`internal/lang/registry.go`:

```go
{
    Name:          "Kotlin",
    Extensions:    []string{".kt", ".kts"},
    LineComments:  []string{"//"},
    BlockComments: [][2]string{{"/*", "*/"}},
    Strings:       [][2]string{{`"`, `"`}, {`'`, `'`}},
},
```

No control-flow changes are needed anywhere else. The starter registry ships
Go, Python, JavaScript, TypeScript, Java, C, C++, C#, Rust, Ruby, Shell, HTML,
CSS, SQL, YAML, JSON, Makefile, Dockerfile, plain Text, and Markdown.

---

## Package layout

```
92-loc-counter/
  cmd/loc/main.go              # flag parsing, worker pool, exit codes
  internal/walk/walk.go        # recursive discovery, ignore rules, binary sniff
  internal/lang/lang.go        # LanguageDef type
  internal/lang/registry.go    # the language table (data)
  internal/lang/detect.go      # filename/extension -> LanguageDef
  internal/count/classify.go   # the per-line state machine (core)
  internal/count/file.go       # read a file, run the classifier -> FileCount
  internal/report/aggregate.go # FileCount[] -> Report (per-language + total)
  internal/report/render.go    # text table + JSON + COCOMO
  internal/count/testdata/     # fixtures with committed golden counts
```

---

## Notes / caveats

- **Per-line, not per-token**: counts are by physical line. A line containing
  both code and a comment counts as code (one line), matching cloc/scc; the
  separate comment is acknowledged but not double-counted as its own line.
- **Python docstrings** are approximated by treating `"""`/`'''` as block
  comments. A triple-quoted *string assigned to a variable* on its own line is
  therefore counted as a comment — a deliberate, documented simplification that
  matches how most LOC tools behave.
- **Ruby** `=begin`/`=end` block comments are modelled as a block pair; this is
  accurate for typical column-0 usage.
- `--exclude` globs use Go's `filepath.Match` syntax and are tested against both
  the base name and the full path.
- Binary files (a NUL byte in the first 8 KB) and files with unknown extensions
  are skipped by default; symlinked directories are not followed.
