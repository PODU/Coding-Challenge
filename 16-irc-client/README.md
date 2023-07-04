# ircclient — Coding Challenge #16: Build Your Own IRC Client

A terminal IRC client in Go implementing a usable subset of the IRC client
protocol (RFC 1459 / RFC 2812): TCP/TLS connect, `NICK`/`USER` registration,
`PING`→`PONG` keep-alive, `JOIN`/`PART`, nickname changes, `PRIVMSG` send and
receive (channel, private, and CTCP `ACTION`), and a clean `QUIT`. It ships with
a full-screen tcell TUI and a `-plain` headless line mode.

## Layout

```
16-irc-client/
  cmd/ircclient/main.go      flag parsing, wiring, lifecycle, signal trap
  internal/parser/           pure protocol parser + encoder (fully unit tested)
  internal/conn/             TCP/TLS transport: CRLF framing, serialised writes
  internal/client/           session state machine + command/event dispatch
  internal/ui/               tcell TUI (tcell.go) and headless mode (plain.go)
```

The `parser` package is pure and dependency-free (no network, no UI), so it is
exhaustively unit testable. `client` depends only on `parser` plus a one-method
`Sender` interface, so its state machine is tested against an in-memory fake.

## Build

```sh
go build ./cmd/ircclient        # produces ./ircclient(.exe)
```

Go 1.24+ is required. The only external dependency is
`github.com/gdamore/tcell/v2` (TUI); the protocol/transport/state-machine
packages use the standard library only.

## Run against a real server

The reference server is Libera.Chat (the modern successor to Freenode):

```sh
# Full TUI, plaintext port 6667
./ircclient -server irc.libera.chat -port 6667 -nick YourNick -channel '#cc-test'

# TLS (port 6697 is implied by -tls when -port is not given)
./ircclient -server irc.libera.chat -tls -nick YourNick

# Headless line mode (no TTY needed; ideal for scripting)
./ircclient -plain -nick YourNick -channel '#cc-test'

# Show raw protocol lines including PING/PONG
./ircclient -debug -nick YourNick
```

### Flags

| Flag       | Default                     | Meaning                                  |
|------------|-----------------------------|------------------------------------------|
| `-server`  | `irc.libera.chat`           | IRC server host                          |
| `-port`    | `6667`                      | server port                              |
| `-tls`     | off                         | use TLS (implies `-port 6697` if unset)  |
| `-nick`    | `CCClient`                  | nickname                                 |
| `-user`    | `guest`                     | username for the `USER` command          |
| `-real`    | `Coding Challenges Client`  | real name                                |
| `-channel` | (none)                      | channel to auto-join after registration  |
| `-plain`   | off                         | headless line mode instead of the TUI    |
| `-debug`   | off                         | show raw protocol lines incl. PING/PONG  |

### In-session commands

Type these on the input line:

| Input                  | Action                                          |
|------------------------|-------------------------------------------------|
| `/join #chan`          | join a channel and make it active               |
| `/part [#chan]`        | leave the active (or named) channel             |
| `/nick NewNick`        | change nickname                                 |
| `/msg <target> <text>` | private or channel message                      |
| `/me <action>`         | CTCP `ACTION` to the active channel             |
| `/raw <line>`          | send a raw protocol line (debugging)            |
| `/quit [message]`      | disconnect and exit cleanly                     |
| `<plain text>`         | message the active channel                      |
| `/help`                | list commands locally                           |

In the TUI: `PgUp`/`PgDn` scroll the backlog, `Up`/`Down` walk input history,
`Home`/`End`/`Left`/`Right` move the cursor, `Ctrl-U` clears the line, and
`Ctrl-C` quits cleanly (sends `QUIT` first).

## Tests

```sh
go test ./...        # parser unit tests + client state machine + mock-server integration
go vet ./...
gofmt -l .           # should print nothing
```

What is covered automatically (no network, runs in CI):

- **`internal/parser`** — table-driven `Parse` tests over real IRC samples:
  numeric replies, `NOTICE`, `JOIN`/`PART` with and without a leading `:` on the
  channel, `NICK` changes, `PRIVMSG` to a channel and to a user, CTCP `ACTION`,
  `PING`, `QUIT` with a reason, IRCv3 message tags, prefix-less lines, trailing
  params containing spaces and colons, and malformed lines (error, no panic).
  Plus `Encode`/round-trip tests asserting exact wire bytes including CRLF.
- **`internal/client`** — registration sends exactly `NICK`+`USER`; auto-`PONG`
  on `PING` (including before registration); `001` sets the registered flag and
  server-assigned nick; self-`JOIN` sets the active channel; self-`NICK` updates
  state; `PRIVMSG`/PM/`ACTION`/`QUIT`/numeric-error rendering; every slash
  command maps to the correct wire line; unknown commands produce an error, not
  a send; concurrent sends are not garbled.
- **`internal/conn`** — CRLF framing and 512-byte truncation, coalesced/split
  line reassembly, write serialisation under concurrency, and an in-process
  **mock-server integration test** that scripts the server side (accept →
  expect `NICK`/`USER` → send `001` → send `PING` → assert matching `PONG` →
  push a `PRIVMSG` → assert it surfaces → send `ERROR` → assert clean shutdown),
  wired through the real `conn` + `client`.

### Manual-test-only (the TUI)

The tcell front-end in `internal/ui/tcell.go` is **manual-test-only** — it
requires a real terminal and is not exercised by `go test`. QA it by hand:
scrollback with `PgUp`/`PgDn`, terminal resize, input history, status-bar
nick/channel correctness, long-line wrapping, `Ctrl-C` clean exit, and that
rapid incoming traffic never blocks typing. For automated end-to-end checks,
use `-plain`, which renders to stdout and reads stdin and so can be scripted.

> The race detector (`go test -race`) needs a working cgo/C toolchain. On a
> stock Windows install without a 64-bit gcc it reports
> "sorry, unimplemented: 64-bit mode not compiled in"; the plain `go test ./...`
> run above does not need cgo and is the authoritative check.

## Manual end-to-end smoke test

```sh
# Terminal 1
./ircclient -plain -nick CCtester1 -channel '#cc-test'
# then type: hello from one

# Terminal 2 (any IRC client) joins #cc-test and replies; both sessions
# should show each other's messages, joins, nick changes, and quits.
```
