# dns-forwarder

A production-grade **DNS forwarder** (Coding Challenge #44). It is a UDP server
that answers DNS queries from clients (`dig`, the OS stub resolver, …) either
from a local **TTL-aware cache** or by **forwarding** to a configured upstream
recursive resolver (default Google Public DNS, `8.8.8.8:53`). Successful answers
are cached and re-served until their TTL expires. An optional **blocklist**
sinkholes ad/tracker domains.

## Features

- **RFC 1035 codec** — bounds-checked header/question/RR parsing with name
  decompression (0xC0 pointers) and a pointer-loop guard; uncompressed encoder.
- **TTL cache** — keyed by `(name, qtype, qclass)`; serves within the TTL window,
  rewrites the counting-down remaining TTL into each served record, and
  re-looks-up on expiry. The clock is injectable for deterministic tests.
- **Forwarding** — per-exchange UDP socket, response correlated to the query by
  transaction ID (stray/spoofed replies dropped); bounded timeout with one
  retry; optional TCP fallback on truncated (TC=1) responses.
- **Concurrency** — each datagram is handled in its own goroutine behind a
  bounded worker semaphore, so a slow upstream never blocks new reads.
- **Robustness** — never crashes on a malformed packet: replies `FORMERR`, and
  `SERVFAIL` on upstream failure/timeout; a panic backstop keeps the server up.
- **Blocklist** — newline-delimited file (bare domains or hosts-file style);
  matches a name or any of its subdomains; answers `NXDOMAIN` or a `0.0.0.0`/`::`
  sinkhole.
- **Observability** — structured `slog` logging with levels; hit/forward/block/
  error counters reported on shutdown.
- **Graceful shutdown** on SIGINT/SIGTERM with clean socket close.

## Layout

```
cmd/dnsfwd/main.go        CLI wiring + signal handling
internal/dnsmsg/          RFC 1035 parse/encode (pure, no I/O)
internal/cache/           TTL cache with injectable clock
internal/upstream/        upstream UDP/TCP resolver client
internal/blocklist/       optional domain blocklist
internal/server/          UDP listener + request handler
```

## Build & run

```sh
go build -o dnsfwd ./cmd/dnsfwd

# Listen on a high port (no privileges needed) and forward to Google DNS:
./dnsfwd -listen :1053 -upstream 8.8.8.8:53

# Acceptance test from the challenge:
dig @127.0.0.1 -p 1053 www.google.com
# Run it twice — the second response is served from cache (visible in -log-level debug).
```

Port 53 requires elevated privileges; use a high port like `1053` for local
testing.

### CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:1053` | UDP listen address `host:port`. |
| `-upstream` | `8.8.8.8:53` | Upstream resolver `host:port`. |
| `-timeout` | `5s` | Per-query upstream timeout. |
| `-cache` | `true` | Enable the TTL cache. |
| `-max-ttl` | `0` | Cap cached TTL seconds (0 = off). |
| `-min-ttl` | `0` | Floor cached TTL seconds (0 = off). |
| `-blocklist` | `""` | Path to a newline-delimited blocklist file. |
| `-sinkhole` | `nxdomain` | Block mode: `nxdomain` or `zero` (`0.0.0.0`/`::`). |
| `-tcp-fallback` | `false` | Retry truncated (TC=1) responses over TCP. |
| `-log-level` | `info` | `debug` / `info` / `warn` / `error`. |

## Tests

All core logic is unit-tested **offline** (no real network egress):

```sh
go test ./... -cover
```

- `dnsmsg` — golden-byte query/response fixtures, compression-pointer decode,
  Decode→Encode round-trip, malformed/pointer-loop negative cases (≥90% covered).
- `cache` — fake clock: hit, remaining-TTL countdown, expiry eviction, TTL
  cap/floor, zero-TTL-not-cached, concurrent access.
- `upstream` — in-process fake UDP resolver: success, ID matching, timeout.
- `server` — end-to-end over loopback with a fake upstream: miss→forward→answer,
  cache hit (upstream call count stays 1), per-qtype isolation, blocklist
  NXDOMAIN + zero sinkhole, malformed→FORMERR (server stays alive),
  upstream-failure→SERVFAIL.

### Live test (gated)

A real-network test is excluded from the default build behind the `live` tag and
an env flag:

```sh
LIVE_DNS=1 go test -tags=live ./internal/server/ -run TestLiveUpstream -v
```

It forwards to `8.8.8.8` and asserts a `NOERROR` answer with ≥1 A record.

## Notes

- The race detector (`go test -race`) requires a 64-bit cgo C toolchain; it is
  unavailable on a MinGW-32-bit-only host. Tests pass without `-race`.
- Forwarding relays the raw query bytes verbatim, preserving any EDNS0 OPT
  record from the client; the message is only *parsed* for logging, the cache
  key, blocklist matching, and TTL rewriting.
