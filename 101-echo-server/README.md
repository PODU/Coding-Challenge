# ccecho — RFC 862 Echo Server

A production-ready echo server (Coding Challenge #101) written in Go. It listens
for client connections and echoes back any data it receives until the connection
is terminated, implementing the Echo Protocol
([RFC 862](https://www.rfc-editor.org/rfc/rfc862)) over both **TCP** and **UDP**.

## Features

- **TCP and UDP** transports (RFC 862, canonical port 7).
- **Concurrent client handling** — goroutine-per-connection for TCP; the accept
  loop never blocks on a slow client.
- **Streaming echo** via `io.Copy` — correct partial reads and arbitrarily large
  payloads with constant memory.
- **Configurable** port, bind host, idle timeout, and shutdown drain window.
- **Graceful shutdown** — on SIGINT/SIGTERM the listener stops accepting new
  connections and active connections are drained (bounded by `-drain`).
- **Structured logging** of lifecycle and per-connection events (`log/slog`).
- **Zero third-party dependencies** — standard library only.

## Build

```sh
cd 101-echo-server
go build ./cmd/ccecho      # produces ./ccecho (ccecho.exe on Windows)
```

## Usage

```
ccecho [flags]

Flags:
  -port int        Port to listen on (default 7, the RFC 862 echo port)
  -host string     Host/interface to bind (default "" = all interfaces)
  -udp             Run in UDP mode instead of the default TCP mode
  -idle duration   Per-connection idle/read timeout, 0 = none (default 0)
  -drain duration  Max time to wait for active conns on shutdown (default 10s)
```

> **Note:** Binding port 7 requires elevated privileges on most systems. For
> unprivileged local runs and tests, use a high port such as `-port 7000`.

### Examples

```sh
ccecho                  # TCP echo on 0.0.0.0:7 (needs privileges)
ccecho -port 7000       # TCP echo on :7000 (unprivileged)
ccecho -udp -port 7000  # UDP echo on :7000
```

### Smoke test

In one terminal:

```sh
ccecho -port 7000
```

In another (TCP):

```sh
nc 127.0.0.1 7000       # type lines; they are echoed back
```

Or UDP:

```sh
ccecho -udp -port 7000
nc -u 127.0.0.1 7000
```

Stop the server with `Ctrl+C`; it drains active connections and exits cleanly.

## Design

In short:

- `cmd/ccecho/main.go` — flag parsing, signal-driven context, `Server.Run`.
- `server.go` — `Config`, `Server`, `serveTCP`, `serveUDP`, `handleConn`.
- `server_test.go` — in-process loopback tests.

**Concurrency:** TCP uses goroutine-per-connection with a `sync.WaitGroup` so
shutdown can wait for in-flight handlers to drain. UDP is connectionless and
handled by a single read loop (`ReadFrom` → `WriteTo` the sender).

**Graceful shutdown:** context cancellation closes the listener (unblocking
`Accept`/`ReadFrom`), then the server waits up to `-drain` for active
connections before exiting; an optional `-idle` deadline bounds stalled peers.

## Tests

All tests run in-process over the loopback interface (server bound to
`127.0.0.1:0`), so they are deterministic and CI-friendly.

```sh
go test ./... -race
```

Coverage includes: single + multi-write TCP echo, multi-megabyte payloads,
EOF/half-close, 100 concurrent clients, UDP datagram echo (single, multiple
senders, empty datagram), graceful drain on cancel, shutdown with no clients,
and invalid-port / bind-failure handling.
