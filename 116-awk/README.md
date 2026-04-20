# goawk — a POSIX-subset awk in Go

Coding Challenge #116. A production-ready implementation of the `awk`
text-processing language built as a classic three-stage interpreter:

```
source ──▶ lexer ──tokens──▶ parser ──AST──▶ interpreter ──▶ output
                                                  │
                            input records ────────┘  (record/field engine,
                                                       variables, builtins)
```

## Build & run

```sh
go build -o goawk ./cmd/goawk
go test ./...

./goawk '{ print $1 }' data.txt
./goawk -F: '{ print $1, $3 }' /etc/passwd
printf 'a\nb\n' | ./goawk 'END{ print NR }'
```

## CLI

```
goawk [-F fs] [-v var=value]... [-f progfile]... ['program'] [file | var=val]...
```

| Flag / arg     | Meaning                                                            |
|----------------|-------------------------------------------------------------------|
| `-F fs`        | Set `FS` (single char or regex; `-F'\t'` and escapes understood). |
| `-v var=value` | Assign before `BEGIN`. Repeatable. Backslash escapes processed.   |
| `-f progfile`  | Read program from file. Repeatable (concatenated).                |
| `program`      | The awk program text (required unless `-f` given).                |
| `file…`        | Input files. `-` means stdin. Absent ⇒ read stdin.                |
| `var=value`    | Assignment among files, applied when reached in `ARGV`.           |
| `--`           | End of options.                                                   |

Exit status: `0` normally, the argument of `exit n`, or `2` on a usage / parse
/ runtime error (reported on stderr with a `goawk:` prefix).

## Supported subset

**Records & fields**
- Input from files or stdin; records separated by `RS` (default newline).
  Single-character `RS` and paragraph mode `RS=""` are supported.
- Field splitting: default whitespace runs; single-char `FS` (incl. `"\t"`);
  multi-char / regex `FS`; per-character `split(s,a,"")`.
- `$0`, `$1 … $NF`, computed `$expr` (`$NF`, `$(NF-1)`). Assigning a field
  rebuilds `$0` via `OFS`; assigning `$0` re-splits and recomputes `NF`;
  assigning `NF` truncates/extends.

**Built-in variables**
`NR NF FNR FS OFS ORS RS SUBSEP FILENAME RSTART RLENGTH CONVFMT OFMT
ARGC ARGV ENVIRON`.

**Patterns**
- Expression patterns, `/regex/` (matched against `$0`), `~` / `!~`.
- Comparison `== != < > <= >=`, logical `&& || !`.
- `BEGIN` / `END`, range patterns `/a/,/b/` (inclusive, re-armable), empty
  pattern (matches all). Multiple rules, top-to-bottom.

**Values & operators**
- Dynamic string/number typing with POSIX numeric-string (`strnum`) rules;
  uninitialised scalars are `"" / 0`.
- Arithmetic `+ - * / % ^` (and `**`), unary `+ -`, pre/post `++ --`,
  string concatenation by juxtaposition.
- Assignment `= += -= *= /= %= ^=`. Number→string via `CONVFMT`/`OFMT`;
  integral values print without a decimal point.

**Control flow**
`if/else`, `while`, `do…while`, C-style `for`, `for (k in arr)`,
`break continue next nextfile exit [code]`, ternary `?:`, `{ … }` grouping.

**Arrays**
Associative, string-indexed; multi-subscript `a[i,j]` joined with `SUBSEP`;
`(k in arr)` and `(i,j) in arr`; `for (k in arr)`; `delete arr[k]` and
`delete arr`.

**Output & string functions**
- `print` (args joined by `OFS`, terminated by `ORS`), `printf`, `sprintf`
  with `%d %i %o %x %X %u %c %e %E %f %g %G %s %%`, flags, width, precision
  and `*`.
- `length substr index split sub gsub match tolower toupper`.
  `&` / `\&` in `sub`/`gsub` replacements.

**Functions & math**
- `function name(params){…}` (and `func`), arrays by reference, scalars by
  value, extra params are locals, `return [expr]`.
- `int sqrt exp log sin cos atan2 rand srand`.

**getline, redirection & pipes**
- `print > "f"`, `print >> "f"`, `print | "cmd"`.
- `getline`, `getline var`, `getline < "file"`, `"cmd" | getline [var]`.
- `close(x)`, `fflush([x])`, `system(cmd)`.

## Architecture

```
cmd/goawk/main.go   CLI: flag parsing, ARGV/ENVIRON, program loading
lexer/              source text -> tokens (regex/division disambiguation)
ast/                node type definitions
parser/             recursive-descent parser + array-parameter analysis
interp/             tree-walking interpreter
  value.go          awk value model (num / str / strnum coercion)
  interp.go         driver, BEGIN/main/END, environment, special vars
  record.go         record reader + field split/rebuild engine
  eval.go           expression evaluation
  exec.go           statement execution
  builtins.go       string + math builtins
  printf.go         printf/sprintf engine
  io.go             input iteration, getline, redirection, pipes, functions
```

## Testing

`go test ./...` runs table-driven golden tests (program + input → output) plus
file-based cases (multiple files, redirection, `getline < file`) and, where a
system `awk` is on `PATH`, a batch of programs is cross-checked against it.

## Out of scope (v1)

Coprocesses `|&`, `@include`, locale collation, and a bytecode VM. Go's
`regexp` is RE2 (no backreferences), which matches the POSIX ERE semantics awk
targets. Multi-character `RS` honours only its first character.
