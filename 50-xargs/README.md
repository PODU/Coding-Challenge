# ccxargs — Build Your Own `xargs`

A from-scratch, production-ready reimplementation of `xargs` in Go (Coding
Challenge #50). It reads whitespace- or NUL-delimited items from stdin (or a
file), builds command lines, and runs them — optionally in parallel.

## Build & test

```sh
go build -o ccxargs .     # produces ./ccxargs(.exe)
go vet ./...
go test ./...
```

Requires Go 1.24+. No third-party dependencies.

## Usage

```
ccxargs [OPTIONS] [command [initial-arguments...]]

  -0, --null               Items are NUL-separated; disables quote/space splitting.
  -d, --delimiter CHAR     Use CHAR (or escape \n \t \r \0 \\) as the sole delimiter.
  -n, --max-args N         At most N items per command invocation (N >= 1).
  -s, --max-chars N        At most N bytes per constructed command line.
  -I, --replace[=R]        Replace token R (default {}) in args with each input item;
                           implies one command per item.
  -P, --max-procs N        Run up to N invocations in parallel (0 = max). Default 1.
  -a, --arg-file FILE      Read items from FILE instead of stdin.
  -r, --no-run-if-empty    Do not run the command if input is empty.
  -t, --verbose            Print each command to stderr before running it.
  -E EOF                   Stop reading at an item equal to EOF (whitespace mode).
  -h, --help               Show usage.
```

If `command` is omitted, `echo` is used (`cmd /c echo` on Windows — see below).

`-n5` and `-n 5` are both accepted. Options are recognised only *before* the
command name; the first non-option token (or the token after `--`) begins the
command and everything after it is literal.

### Examples

```sh
# default command is echo
echo a b c | ccxargs                       # -> a b c

# one invocation per item
ls | ccxargs -n 1 cat

# template substitution, one command per item
ls | ccxargs -I {} mv {} backup/{}

# NUL-delimited input from find
find . -name '*.log' -print0 | ccxargs -0 rm

# 10 downloads in parallel, one URL each
cat urls.txt | ccxargs -n 1 -P 10 curl -O
```

## Architecture

```
stdin / -a file  ──▶  Tokenizer  ──▶  Command Builder  ──▶  Executor  ──▶  os/exec
                      (SplitFunc)      (batch / -I tmpl)     (worker pool
                      ws / nul / -d                           + -P semaphore)
```

- **`tokenizer.go`** — a `bufio.Scanner` with a mode-selected `SplitFunc`:
  whitespace (POSIX quoting + backslash escapes), NUL (`-0`), and single-byte
  delimiter (`-d`). Empty fields between delimiters are preserved; a trailing
  delimiter never yields an empty item. `-E` truncates input in whitespace mode.
- **`builder.go`** — pure `items -> []Job`. *Append* mode batches by `-n` and
  `-s`; *replace* mode (`-I`) substitutes the token into the fixed arguments and
  emits one job per item.
- **`executor.go`** — a goroutine worker pool bounded by a `-P` semaphore. `-P 1`
  degenerates to serial, input-ordered execution. Each child's result is mapped
  to a POSIX exit code; the worst result becomes `ccxargs`'s own exit code. A
  fatal result (124/125/126/127) stops new jobs from launching while in-flight
  jobs drain.
- **`main.go`** — CLI parsing, default-command resolution, and wiring.

## Exit codes (POSIX)

| Code | Meaning |
|------|---------|
| 0    | all invocations succeeded |
| 123  | one or more commands exited 1–125 |
| 124  | a command exited 255 |
| 125  | a command was killed by a signal |
| 126  | command found but could not be executed |
| 127  | command not found |
| 1    | `ccxargs` itself failed (bad option, I/O error) |

## Windows vs. Unix execution differences

`ccxargs` execs binaries directly through `os/exec` — **it never invokes a
shell** (no globbing, no shell operators), exactly like real `xargs`. That has a
few platform consequences:

- **`echo` is not a binary on Windows.** It is a `cmd.exe` builtin, so the
  default command resolves to `cmd /c echo` on Windows and `echo` on Unix
  (`runtime.GOOS`). When you supply your own command on Windows, wrap builtins
  yourself, e.g. `... | ccxargs -n 1 cmd /c echo`.
- **Command-line length.** Unix enforces `ARG_MAX` (`E2BIG`); Windows caps the
  `CreateProcess` command line near 32 KiB. When `-s` is not given, `ccxargs`
  applies a conservative default cap (30000 bytes on Windows, 131072 on Unix) so
  a batch never overflows the OS limit. A single item larger than `-s` cannot be
  split and is still emitted on its own.
- **Argument quoting.** Unix passes a real `argv[]`; Windows reconstructs one
  command-line string that the child re-parses. `ccxargs` builds explicit argv
  slices and relies on Go's `syscall.EscapeArg`. Programs with non-standard
  parsing (notably `cmd.exe`) may still need care.
- **Signals.** "Fatal on signal" (exit 125) maps cleanly on Unix. Windows has no
  POSIX signals, so an abnormally terminated child surfaces as a non-zero exit
  code and is mapped to the closest POSIX code.
- **Parallel output ordering.** With `-P > 1` child stdout interleaves; output is
  not serialized. This matches real `xargs`.

> Note for Git Bash / MSYS users: the shell rewrites a bare `/c` argument into a
> Windows path (`C:/`) before `ccxargs` ever sees it. Use
> `MSYS_NO_PATHCONV=1 ccxargs ... cmd /c echo` (or run from PowerShell/cmd) when
> driving `cmd /c` from Git Bash. This is a shell artifact, not a `ccxargs` bug.

## Tests

- `tokenizer_test.go` — whitespace/quote/escape tables, `-0`, `-d` (incl. empty
  fields), `-E`, delimiter-escape parsing, unbalanced-quote error.
- `builder_test.go` — `-n` batch boundaries, default command, `-s` flush +
  oversized item, `-I` substitution (incl. multiple occurrences), empty input
  with and without `-r`.
- `executor_test.go` — real process spawns against a portable command
  (`cmd /c echo` on Windows, `echo`/`sh -c` on Unix): serial, parallel,
  exit-code mapping (123/124/255/worst-wins), command-not-found (127), and a
  Unix-only killed-by-signal case (125, skipped on Windows).
