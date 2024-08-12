# ccmc — Memcached CLI (Coding Challenge #70)

A small, dependency-free command-line client that speaks the **Memcached text
protocol** over TCP. It runs one-shot commands (`set`, `get`, `add`, `replace`,
`append`, `prepend`, plus `cas`, `delete`, `incr`, `decr`, `stats`) and, when
invoked with no command, drops into an interactive REPL.

It talks to any real `memcached` daemon and to the server from
[Coding Challenge #17](https://codingchallenges.substack.com/p/coding-challenge-17),
since both implement the same protocol.

## Build

```sh
go build -o ccmc ./cmd/ccmc      # produces a single static binary
go install ./cmd/ccmc            # or install onto $GOPATH/bin
```

Requires Go 1.24+. Zero third-party dependencies (standard library only).

## Usage

```
ccmc [global flags] [command [args...]]

Global flags:
  -host string   memcached host (default "localhost")
  -p int         memcached port (default 11211)
  -t duration    dial/read timeout (default 5s)
  -flags uint    flags value for storage commands (default 0)
  -exptime int   expiration seconds for storage commands (default 0)
  -noreply       send storage/delete with noreply (no server ack)
  -q             quiet: print only the raw value(s) for get
  -h, --help, -?, --?   show usage and exit
```

Global flags must appear **before** the command.

### Commands

| Command | Form | Behaviour |
|---------|------|-----------|
| `set` | `set <key> [value]` | Store data, always succeeds |
| `add` | `add <key> [value]` | Store only if the key is **absent** |
| `replace` | `replace <key> [value]` | Store only if the key is **present** |
| `append` | `append <key> [value]` | Append to an existing value |
| `prepend` | `prepend <key> [value]` | Prepend to an existing value |
| `get` | `get <key> [key...]` | Retrieve one or more keys |
| `gets` | `gets <key> [key...]` | Retrieve with the CAS unique |
| `cas` | `cas <key> <casid> [value]` | Check-and-set |
| `delete` | `delete <key>` | Remove a key |
| `incr` | `incr <key> <delta>` | Increment a numeric value |
| `decr` | `decr <key> <delta>` | Decrement a numeric value |
| `stats` | `stats [arg]` | Server statistics |

For storage commands the value is taken from the trailing arguments (joined with
spaces) or, if none are given, read from **stdin** — handy for large or binary
values. An empty value is valid.

### Examples

```sh
% ccmc -host server.com -p 12121 set testkey testvalue
STORED

% ccmc get testkey
testkey: testvalue

% printf 'blob' | ccmc set bigkey
STORED

% ccmc get a b c          # multi-key; misses reported on stderr
a: AA
c: CC
b: (not found)

% ccmc stats
curr_items  7
pid         1234
uptime      4200
```

### Interactive REPL

Run with no command to open a persistent session against one connection:

```
% ccmc -host localhost -p 11211
ccmc interactive mode. Type 'help' for commands, 'quit' to exit.
mc> set foo bar
STORED
mc> get foo
foo: bar
mc> quit
```

A bad line reports the error but keeps the session alive; the REPL reconnects
automatically if the connection drops.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | success / stored / value found |
| `1` | logical miss: not found / `NOT_STORED` / `EXISTS` |
| `2` | usage error |
| `3` | connection or protocol error |

## Architecture

```
cmd/ccmc/main.go        Entry point: flag parsing, one-shot vs REPL selection
internal/protocol/      Text-protocol codec (pure, no I/O) — the TDD core
    request.go          Encode storage/retrieval/delete/incr/stats requests
    response.go         Parse server replies (exact byte framing) into typed results
    errors.go           ERROR / CLIENT_ERROR / SERVER_ERROR typed errors
internal/client/        TCP client: dial, deadlines, send request, read framed reply
    client.go           Conn wrapper over bufio with per-op deadlines
    ops.go              Set/Get/Gets/Store/Delete/IncrDecr/Stats
internal/cli/           Command parsing + reply formatting (presentation)
    parse.go            argv -> validated Command
    format.go           typed results -> readable output (goldens)
    execute.go          dispatch + exit codes
    repl.go             interactive loop with reconnect
    help.go             usage text
```

The three layers are independently testable: the codec is pure (unit-tested with
golden byte slices), the client is exercised against an in-process fake server,
and the formatter is golden-tested.

Value data is always read by its declared `<bytes>` length and the trailing
CRLF consumed separately, so values containing `\r\n` or NUL bytes round-trip
correctly. Every read and write carries a deadline, so a dead or hostile server
can never hang the CLI.

## Tests

```sh
go test ./...              # all unit + integration tests (hermetic, no network)
go test -cover ./internal/protocol/   # codec coverage (>90%)
```

- **`internal/protocol`** — table-driven encode/decode tests for every command
  and reply, multi-key get, embedded-CRLF/binary framing, truncation, and the
  three error families. ~94% statement coverage.
- **`internal/client`** — an in-process fake memcached server (`net.Listener` on
  `127.0.0.1:0` speaking the text protocol) backs round-trip tests:
  set→STORED, get round-trip and miss, add-twice, replace-missing,
  append/prepend, multi-key mixed hits/misses, delete-then-miss, incr/decr incl.
  `NOT_FOUND` and non-numeric `CLIENT_ERROR`, cas (stale/missing/ok), stats,
  noreply, binary values, plus connection-refused and read-timeout paths.
- **`internal/cli`** — `ParseCommand` for every command and error case, golden
  output for hits/misses/stats, and end-to-end `Execute` tests asserting stdout,
  stderr, and exit codes.

### Live interop (manual)

```sh
docker run --rm -p 11211:11211 memcached:alpine
go build -o ccmc ./cmd/ccmc
./ccmc set foo bar && ./ccmc get foo && ./ccmc stats
```

## Notes

- `go test -race` requires a 64-bit cgo toolchain; on a machine with only a
  32-bit MinGW gcc the race detector is unavailable, but the standard
  `go test ./...` run is fully hermetic and passes.
