# redis-cli (Coding Challenge #37)

A from-scratch clone of `redis-cli` in Go. It connects to a Redis server over
TCP, speaks the **RESP** wire protocol, and lets you send commands and read
replies — as an interactive REPL, a one-shot command, or a pipe/batch reader on
stdin. Replies are formatted to match the real `redis-cli`.

No third-party dependencies — only the Go standard library.

## Build

```sh
go build -o redis-cli .
# Windows: go build -o redis-cli.exe .
```

## Usage

```
redis-cli [-h <host>] [-p <port>] [COMMAND [ARG ...]]
```

- `-h <host>` — server hostname/IP (default `127.0.0.1`).
- `-p <port>` — server TCP port (default `6379`).

### Modes

**Interactive REPL** — no trailing command and a TTY stdin:

```
$ redis-cli
127.0.0.1:6379> PING
PONG
127.0.0.1:6379> SET name "John Doe"
OK
127.0.0.1:6379> GET name
"John Doe"
127.0.0.1:6379> quit
```

- The prompt shows the connection target (`host:port>`).
- `quit` / `exit` leave the client **locally** — they are never sent to the
  server. EOF (Ctrl-D) also exits cleanly.
- Server errors are shown but the session continues.

**One-shot** — a command supplied on argv (mirrors `redis-cli PING`):

```
$ redis-cli PING
PONG
$ redis-cli -p 6380 GET name
"John Doe"
```

**Pipe / batch** — commands streamed on stdin (file redirect or a pipe).
Commands run sequentially and execution **stops on the first error** (exit
code 1):

```
$ redis-cli < commands.txt
$ printf 'SET a 1\nGET a\n' | redis-cli
OK
"1"
```

### Help system

Backed by an embedded command-doc table; needs no server connection:

```
redis-cli> help            # overview + command groups
redis-cli> help GET        # detail for one command
redis-cli> help @string    # every command in a group
```

`help` also works as a one-shot: `redis-cli help @list`.

### Quoting

Arguments may be quoted so values with spaces survive:

```
SET k "two words"
SET k 'two words'
SET k "tab\tand\nnewline"   # C-style escapes inside double quotes
```

An unterminated quote is reported as a parse error and nothing is sent.

## Reply formatting

| Reply           | Display              |
| --------------- | -------------------- |
| Simple string   | `PONG` (bare)        |
| Bulk string     | `"value"` (quoted)   |
| Integer         | `(integer) 42`       |
| Error           | `(error) ERR ...`    |
| Null bulk/array | `(nil)`              |
| Empty array     | `(empty array)`      |
| Array           | numbered, indented when nested |

```
127.0.0.1:6379> LRANGE k 0 -1
1) "a"
2) 1) "nested"
   2) (integer) 7
```

## Running against a real server

Works against any RESP server:

- A real `redis-server` (default `127.0.0.1:6379`).
- The Coding Challenge #8 ("Build your own Redis server") implementation —
  point `-h`/`-p` at it, e.g. `redis-cli -p 6379 PING`.

```sh
redis-cli PING
redis-cli SET name John
redis-cli GET name
redis-cli INCR counter
redis-cli LPUSH list a b c
redis-cli LRANGE list 0 -1
```

## Architecture

| Package  | Responsibility |
| -------- | -------------- |
| `resp`   | RESP codec: `Encode` (command → bytes), `Decode` (bytes → `Value`, recursive for nested arrays), `Marshal` (inverse, used by tests). Pure, no network. |
| `client` | `Client` over `net.Conn` with buffered RESP read/write: `Dial`, `Send`, `Close`. |
| `repl`   | Line tokenizer (quotes/escapes), interactive loop, pipe mode, local `quit`/`exit`/`help`. |
| `format` | `Render` — `Value` → redis-cli-style display string. |
| `help`   | Embedded command docs (`commands.json`): overview, per-command, per-group. |
| `main`   | Flag parsing and mode selection. |

## Tests

```sh
go test ./...
go test ./... -cover
go test ./... -race -cover   # needs a 64-bit cgo toolchain
```

Coverage:

- **`resp`** — table-driven encode/decode for every type, nested arrays, nils,
  empty values, binary-safe bulk strings, and negative cases (bad length,
  missing CRLF, unknown type byte, truncated/empty stream). Marshal⇄Decode
  round-trips.
- **`format`** — golden-string assertions including nested arrays, index
  alignment, nil, empty array, and quoting/escapes.
- **`repl`** — tokenizer (quotes, escapes, unterminated), pipe-mode
  stop-on-error, local commands not sent to the server, interactive prompt and
  error tolerance.
- **`client`** — integration against an **in-process fake Redis server** (a real
  `net.Listener` speaking RESP): PING→PONG, SET→OK, GET miss→`(nil)`, array
  replies, error propagation, arg round-trip across the socket.
- **`help`** — overview, command, group, and usage lookups.
```
