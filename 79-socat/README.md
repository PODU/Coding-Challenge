# ccsocat — a `socat` in Go

Coding Challenge #79. `ccsocat` opens **two address endpoints** and relays bytes
between them, with correct half-close / EOF semantics. Every endpoint type
reduces to `io.ReadWriteCloser`, so any endpoint can be wired to any other.

## Build

```sh
go build -o ccsocat ./cmd/ccsocat
```

Pure standard library, zero third-party dependencies. Builds on Windows, Linux,
and macOS.

## Usage

```
ccsocat [options] <address1> <address2>

Options:
  -u            unidirectional: transfer address1 -> address2 only
  -U            reverse unidirectional: address2 -> address1 only
  -d            increase verbosity (repeatable) — diagnostics to stderr
  -h, --help    show this help
```

### Address kinds

| Spec | Meaning |
|------|---------|
| `-` / `STDIO` / `STDIN!!STDOUT` | stdin (read) + stdout (write) |
| `TCP[4|6]:host:port` | TCP connect (client) |
| `TCP[4|6]-LISTEN:port[,reuseaddr][,fork]` | TCP listening server |
| `UDP[4|6]:host:port` | UDP "connected" client |
| `UDP[4|6]-LISTEN:port` | UDP server (locks onto first peer) |
| `FILE:path[,create][,append][,trunc]` | file (`OPEN:` is an alias; `creat` == `create`) |
| `EXEC:'command args'` | spawn a process, relay its stdin/stdout |

Per-address options are comma-separated; positional params are colon-separated.
Unknown types and unknown options are rejected with a clear error.

Exit codes: `0` clean EOF/close · `1` runtime/IO error · `2` usage/parse error.

### The four challenge steps

```sh
# 1. TCP client wired to your terminal — type a raw HTTP request
ccsocat - TCP4:eu.httpbin.org:80

# 2. Read stdin, write to a file (Ctrl+D to end)
ccsocat -u STDIO FILE:test.txt,create

# 3. Forking TCP server appending every connection to a log
ccsocat -u TCP4-LISTEN:3333,reuseaddr,fork OPEN:./test.log,creat,append

# 4. Tail a file and forward it over TCP
ccsocat -u EXEC:'tail -f test.txt' TCP4:localhost:3333
```

## Architecture

```
argv ─► CLI parser ─► AddressSpec{Kind, Params, Options}   (internal/addr)
                              │  one per side
                              ▼
                     Endpoint factory ─► io.ReadWriteCloser  (internal/endpoint)
                              │  left, right
                              ▼
                     Relay core: io.Copy per direction       (internal/relay)
                     half-close on each EOF; close both
```

- `internal/addr` — pure address-spec parser (`Parse`), fully unit-tested.
- `internal/endpoint` — factory: `Open` (connect/file/exec/stdio) and `Listen`
  (TCP/UDP servers). `SO_REUSEADDR` is set via per-platform `Control` hooks.
- `internal/relay` — the bidirectional pump, written against
  `io.ReadWriteCloser` so it is testable with in-memory streams.

Half-close: when one direction reaches EOF the relay calls `CloseWrite` on the
downstream endpoint (when it implements `CloseRead`/`CloseWrite`, e.g.
`*net.TCPConn`), sending FIN while the reverse direction keeps draining — this
is what makes `- TCP4:host:80` work for request/response HTTP.

## EXEC: Windows vs Unix

`EXEC` splits the command on whitespace and runs the program **directly, without
a shell** (like real `socat`'s `EXEC`, not `SYSTEM`). Implications:

- **Unix:** `EXEC:'tail -f test.txt'` runs `tail` with args `-f test.txt`.
  Shell features (pipes, globs, redirection, quotes-with-spaces) are *not*
  interpreted. To use them, invoke a shell explicitly:
  `EXEC:'sh -c "tail -f test.txt | grep ERROR"'`.
- **Windows:** there is no `tail`/`cat`. Use built-ins via `cmd`, e.g.
  `EXEC:'cmd /c type test.txt'`, or any program on `PATH`. Shell built-ins
  (`echo`, `type`, `dir`) require the `cmd /c` prefix.

The child's stderr is forwarded to `ccsocat`'s stderr; closing the endpoint
closes the child's stdin (EOF) and reaps the process via `Wait`.

## Tests

```sh
go test ./...          # all tests
go test -short ./...   # skips the EXEC process-spawn test
```

Coverage: address-spec parse tables (every kind + options + error cases),
bidirectional relay over in-process `net.Pipe` loopback (TCP↔TCP and STDIO↔TCP
analogues with injected streams), unidirectional `-u`, half-close/EOF
propagation, FILE read/append on a temp file, an EXEC echo round-trip (the test
binary re-execs itself as a portable `cat`), and a TCP-LISTEN + loopback connect
integration test on an ephemeral port.

## Caveats / out of scope

- **Paths with colons.** Because positional params are colon-separated, a
  Windows drive-letter path (`FILE:C:\tmp\x.txt`) is mis-parsed. Use relative or
  forward-slash POSIX-style paths, or run from the target directory. Unix paths
  are unaffected.
- **`fork` is TCP-only.** `UDP-LISTEN` is connectionless and locks onto the
  first peer (single logical connection); it does not fork.
- **Non-half-closable bidirectional pairs.** If neither endpoint supports
  half-close and a peer blocks waiting for FIN, the reverse direction can hang
  until the process is signaled — TCP, EXEC, STDIO, and files all avoid this.
- UNIX-domain sockets, SSL/TLS, pty/serial, and proxy CONNECT are out of scope
  but slot into the same factory without touching the parser or relay core.
