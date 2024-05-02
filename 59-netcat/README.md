# ccnc — Coding Challenge #59: Build Your Own netcat

A production-quality [`netcat`](https://en.wikipedia.org/wiki/Netcat) clone in
Go. `ccnc` opens a TCP or UDP connection — as a **client** (connect to
`host port`) or a **server** (`-l`) — and pumps bytes bidirectionally between
**stdin → socket** and **socket → stdout**. It also supports port scanning
(`-z`), process execution (`-e`) and canonical hex dumping (`-x`).

The core is the **bidirectional relay**: two independent `io.Copy` loops running
concurrently, with correct EOF / half-close semantics so closing one direction
does not prematurely tear down the other.

## Build

```sh
go build -o ccnc .      # produces ./ccnc (ccnc.exe on Windows)
```

No third-party dependencies — standard library only.

## Usage

```
ccnc [options] [host] [port]

Modes:
  ccnc host port              TCP connect (client)
  ccnc -l -p port             TCP listen  (server)
  ccnc -u host port           UDP client
  ccnc -u -l -p port          UDP server
  ccnc -z host port[-port]    port scan (no data sent)

Options:
  -l            listen for an incoming connection
  -p port       local/remote port number
  -u            UDP instead of TCP
  -z            zero-I/O mode (scan)
  -e proc       execute proc, pipe its stdio over the connection
  -x            hex-dump traffic to stderr
  -v            verbose
  -h            help

Exit codes:
  0  normal termination (EOF / peer closed / scan completed)
  1  usage error / connection refused / bind failure
```

Grouped short flags work too: `ccnc -lvp 8888`.

### Examples

```sh
# Terminal A: TCP server that echoes (pipe its own input back)
ccnc -l -p 8888

# Terminal B: TCP client — type and see the round-trip
ccnc 127.0.0.1 8888

# Send a file and exit on EOF (half-close): the reply is still read first
echo "GET / HTTP/1.0" | ccnc example.com 80

# UDP server / client
ccnc -u -l -p 9999
ccnc -u 127.0.0.1 9999

# Port scan a range, printing only the open ports
ccnc -z localhost 8880-8890

# Hex-dump traffic (canonical hexdump -C) to stderr while relaying
ccnc -x 127.0.0.1 8888

# Execute a process and wire its stdio to the connection (see security note)
ccnc -l -p 8888 -e /bin/sh        # Unix
ccnc -l -p 8888 -e "cmd /c dir"   # Windows
```

## Design

```
argv ─▶ parseArgs ─▶ Config ─▶ run(cfg, stdin, stdout, stderr)
                                  │ dispatch
        ┌──────────┬─────────────┼───────────────┬──────────┐
   connect mode  listen mode  UDP modes      scan (-z)   exec (-e)
        └────┬─────┴──────────────┘               │
             ▼                                     ▼
        relay(conn, in, out, halfClose)     scanPorts → open ports
```

- **`parseArgs`** is pure (no I/O) and unit-tested in isolation. It normalises
  both port styles (`-p port` and positional `host port`) plus `-z` ranges.
- **`run`** injects `stdin`/`stdout`/`stderr` into every handler — nothing
  touches `os.Stdin`/`os.Stdout` directly, which is what makes the program
  testable over loopback with `bytes.Buffer`s.
- **`relay`** returns when the **conn → stdout** copy completes (remote EOF /
  close), mirroring real `nc`. On local stdin EOF the **stdin → conn** pump
  half-closes the TCP write side (`TCPConn.CloseWrite`) so the peer sees EOF
  while we keep reading the reply. The stdin pump is not waited on once the
  remote side is gone (so a TTY read can't hang the exit).
- **UDP** has no EOF / half-close; the server learns the peer address from the
  first datagram, then exchanges datagrams both ways.
- **`-z`** probes each port with a short-timeout `DialTimeout` through a bounded
  worker pool (128 workers) and prints only the open ports.
- **`-x`** wraps the relay streams: TX via `io.TeeReader`, RX via
  `io.MultiWriter`, dumping canonical `hexdump -C` lines to stderr while passing
  the data through unchanged.

## Tests

```sh
go test ./...        # unit + in-process integration, no external nc needed
```

Coverage:

- `parseArgs` — table-driven flag/port/range/error cases.
- TCP relay echo over a loopback listener (full round-trip).
- Client ↔ server **bidirectional** data relay (byte-exact, both directions).
- **Half-close / EOF** — peer observes stdin EOF yet its queued reply still
  arrives before exit.
- **UDP** datagram relay (both directions) over loopback.
- **`-z` port scan** against known-open listeners with a closed gap port.
- **Hex-dump** canonical formatting (single short line + full 16-byte rows +
  multi-write offset continuity).
- **`-e` exec** — child process stdout relayed back over the connection
  (uses `sh -c` / `cmd /c`; skipped if no shell is found).

## Windows vs. Unix differences

- **`-e` exec.** The wired-up program differs by platform: use `/bin/sh`,
  `/bin/bash`, etc. on Unix and `cmd`, `powershell`, etc. on Windows. The `-e`
  command string is split on whitespace into program + arguments. There is no
  cross-platform "shell" — pick the one that exists on the target host. The
  exec test branches on `runtime.GOOS` accordingly.
- **Half-close.** `CloseWrite` is used to signal EOF to the peer on TCP; it is a
  `*net.TCPConn` capability and behaves the same on both platforms. UDP never
  half-closes on either.
- **Security note.** `-e` (reverse/bind shell) hands a remote peer control of a
  local process. It is off by default and should only be used deliberately on
  trusted networks.

## Caveats / out of base scope

- TCP/UDP listen handles a **single** connection then exits (classic `nc -l`).
  A `-k` keep-listening loop is a natural extension.
- `-z` on **UDP** reports a successful `Dial` as "open", but UDP gives no
  connection guarantee, so this is best-effort (same limitation as real `nc`).
- The `-e` command splits on whitespace only (no shell quoting); wrap complex
  commands in an explicit shell invocation (`-e "sh -c '…'"`).
