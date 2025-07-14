# `which` — Coding Challenge #93

A production-ready, cross-platform clone of the Unix `which` utility, written in
Go. It locates a program by searching the directories listed in the `PATH`
environment variable, in order, and prints the full path of the first match —
the same binary your shell would invoke.

## Features

- Multiple program names, each resolved independently: `which go ls python3`
- `-a` / `--all` — print **all** matches across `PATH`, not just the first
- `-s` / `--silent` — no output; signal found/not-found via exit status only
- Absolute / relative path arguments bypass `PATH` and are validated directly
- Cross-platform executability rules:
  - **Unix:** a regular file with any execute permission bit (`mode & 0111`)
  - **Windows:** a regular file whose extension is listed in `PATHEXT`
    (default `.COM;.EXE;.BAT;.CMD;.VBS;.JS;.WS;.MSC`), matched case-insensitively
- De-duplication of identical resolved paths under `-a`

## Usage

```
which [-a] [-s] program ...
```

| Flag | Long       | Meaning                                            |
|------|------------|----------------------------------------------------|
| `-a` | `--all`    | List all matches in `PATH`, not just the first.    |
| `-s` | `--silent` | No output; exit status alone signals found-ness.   |

### Exit codes (BSD convention)

| Code | Meaning                                            |
|------|----------------------------------------------------|
| `0`  | Every program name was found (≥1 match each).      |
| `1`  | One or more names were not found.                  |
| `2`  | Usage error (no names supplied, or unknown flag).  |

A per-name diagnostic — `which: no <name> in (<PATH>)` — is written to stderr
for each unfound name (suppressed under `-s`).

## Build & run

```sh
# from the 93-which directory
go build -o which ./cmd/which      # produces ./which (or which.exe on Windows)

./which go
./which -a python3
./which -s git && echo "git is installed"
```

Cross-compile for another OS:

```sh
GOOS=linux  GOARCH=amd64 go build -o which-linux  ./cmd/which
GOOS=darwin GOARCH=arm64 go build -o which-darwin ./cmd/which
```

## Layout

```
93-which/
  go.mod
  cmd/which/main.go            CLI: flag parse, exit-code aggregation, I/O
  cmd/which/main_test.go       end-to-end tests of the run() boundary
  internal/resolver/
    resolver.go                pure resolution logic (no os.Exit, no globals)
    resolver_test.go           hermetic cross-OS tests (injected PATHEXT)
    resolver_unix_test.go      Unix exec-bit tests (build-tagged !windows)
```

## Design notes

The core logic in `internal/resolver` is a pure function:

```go
func Resolve(name string, dirs []string, exts []string, all bool) []string
```

It takes the parsed `PATH` directories and the executable-extension set as
explicit parameters rather than reading the environment or calling `os.Exit`.
This dependency injection makes resolution **hermetically testable**: tests
build a fake filesystem under `t.TempDir()` and pass a synthetic `PATH`/`PATHEXT`,
so nothing depends on the host's real binaries or environment.

`main.go` is the only place that touches process state (`os.Args`, `os.Getenv`,
`os.Stdout/Stderr`, `os.Exit`); its logic is factored into a `run()` function
with injected writers and env strings, so even the CLI is unit-testable without
spawning a subprocess.

The single behavioral fork between platforms is the executability test
(exec-bit vs `PATHEXT`), selected by whether the injected extension slice is
empty. Because the rule is parameterized, the Windows `PATHEXT` path is covered
by tests on every OS, while the Unix exec-bit path is verified by a
build-tagged test file.

## Testing

```sh
go vet ./...
go test ./...
```

All tests are hermetic and require no special host setup.
