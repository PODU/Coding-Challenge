# nats-server — Coding Challenge #25

A minimal, production-quality [NATS](https://nats.io) message broker in Go,
implementing the subset of the NATS client protocol from
[John Crickett's Coding Challenge #25](https://codingchallenges.substack.com/p/coding-challenge-25-build-your-own).

Clients open a TCP connection, receive an `INFO` greeting, and exchange
newline-terminated protocol frames (`CONNECT`, `PING`/`PONG`, `SUB`, `PUB`,
`UNSUB`). The server routes each published message to every connection currently
subscribed to that subject.

## Layout

```
cmd/nats-server/      main: flags, signal handling, wiring
internal/protocol/    pure parser + frame encoders (no I/O, TDD core)
internal/router/      subject -> subscription index, concurrency-safe fan-out
internal/server/      TCP listener, per-connection client read loop
```

The `protocol` package is I/O-free (`Parse(line) -> Command`, plus encoders for
`INFO`/`+OK`/`PONG`/`MSG`/`-ERR`), so it is unit-testable in isolation. The
`router` package is decoupled from the network via a `Subscriber` interface, so
there is no import cycle and the routing logic is testable without sockets.

## Build & Run

```sh
go build -o nats-server ./cmd/nats-server
./nats-server -port 4222            # flags: -addr (default 0.0.0.0), -port (default 4222)
go test ./...                        # all tests
go test ./... -race                  # data-race check (needs a 64-bit C toolchain)
```

The server logs the listen address and connection lifecycle to stderr and shuts
down gracefully on `SIGINT`/`SIGTERM`.

## Supported protocol

| Direction | Frame | Notes |
|-----------|-------|-------|
| S→C | `INFO {json}\r\n` | Sent once on connect (`server_id`, `version`, `max_payload`, `proto`, …). |
| C→S | `CONNECT {json}\r\n` | `verbose` honored (default `true`); bare/non-JSON tolerated. |
| C→S | `PING\r\n` → S→C `PONG\r\n` | |
| C→S | `SUB <subject> [queue] <sid>\r\n` | Queue token parsed (no queue-group load balancing yet — all matching subs receive). |
| C→S | `UNSUB <sid> [max_msgs]\r\n` | Removes now, or auto-removes after `max_msgs` more deliveries. |
| C→S | `PUB <subject> [reply] <#bytes>\r\n<payload>\r\n` | Length-prefixed, binary-safe payload. |
| S→C | `MSG <subject> <sid> [reply] <#bytes>\r\n<payload>\r\n` | One per matching subscription. |
| S→C | `+OK\r\n` | When the client negotiated `verbose`. |
| S→C | `-ERR '<reason>'\r\n` | On protocol error (e.g. `Unknown Protocol Operation`). |

Subject matching is **exact-match**, as specified by the challenge. Token
wildcards (`*` / `>`) are a noted stretch goal and are not implemented.

## Concurrency model

- One goroutine per connection for reads.
- A single `RWMutex`-guarded registry: read-locked on publish, write-locked on
  sub/unsub/disconnect. Publish snapshots the matching subscription set under the
  read lock and writes out afterward, so a slow consumer never stalls other
  connections' sub/unsub.
- Each client owns a write mutex with a write deadline so concurrent publishers
  cannot interleave bytes and a dead consumer cannot wedge a publisher.
- On disconnect the read error tears down the goroutine and removes all of that
  client's subscriptions.

## Compatibility with the real `nats` CLI

The server speaks enough of the protocol to interoperate with the official
[`nats` CLI](https://github.com/nats-io/natscli) for the supported subset.
Manual smoke test (requires the external `nats` binary, not part of CI):

```sh
./nats-server -port 4222 &

# terminal A
nats -s nats://127.0.0.1:4222 sub foo

# terminal B
nats -s nats://127.0.0.1:4222 pub foo "Hello John!"
```

The subscriber prints the delivered message. Throughput can be exercised with:

```sh
nats -s nats://127.0.0.1:4222 bench foo --pub 1 --sub 1 --msgs 100000
```

Notes / limitations vs. real NATS: no wildcard subjects, no queue-group load
balancing, no auth/TLS, no JetStream, no request/reply auto-`INBOX` handling
beyond passing the `reply` token through. The `reply` subject is propagated in
`PUB`→`MSG`, so basic core request/reply still flows.

## Tests

- `internal/protocol` — parser round-trips for every verb, case-insensitivity,
  CONNECT option defaults, malformed-input errors, and exact encoder byte output
  (including the `Hello John!` example).
- `internal/router` — exact-subject routing, isolation, unsubscribe,
  `max_msgs` auto-unsub, and a concurrent multi-publisher fan-out.
- `internal/server` — in-process loopback integration: INFO greeting, `+OK`,
  `PING`/`PONG`, SUB+PUB→MSG delivery, multi-subscriber + isolation, UNSUB stops
  delivery (connection stays open), `UNSUB sid max_msgs`, binary-safe payloads
  containing embedded CRLFs, unknown-op `-ERR`, and a concurrent fan-out.
```
