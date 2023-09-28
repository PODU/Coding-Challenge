# ntpclient — Coding Challenge #28 (NTP client)

A small, dependency-free **SNTP client** in Go. It queries an NTP server over
UDP, decodes the 48-byte RFC 5905 response, and reports the local clock
**offset** (θ) and **round-trip delay** (δ).

Reference: [Coding Challenge #28](https://codingchallenges.substack.com/p/coding-challenge-28-ntp-client) ·
Protocol: **RFC 5905** (NTPv4 message format), **RFC 4330** (SNTP client behaviour).

## Layout

```
cmd/ntpclient/main.go        CLI: flags, output formatting, exit codes
internal/ntp/packet.go       48-byte packet struct, Marshal/Unmarshal (big-endian)
internal/ntp/timestamp.go    NTP 64-bit timestamp <-> time.Time (32.32 fixed point)
internal/ntp/client.go       UDP transport + offset/delay math (Query / QueryBest)
internal/ntp/kod.go          Kiss-o'-Death / leap / mode / version / originate checks
internal/ntp/*_test.go       Offline unit tests + gated live test
```

The codec (`packet.go`), timestamp conversion (`timestamp.go`), and offset/delay
math (`CalculateOffset` / `CalculateDelay` in `client.go`) are deliberately
separated from the UDP transport so the core is fully unit-testable offline.

## Build & test

```sh
go build ./...
go vet ./...
go test ./...          # fully offline, deterministic
```

The default `go test` run does **not** touch the network. The live query is
gated behind an environment flag:

```sh
NTP_LIVE_TEST=1 go test ./internal/ntp -run TestLiveQuery -v
```

## Run

```sh
go run ./cmd/ntpclient                       # query pool.ntp.org
go run ./cmd/ntpclient -v                     # show all four timestamps
go run ./cmd/ntpclient -server time.google.com -samples 4
go run ./cmd/ntpclient -json
```

### Flags

| Flag       | Default            | Meaning                                        |
|------------|--------------------|------------------------------------------------|
| `-server`  | `pool.ntp.org:123` | NTP server `host[:port]` (port 123 if omitted) |
| `-timeout` | `5s`               | UDP read timeout                               |
| `-samples` | `1`                | Number of queries; reports best (lowest delay) |
| `-json`    | `false`            | Machine-readable JSON output                   |
| `-v`       | `false`            | Verbose: print all four timestamps + header    |
| `-set`     | `false`            | Reserved; clock-stepping not implemented (see caveats) |

### Example output

```
server:   pool.ntp.org:123
stratum:  2
ref id:   0xa29fc801 (....)
offset:   -3.142 ms
delay:    21.880 ms
local:    2026-06-26T12:00:00.123Z
ntp time: 2026-06-26T12:00:00.120Z
```

### Exit codes

| Code | Meaning                          |
|------|----------------------------------|
| 0    | success                          |
| 1    | usage / flag error               |
| 2    | network / timeout                |
| 3    | invalid response / Kiss-o'-Death |

## How it works

- **Request** — a 48-byte packet whose first byte is `0x23` (LI=0, VN=4,
  Mode=3 client). The transmit timestamp (t₀) is written so the server echoes
  it back in the Originate field for correlation.
- **Timestamps** — 64-bit NTP timestamps are 32-bit seconds since the NTP epoch
  (1900-01-01) plus a 32-bit binary fraction. Conversion to Unix time uses the
  constant **2 208 988 800** seconds and 32.32 fixed-point fraction scaling.
- **Math** —
  - offset θ = `((t1 − t0) + (t2 − t3)) / 2`
  - delay  δ = `(t3 − t0) − (t2 − t1)`

  where t₀ = client transmit, t₁ = server receive, t₂ = server transmit,
  t₃ = client receive. t₀ and t₃ are taken from the local clock immediately
  around the `Write`/`Read` to minimise software latency in the measurement.

## Edge cases handled

- **Kiss-o'-Death** (stratum 0): the 4-char Reference ID code (`DENY`, `RSTR`,
  `RATE`, …) is surfaced as a `*KissOfDeathError`; `QueryBest` stops retrying.
- **Leap indicator = 3** (unsynchronised) → rejected.
- **Mode** must be 4 (server); **version** must be 3 or 4.
- **Originate mismatch**: the response Originate timestamp must equal the t₀ we
  sent — guards against stale / spoofed replies.
- **Stratum > 15** (non-KoD) → unusable.
- **Timeout / short read** → network error / decode error.

## Caveats

- **2036 era rollover.** The 32-bit NTP seconds field wraps at the end of NTP
  era 0 (2036). This client assumes era 0 for the absolute `time.Time`
  conversion. The offset/delay math operates on `time.Duration` differences, so
  near-term measurements are unaffected; full NTPv4 era handling is future work.
- **`-set` is not implemented.** Stepping the system clock is privileged and
  OS-specific. The flag is accepted but returns a usage error pointing you to
  OS tooling (`w32tm` on Windows, `ntpdate`/`adjtimex` on Linux) with the
  reported offset. Reporting (the core challenge) is complete.
- **Single-packet SNTP.** No NTP server/peer state machine, no Marzullo
  filtering across multiple servers — by design for this challenge.
- Live tests require outbound UDP/123, which some networks block.
