# ccsh — Build Your Own Shell (Coding Challenge #14)

`ccsh` is a small but production-quality interactive shell written in Go. It
implements a read–eval loop, external command execution with arguments, the
`cd` / `pwd` / `exit` / `history` built-ins, `|` pipelines, `> >> <` redirection,
quoting/escaping, persistent command history with arrow-key recall, and SIGINT
handling that interrupts the running child without killing the shell.

## Build & run

```sh
go build -o ccsh .      # produces ccsh (ccsh.exe on Windows)

./ccsh                  # interactive REPL
./ccsh -c "echo hi"     # run one command string and exit
./ccsh script.ccsh      # run a script file line by line, then exit
echo "echo hi" | ./ccsh # piped stdin is executed non-interactively
```

The only third-party dependency is
[`github.com/peterh/liner`](https://github.com/peterh/liner), used solely for
cross-platform raw-mode line editing (arrow-key history, in-line editing).
Everything else uses the Go standard library.

## Features

| Feature | Notes |
|---------|-------|
| REPL with `ccsh> ` prompt | Loops until `exit` or EOF (Ctrl-D). |
| External commands + args | Resolved via `PATH` (`os/exec`). |
| Built-ins | `cd [dir]`, `cd -`, `pwd`, `exit [n]`, `history`, `history -c`. |
| Pipelines | `a | b | c`, wired with OS pipes; status of the last stage is returned. |
| Redirection | `>` (truncate), `>>` (append), `<` (input); combine with pipes. |
| Quoting / escaping | `'single'`, `"double"`, and `\` escapes in the tokenizer. |
| History | Persisted to `~/.ccsh_history`, reloaded on startup, consecutive duplicates suppressed. |
| Signals | Ctrl-C interrupts the child, not the shell. |
| Script mode (bonus) | `ccsh file.ccsh`; lines starting with `#` are comments. |

### Exit-status conventions (POSIX-style)

`0` success · `1` general error · `2` syntax error · `126` not executable ·
`127` command not found · `130` interrupted.

## Architecture

The interpreter is split by concern so the parser and command-dispatch logic are
fully unit-testable **without** a TTY:

| File | Responsibility |
|------|----------------|
| `lexer.go` | `Tokenize` — quotes, escapes, operators (`\| > >> <`). |
| `parser.go` | `Parse` — builds a `Pipeline` of `Command`s with redirections; rejects empty stages, missing redirect targets, and background `&`. |
| `shell.go` | `Shell` state + `Execute` (dispatch to builtin or pipeline). IO streams are injectable, so tests run against `bytes.Buffer`. |
| `builtins.go` | `cd`, `pwd`, `exit`, `history` in a dispatch map. |
| `executor.go` | `RunPipeline` — spawns processes, wires pipes/redirects, forwards SIGINT, collects status. |
| `history.go` | Load / append / save `~/.ccsh_history`. |
| `main.go` | Flags, history load, signal wiring, the `liner`-based REPL, script/`-c`/stdin modes. |
| `ccsh_test.go` | Unit tests (lexer, parser, builtins, history, pipeline execution). |

## Testing

```sh
gofmt -l .       # formatting (no output = clean)
go vet ./...
go build ./...
go test ./...
```

All of the above pass. Tests cover: tokenizer (bare words, quoting, escapes,
operator splitting, unterminated-quote errors), pipe/redirect parsing and syntax
errors, the `cd`/`pwd`/`exit`/`history` built-ins, history round-trip +
duplicate suppression + missing-file tolerance, unknown-command handling, and
2-/3-stage pipeline execution plus pipe-into-redirect — exercised through the
platform command processor (`cmd /c` on Windows, `sh -c` on Unix).

### What is **not** auto-tested (verified by design / manual checklist)

These depend on an interactive console and cannot be driven from `go test`:

- **Arrow-key history recall and in-line editing** — provided by `liner` in raw
  console mode. Verified manually; the underlying history store is unit-tested.
- **Ctrl-C (SIGINT) forwarding** — at the prompt it cancels the current input
  line (`liner` returns `ErrPromptAborted`); while a child runs, the OS delivers
  the interrupt to the child and `Shell.InterruptForeground` also kills the
  registered foreground process. The shell never exits. Verified manually.
- **Ctrl-D (EOF)** — exits cleanly, saving history. Verified manually.

### `go test -race`

The race detector is **not** runnable in this environment: `-race` requires cgo
with a 64-bit toolchain, and the installed MinGW GCC is 32-bit
(`cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`). The pipe/
goroutine wiring is otherwise straightforward (each pipe fd is closed exactly
once in the parent after `Start`). On a machine with 64-bit gcc/clang,
`go test -race ./...` runs unmodified.

## Windows vs. Unix behaviour differences

This shell was developed and tested on **Windows (go1.24.5 windows/amd64)**; the
same source compiles and runs on Linux/macOS. Notable platform differences:

- **Signal delivery.** Unix terminals send `SIGINT` to the whole foreground
  process group, so a child dies naturally on Ctrl-C. Windows has no POSIX
  process groups; `ccsh` registers the foreground child and calls
  `Process.Kill()` from its `os.Interrupt` handler. The net effect (child dies,
  shell survives, status `130`) is the same.
- **Home / history path.** `~/.ccsh_history` resolves to `%USERPROFILE%\.ccsh_history`
  on Windows and `$HOME/.ccsh_history` on Unix (via `os.UserHomeDir`).
- **Available external commands differ.** Examples in the challenge such as
  `ls -la | tr a-z A-Z` are Unix-centric. On Windows use the equivalents, e.g.
  `dir`, `findstr`, `sort`, or invoke the command processor: `cmd /c <command>`.
  The test suite picks `cmd /c` vs `sh -c` automatically per platform.
- **Path separators.** Redirection targets and `cd` arguments use the host
  convention (`\` on Windows, `/` on Unix). Inside double quotes, backslashes in
  Windows paths are preserved literally.
- **Line endings.** History and script files are read with a line scanner that
  tolerates both `\n` and `\r\n`.

## Out of scope (documented, intentionally rejected)

- Background execution (`&`) — parser rejects it with a clear message rather than
  mis-running the command.
- Built-ins inside a pipeline (e.g. `pwd | cat`) run as a normal pipeline stage;
  state-changing built-ins (`cd`) are only honoured as a single command, matching
  typical shell behaviour for this challenge.
- Job control, globbing, and variable/`$` expansion.
