# Realtime Chat (Coding Challenge #24)

A concurrent, multi-user chat server in Go. Many clients connect at once, each
with a username; every message is broadcast to all other members of the
client's room (everyone *except* the sender). Ships as a single static binary
that also serves an embedded browser client.

## Features

- **WebSocket transport** with a built-in browser web client (served by the
  same binary, no CDN).
- **Rooms / channels** — broadcasts are scoped to a room (default `general`).
- **Presence** — join/leave system notices and a live member list.
- **Channel-based broadcast hub** — a single owner goroutine holds all shared
  state (no membership locks); one read pump + one write pump per connection.
- **Bounded per-client send buffers** — a slow or dead client is dropped and
  disconnected instead of blocking the hub or its peers.
- **Graceful shutdown** on `SIGINT`/`SIGTERM`.
- **Username de-duplication** per room (`bob`, `bob-2`, ...).
- **Optional raw-TCP adapter** (`-tcp :7007`) preserving the original
  `telnet`/CLI workflow.

## Requirements

- Go 1.24+
- One dependency: `github.com/gorilla/websocket` (fetched by `go` on first
  build).

## Run

```sh
cd 24-realtime-chat
go run ./cmd/server                 # HTTP/WebSocket on :8080
go run ./cmd/server -tcp :7007      # also enable the raw-TCP adapter
go run ./cmd/server -http :9000     # custom HTTP address
```

Then **open a browser** at <http://localhost:8080/>, pick a name and a room,
and start chatting. Open a second tab (or another machine on the LAN) to see
broadcast and presence in action.

### CLI client

```sh
go run ./cmd/client -addr ws://localhost:8080/ws -user alice -room general
# omit -user to be prompted; type /quit or exit to leave
go run ./cmd/client -reconnect      # auto-reconnect with backoff
```

### telnet (raw-TCP adapter)

```sh
# server must be started with -tcp :7007
telnet localhost 7007
# first line you type is your username; each later line is a message
```

## Build a single binary

```sh
go build -o chat-server ./cmd/server   # web client is embedded via embed.FS
./chat-server
```

## Test

```sh
go vet ./...
go test ./... -race -count=1
```

The suite is fully in-process — no external services, brokers, or databases:

- **Hub unit tests** (`internal/chat`): join/leave, broadcast-excludes-sender,
  room routing/isolation, slow-client buffer drop, duplicate-username
  suffixing, and a fan-out-under-load race test.
- **WebSocket integration tests** (`internal/transport`): real HTTP upgrade and
  frames over loopback via `httptest` + the gorilla dialer — broadcast,
  presence, quit, oversize-frame close, and graceful shutdown.
- **Raw-TCP adapter test**: two `net.Dial` clients, `Name: message` fidelity.

## Wire protocol

WebSocket text frames carry JSON.

Client → server: `{"type":"message","text":"..."}`, `{"type":"join","room":"..."}`,
`{"type":"quit"}`.

Server → client: `message`, `system`, `presence`, `join` (effective-name ack),
`error`.

## Layout

```
cmd/server     server: hub + HTTP/WS + optional TCP + signal handling
cmd/client     CLI chat client
internal/chat  Hub, Client, message types, hub unit tests
internal/transport  WebSocket handler + pumps, raw-TCP adapter, tests
web            embedded single-page browser client (index.html, app.js, style.css)
```

> Note: the socket read/write pumps live in `internal/transport` (next to the
> WebSocket dependency) rather than in `internal/chat` as the design sketch
> suggested, keeping the `chat` package pure-stdlib and trivially testable.
