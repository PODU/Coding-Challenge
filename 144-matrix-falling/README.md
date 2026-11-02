# Matrix falling text — Coding Challenge #144

`ccmatrix`, falling "digital rain" from The Matrix in the terminal. Every
column gets a trail with a bright white head and a fading coloured tail. Trails
start at random times and fall at different speeds. Written in Go with raw
ANSI escape codes and no terminal library.

Source: https://codingchallenges.substack.com/p/coding-challenge-144-matrix-falling

## Build & test

```sh
go build -o ccmatrix .   # ccmatrix.exe on Windows
go vet ./...
go test ./...
```

Requires Go 1.24+. Standard library only. Runs on Linux, macOS and the BSDs
(termios + SIGWINCH) and on Windows 10+ (console VT mode).

## Usage

```
ccmatrix [-s] [-u DELAY] [-C COLOUR] [-r] [-c] [-b|-B] [-f]

  -s          screensaver mode: any key or mouse click exits
  -u DELAY    delay between updates, 0 (fastest) to 10 (default 4)
  -C COLOUR   trail colour: green (default), red, blue, white, yellow,
              cyan, magenta, black
  -r          rainbow trails
  -c          half-width katakana instead of letters and digits
  -b          some characters bold
  -B          all characters bold
  -f          trail characters flicker as they fall
  -h          show this help
```

Keys while running: `q` (or Ctrl+C) quits, `0`–`9` change the speed at once.
Any other key is ignored, unless `-s` is given.

Flags can be grouped and values attached, as in cmatrix: `ccmatrix -sc -u2`.

```sh
ccmatrix                     # green, default speed
ccmatrix -c -C cyan -u 2     # katakana, cyan, fast
ccmatrix -r -B               # bold rainbow
ccmatrix -s                  # screensaver: any key exits
ccmatrix -u 11               # error, exit 1, nothing drawn
ccmatrix -C purple           # lists supported colours, exit 1
```

For `-c` the terminal font needs half-width katakana (Windows Terminal,
GNOME Terminal, iTerm2 and kitty all work by default) and the terminal must be
using UTF-8.

## Challenge steps

| Step | Feature | Status |
|---|---|---|
| 1 | Raw mode, one falling character, `q` quits, terminal restored | ✅ |
| 2 | White head, green trail, blank behind it | ✅ (trail fades bright → normal → dim) |
| 3 | A trail in every column, random start and speed | ✅ |
| 4 | `-s` screensaver: any key or mouse exits | ✅ |
| 5 | `-u 0..10`, keys `0`–`9`, errors exit non-zero without drawing | ✅ |
| 6 | `-C` colours, `-r` rainbow, heads stay white | ✅ |
| 7 | `-c` katakana, resize larger and smaller | ✅ |
| Extra | `-b`/`-B` bold, `-f` flicker, diff-only redraw for low CPU | ✅ |

## Verification

- `go test ./...`: options, input parsing, the simulation (trail invariants,
  speeds, colours, katakana, resize to sizes down to 0×0), the renderer, and
  the full loop against a fake terminal (quit keys, ignored keys, `-s`, speed
  keys, resize bounds, terminal restore sequence).
- Cross-compiled for Linux, macOS and FreeBSD.
- Run live in a Windows console: `a`, `x`, space, `5` and Up are ignored, `q`
  exits with code 0, and `-s` exits on space.
