# cctmux — build your own tmux

A terminal multiplexer in Go: client–server, named sessions, split panes,
stacked windows, detach/reattach, a status bar, copy mode, and a configurable
prefix key. [Coding Challenge #127](https://codingchallenges.substack.com/p/coding-challenge-127-tmux).

Zero dependencies (`go.mod` has no requires). The engine is a **pure, fully
tested core**; the interactive server/client runs on **Linux** (real PTYs + Unix
sockets). Every OS can run the test suite and the `demo`.

## Build

```sh
cd 127-tmux
go build -o cctmux ./cmd/cctmux
```

## Use (Linux)

```sh
./cctmux new-session -s work     # create "work" and attach
./cctmux ls                      # list sessions
./cctmux attach -t work          # reattach after detaching
./cctmux kill-session -t work
./cctmux -p C-a new-session -s x # custom prefix (Ctrl+a)
```

Inside a session, the **prefix** is `Ctrl+b` (configurable):

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `d` | detach | `c` | new window |
| `n` / `p` | next / prev window | `0`–`9` | select window |
| `"` | split top/bottom | `%` | split left/right |
| arrows | move between panes | Ctrl+arrows | resize pane |
| `z` | zoom pane | `x` | kill pane |
| `[` | copy mode (`q` to exit) | `:` | command prompt |

Pressing the prefix twice sends a literal prefix keystroke to the shell.

Config file `~/.cctmux.conf` (or `-f <path>`):

```
set prefix C-a
```

## Demo (any OS, no shell needed)

```sh
go run ./cmd/cctmux demo
```

Renders sample frames — a three-pane split layout and window switching — using
mock shells, so it works on Windows and macOS too:

```
== Three panes (vertical + horizontal split) ==
|echo hello from cctmux        │ls -la                       |
|                              │─────────────────────────────|
|                              │top                          |
|[demo]                   0:bash*                    09:36:54|
```

## Test

```sh
go test ./...                 # hermetic; passes on any OS
go test -cover ./...
GOOS=linux go build ./...     # cross-compile-check the Linux I/O layer
```

## Layout

```
cmd/cctmux/       CLI, config parsing, demo, integration tests
internal/
  layout/   pane split-tree: geometry, navigate, resize, zoom
  screen/   cell-grid terminal emulator + ANSI subset + scrollback
  session/  Manager → Session → Window → Pane model
  render/   composite panes + borders + status bar; emit escape codes
  protocol/ length-prefixed client↔server framing
  command/  tmux-verb parser/dispatcher
  input/    prefix-key state machine
  pty/      PTY interface, echo mock, real Linux PTY (/dev/ptmx)
  server/   daemon: socket, attach loop, pane readers, clock   (linux)
  client/   raw mode, SIGWINCH, auto-start server              (linux)
```

