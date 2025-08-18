# whois — Coding Challenge #96

A small, dependency-free **WHOIS client** in Go implementing the WHOIS protocol
(**RFC 3912**). It looks up **domains, IP addresses, and AS numbers** by
connecting to `whois.iana.org` on **TCP port 43**, then **follows referrals**
(`refer:`, `whois:`, `Registrar WHOIS Server:`) recursively down to the
authoritative registry/registrar, parses the key fields, and prints a clean
merged record.

## Build

```sh
go build ./...
go build -o whois ./cmd/whois     # or whois.exe on Windows
```

Requires Go 1.24+. No third-party dependencies (standard library only).

## Usage

```
whois [flags] <query>

<query>   Domain (example.com), IPv4/IPv6 (8.8.8.8), or ASN (AS15169). Required.

Flags:
  -h, --host <server>   Query this WHOIS server directly; skip IANA discovery
                        (implies a single, direct hop — no referral following).
  -p, --port <n>        TCP port (default 43).
  -r, --raw             Print raw concatenated server responses, no parsing.
  -j, --json            Emit the parsed merged record as JSON.
      --no-referral     Do not follow refer/registrar referrals (single hop).
      --timeout <dur>   Per-hop dial+read timeout (default 10s).
  -v, --verbose         Log each hop (server, query sent) to stderr.
```

### Examples

```sh
whois example.com                         # IANA -> .com registry -> registrar
whois -j google.com                       # merged record as JSON
whois 149.72.235.118                      # IANA -> RIR (ARIN/RIPE/...) -> IP record
whois AS15169                             # ASN lookup
whois -h whois.verisign-grs.com example.com   # query one server directly
whois -r -v example.com                   # raw output + per-hop logging on stderr
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — record found |
| 1 | Usage error |
| 2 | Network / timeout error |
| 3 | No match / object not found |

## How it works

1. **resolver** classifies the query (domain / IPv4 / IPv6 / ASN) and formats
   the request per server (e.g. Verisign needs `domain <name>`, ARIN needs
   `n + <ip>`).
2. **client** is the RFC 3912 transport: dial `:43`, write `request + CRLF`,
   read until the server closes the connection (EOF).
3. **referral loop** seeds at IANA and hops to each referral server (with a
   visited-set and max-hop cap for cycle/runaway protection), merging every
   hop into one `Record`.
4. **parser** turns each raw response into ordered key/value `Field`s (comments
   and blank lines stripped) and extracts the referral + no-match signals.
5. **presenter** renders the merged record as aligned text, raw dump (`-r`), or
   JSON (`-j`).

## Tests

All parsing and query-building logic is tested **offline** against checked-in
fixtures under `internal/whois/testdata/`. The transport is exercised by an
**in-process fake port-43 server**; the referral loop is tested with two fake
servers chained by a `refer:` field. No network access is required.

```sh
go test ./...            # full offline suite (unit + transport + integration)
go vet ./...
gofmt -l .               # should print nothing
```

A live smoke test against `whois.iana.org` is gated behind a build tag and is
**excluded** from the default run:

```sh
go test -tags live -run TestLive ./...
```

## Layout

```
96-whois/
├── README.md
├── go.mod
├── cmd/whois/main.go            CLI wiring (flags, exit codes)
└── internal/whois/
    ├── client.go / client_test.go        TCP :43 transport + fake-server tests
    ├── resolver.go / resolver_test.go     classify + per-server query formatting
    ├── parser.go / parser_test.go         text -> Field/Record, referral, no-match
    ├── referral.go / referral_test.go     recursive orchestrator + integration
    ├── present.go                         human / raw / JSON output
    ├── live_test.go                       network smoke test (-tags live)
    └── testdata/                          checked-in fixture WHOIS responses
```

## Out of scope / future work

- RDAP (JSON-over-HTTPS, RFC 7482/9082) fallback.
- TLD→server resolution caching, batch mode, colourised TTY output.
