# Coding Challenge #90 — SMTP Server (Go)

A small, production-ready **SMTP server** implementing the core of
[RFC 5321](https://www.rfc-editor.org/rfc/rfc5321): a concurrent TCP server that
speaks the line-oriented SMTP command protocol, drives each connection through
the mail-transaction state machine (`HELO` → `MAIL FROM` → `RCPT TO` → `DATA`),
and persists accepted messages to a maildir-style store on disk.

## Features

- TCP listener with a `220` greeting on connect (port `2525` default, `:25` configurable).
- Commands: `HELO`, `EHLO` (multiline capabilities), `MAIL FROM`, `RCPT TO`,
  `DATA`, `RSET`, `NOOP`, `VRFY`, `QUIT`.
- Explicit, typed state machine with RFC 5321 reply codes
  (`220`/`221`/`250`/`252`/`354`/`421`/`500`/`501`/`502`/`503`/`552`/`554`).
- `DATA` terminated by `\r\n.\r\n`, with full dot-stuffing transparency (RFC 5321 §4.5.2).
- Multiple recipients, `RSET`, and out-of-order command handling (`503`).
- One goroutine per connection; atomic, mutex-guarded message store with unique IDs.
- Graceful shutdown on SIGINT/SIGTERM (drains in-flight sessions).
- Idle read/write deadlines per connection.
- Zero third-party dependencies — standard library only.

## Layout

```
90-smtp-server/
├── go.mod
├── cmd/smtpd/main.go            # CLI entry point
└── internal/smtp/
    ├── server.go                # listener + accept loop + graceful shutdown
    ├── session.go               # per-connection loop, dispatch, DATA reader
    ├── parser.go                # line → Command, address extraction
    ├── state.go                 # State enum
    ├── reply.go                 # reply-code constants + formatting
    ├── envelope.go              # Envelope / Transaction types
    ├── store.go                 # MessageStore: Maildir + in-memory impls
    └── *_test.go                # unit + in-process integration tests
```

## Build & test

```bash
go build ./...
go vet ./...
go test ./...
```

> Note: `go test -race` requires a 64-bit cgo toolchain. On a Windows host with
> only a 32-bit GCC the race detector cannot link; plain `go test ./...` runs all
> tests.

## Run

```bash
# Unprivileged challenge port, storing mail under ./maildir
go run ./cmd/smtpd --addr :2525 --maildir ./maildir

# Standard SMTP port (needs privileges)
go run ./cmd/smtpd --addr :25 --maildir /var/mail/cc
```

### Flags

| Flag         | Default      | Description                                        |
|--------------|--------------|----------------------------------------------------|
| `--addr`     | `:2525`      | Listen address `host:port` (`:25` for standard).   |
| `--maildir`  | `./maildir`  | Directory for received messages (created if absent).|
| `--hostname` | OS hostname  | Hostname announced in the greeting / EHLO.         |
| `--timeout`  | `5m`         | Idle read/write timeout per connection.            |

## Try it by hand

```
$ telnet localhost 2525
220 myhost CC SMTP Server ready
HELO tester
250 myhost Hello tester
MAIL FROM:<me@example.com>
250 OK
RCPT TO:<you@example.com>
250 OK
DATA
354 Start mail input; end with <CRLF>.<CRLF>
Subject: Hello

This is the body.
.
250 OK: message accepted for delivery
QUIT
221 myhost closing connection
```

Each accepted message is written to `<maildir>/<timestamp>-<seq>.eml` with
`X-CC-From`, `X-CC-To`, and `X-CC-Received` trace headers, a blank line, then the
client's (dot-unstuffed) message.

## Verify with a real client

Python's standard-library `smtplib`:

```python
import smtplib
with smtplib.SMTP("127.0.0.1", 2525, timeout=5) as s:
    s.helo("tester")
    s.sendmail("from@ex.com", ["to@ex.com"], "Subject: Hi\r\n\r\nHello CC SMTP")
```

The test suite exercises this automatically (`TestIntegrationPythonSMTPLib`,
skipped if Python is absent) alongside Go's `net/smtp` client
(`TestIntegrationNetSMTPClient`).

## Tests

- **`parser_test.go`** — command parsing, address extraction, ESMTP-param tolerance, malformed input.
- **`state_test.go`** — state-machine transitions and reply codes (out-of-order → `503`, bad syntax → `501`, EHLO multiline, reply-code constants).
- **`store_test.go`** — maildir write contents, concurrent saves yield distinct files, in-memory store.
- **`integration_test.go`** — in-process server over loopback: full session, dot-stuffing over the wire, out-of-order, RSET, concurrency, and `net/smtp` + `smtplib` interop.
