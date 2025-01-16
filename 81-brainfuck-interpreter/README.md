# bf — a Brainfuck interpreter

Coding Challenge #81. A production-ready Brainfuck interpreter in Go: a
tokenizer and **bytecode compiler** with a precomputed O(1) jump table and
**optimization passes**, executed by a small virtual machine. A naive
tree-walking interpreter is kept as a correctness oracle.

## Build

```sh
go build -o bf .      # produces ./bf (bf.exe on Windows)
```

No third-party dependencies — only the Go standard library (Go 1.24+).

## Usage

```
bf [flags] [program.bf]   Run a program from a file
bf [flags] -              Read the program from stdin, then run
cat prog.bf | bf -        Same, via a pipe
bf [flags] repl           Interactive REPL (state persists across lines)
```

### Examples

```sh
./bf programs/hello.bf            # -> Hello World!
./bf programs/hello6.bf           # -> Hello!
echo -n 'meow 123' | ./bf programs/cat.bf   # -> meow 123  (echo / cat)
echo -n '+++[-]>>' | ./bf --dump -           # print compiled bytecode
```

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--cells N`   | `30000` | Tape size in cells |
| `--eof MODE`  | `0`     | EOF policy for `,`: `0` (store 0) \| `neg1` (store 255) \| `unchanged` |
| `--wrap-ptr`  | off     | Make the tape circular instead of erroring on out-of-bounds |
| `--opt LEVEL` | `full`  | Optimization level: `none` \| `basic` \| `full` |
| `--naive`     | off     | Use the naive tree-walking interpreter instead of the VM |
| `--dump`      | off     | Print compiled bytecode and exit |
| `-h`, `--help`| —       | Usage |

Program input for `,` comes from stdin; output from `.` goes to stdout;
diagnostics go to stderr.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Runtime error (e.g. data pointer out of bounds) |
| `2` | Compile error (unbalanced brackets) or usage error |
| `3` | I/O error (file not found, read failure) |

## Language model

- **8 commands**: `>` `<` `+` `-` `.` `,` `[` `]`. Every other byte is a comment.
- **Tape**: 30,000 cells by default, each an 8-bit unsigned byte. Cell
  arithmetic wraps modulo 256 (`255 + 1 = 0`, `0 - 1 = 255`).
- **Pointer bounds**: moving below cell 0 or past the last cell is a runtime
  error by default; `--wrap-ptr` makes the tape circular instead.
- **EOF on `,`**: configurable via `--eof` (default stores 0, which is what the
  canonical `cat` program relies on to terminate).

## Architecture

```
source bytes -> Tokenizer -> Bracket matcher / Compiler -> []Instruction -> VM
                (lexer.go)        (compiler.go)            (bytecode)    (vm.go)
```

- `lexer.go` — strips comments, keeping the 8 command bytes with their byte
  offsets (for diagnostics).
- `compiler.go` — single-pass stack-based bracket matcher producing a
  precomputed jump table (O(1) jumps at runtime), plus optimization passes:
  - **basic**: run-length collapse — `+++++` → `Add(5)`, `>>>` → `Move(3)`.
  - **full**: idiom patterns — `[-]`/`[+]` → `Clear`, `[>]`/`[<]` →
    `ScanRight`/`ScanLeft`, and balanced copy/multiply loops such as `[->+<]` →
    `AddTo(offset, factor)`.
- `vm.go` — the bytecode dispatch loop with buffered I/O.
- `interp.go` — the naive interpreter (correctness oracle).
- `tape.go`, `main.go`, `dump.go`, `errors.go` — memory, CLI, debug listing,
  typed errors.

## Tests & benchmarks

```sh
go test ./...                      # full suite
go test -bench=Busy -benchmem .    # optimizer benchmark (none vs basic vs full)
```

The suite asserts byte-exact output for canonical programs (Hello World, the
challenge's "Hello!", cat/echo), verifies that the **VM matches the naive
interpreter** across every optimization level, and covers bracket matching and
errors, cell wrap, pointer bounds, EOF modes, the optimizer's emitted bytecode,
and the CLI exit codes.
