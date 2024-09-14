# Coding Challenge #73 — Text Editor (Go)

A terminal raw-mode modal text editor in the spirit of `kilo`/`vim`. It opens a
file, lets you move the cursor, switch between control and insert modes, edit
text (insert/delete characters, split/join lines), scroll documents larger than
the screen, search, and save — with a status bar showing file metadata and mode.

The defining design constraint: the **editing core is pure Go with no terminal
dependencies**, so it is fully unit-tested headlessly. The terminal layer (raw
mode, key decoding, ANSI rendering) is a thin, swappable shell around that core.

## Layout

```
cmd/editor/main.go     wiring + run loop (impure)
editor/                pure editor core: cursor/scroll/modes/search/render
buffer/                pure document model: rows, tab expansion, load/save
term/                  terminal layer
  key.go               pure Key type + ReadKey escape-sequence decoder
  terminal.go          raw mode (golang.org/x/term) + ANSI renderer (impure)
  vt_windows.go        enables ANSI on the Windows console (build-tagged)
  vt_other.go          no-op on Unix (build-tagged)
```

- `buffer` and `editor` have **no syscall dependencies** and are where the test
  suite lives (`buffer/buffer_test.go`, `editor/editor_test.go`). The key decoder
  is also pure and tested (`term/key_test.go`).
- Only `term/terminal.go` (raw mode, size, frame writing) and `cmd/editor` touch
  the real terminal.

## Build & run

```
go build -o editor ./cmd/editor
./editor path/to/file.txt      # opens (or creates-on-save) the file
./editor                       # starts empty; :w needs a name (see caveat)
```

## Test

```
go test ./...        # entire pure core, headless
go vet ./...
gofmt -l .           # should print nothing
```

## Keybindings

| Mode    | Key                       | Action                                |
|---------|---------------------------|---------------------------------------|
| Control | `h` `j` `k` `l` / arrows  | move left / down / up / right         |
| Control | `0` `$` / `Home` `End`    | start / end of line                   |
| Control | `PageUp` `PageDown`       | scroll one screen                     |
| Control | `i` / `a`                 | enter insert mode (at / after cursor) |
| Control | `x` / `Del`               | delete char under cursor              |
| Control | `/` or `Ctrl-F`           | search (Enter run, Esc cancel)        |
| Control | `n` / `N`                 | next / previous match                 |
| Control | `:`                       | enter command mode                    |
| Insert  | any printable rune        | insert character                      |
| Insert  | `Enter`                   | split line / new line                 |
| Insert  | `Backspace`               | delete left (joins lines at col 0)    |
| Insert  | `Esc`                     | return to control mode                |
| Command | `:w` `:wq` `:q` `:q!`     | save / save+quit / quit / force quit  |

`:q` is refused while there are unsaved changes; use `:w` then `:q`, or `:q!`.

## Windows console handling

The editor targets Windows 11 as well as Linux/macOS from one codebase:

- **Raw mode** is entered via `golang.org/x/term.MakeRaw`, which on Windows sets
  the console input mode (disabling line input, echo, and special-key processing
  for Ctrl-C/Ctrl-S/etc.) and on Unix uses termios. `term.Restore` puts the
  console back exactly as it was; it is called from a `defer` so the terminal is
  restored even on a panic.
- **ANSI output.** The renderer emits VT100/xterm escape sequences. Modern
  Windows terminals (Windows Terminal, and `conhost` on Windows 10+) support
  these, but the flag must be turned on explicitly: `vt_windows.go` enables
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING` on the stdout handle on startup and
  restores the previous mode on exit. `vt_other.go` is a no-op on Unix. The two
  files are selected by `//go:build` tags so each platform compiles only its own.
- **Window size** is read each frame via `term.GetSize`, so resizing the console
  reflows the layout on the next keypress (falls back to 24×80 if size is
  unavailable). Use Windows Terminal or a recent console for best results; very
  old `conhost` without VT support will show raw escape codes.
- Line endings: files are loaded splitting on `\n` with a trailing `\r` stripped
  (CRLF tolerant) and saved with `\n`. Raw bytes are otherwise preserved.

## Caveats

- **Save-as prompt not bound.** Starting with no filename and running `:w`
  reports "No filename" rather than prompting. The save machinery
  (`Editor.Save`, atomic temp-file + rename) is complete and tested; only the
  interactive name prompt is unbound. Pass a filename on the command line.
- **`go test -race`** requires a 64-bit C toolchain (cgo). The race detector was
  not run in the build environment because only a 32-bit MinGW gcc was present;
  the non-race suite passes. On a machine with 64-bit gcc/clang, `go test -race
  ./...` works against the pure core.
- The renderer assumes a VT-capable terminal (see Windows note above).
- Editing is O(line length) per keystroke (a simple `[]Row` model, no rope),
  which is ample for an interactive editor at this scope.
