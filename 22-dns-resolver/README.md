# dnsresolve — Build Your Own DNS Resolver (Coding Challenge #22)

A from-scratch DNS resolver in Go. It hand-constructs binary DNS query messages,
sends them over UDP (falling back to TCP on truncation), parses responses
including RFC 1035 **message compression pointers**, and — for full credit —
**recursively resolves a hostname starting from a root nameserver**, following
NS/A referrals down the delegation chain until an authoritative A record is
found.

No third-party DNS library is used: the wire-format encoder and decoder are
written by hand against RFC 1035.

## Build

```sh
go build -o dnsresolve ./cmd/dnsresolve
```

Produces a single static binary with no runtime dependencies. Requires Go 1.24+.

## Usage

```
dnsresolve [flags] <hostname>

Flags:
  -type string     record type: A (default), NS, CNAME, AAAA
  -server host     query this resolver directly (RD=1) instead of recursing
                   from root, e.g. -server 8.8.8.8   (covers Steps 1-3)
  -trace           print every server consulted and the records returned
  -timeout dur     per-query UDP timeout (default 3s)
  -retries int     UDP attempts per server (default 2)
  -tcp             force TCP; otherwise TCP is used only on a truncated reply
  -json            machine-readable output

Exit codes:
  0 resolved   1 usage error   2 NXDOMAIN / no records   3 network/timeout
```

### Examples

Recursive resolution from the root, with a trace of every server consulted:

```
$ dnsresolve -trace dns.google.com
;; querying a.root-servers.net (198.41.0.4) for dns.google.com A -> referral
;;   -> NS a.gtld-servers.net (glue 192.5.6.30)
;;   ...
;; querying l.gtld-servers.net (192.41.162.30) for dns.google.com A -> referral
;;   -> NS ns1.google.com (glue 216.239.32.10)
;;   ...
;; querying ns2.google.com (216.239.34.10) for dns.google.com A -> NOERROR
;;   dns.google.com A 8.8.8.8 (ttl 900)
;;   dns.google.com A 8.8.4.4 (ttl 900)
8.8.8.8
8.8.4.4
```

Query a recursive resolver directly (Steps 1–3 of the challenge):

```
$ dnsresolve -server 8.8.8.8 dns.google.com
8.8.8.8
8.8.4.4
```

Other record types and machine-readable output:

```
$ dnsresolve -type NS google.com
$ dnsresolve -json example.com
```

## Architecture

The codec is a pure, I/O-free package so it is **fully testable offline**.

```
cmd/dnsresolve/main.go      CLI: flags, output formatting, exit codes
internal/message/           hand-written RFC 1035 codec (no network)
  types.go                  Header/Question/Record/Message + enums + accessors
  encode.go                 Message -> []byte (length-prefixed labels, big-endian)
  decode.go                 []byte -> Message, incl. compression-pointer handling
internal/transport/         UDP exchange w/ timeout + retries; TCP on truncation
internal/resolver/          iterative resolution from root; NS/A referral walk
  roots.go                  the 13 root nameserver IPv4 addresses
```

Key implementation details:

- **Compression pointers** (`decode.go` `readName`): a label whose two high bits
  are `11` is a 14-bit pointer to an earlier message offset. The decoder follows
  pointers, caps the number of jumps (loop guard), bounds-checks every offset,
  and returns the position *after* the pointer so the caller continues
  correctly. NS/CNAME RData that contains pointers is re-decoded against the full
  message via a recorded RData offset.
- **Strict bounds checking**: every read on untrusted network input is bounds-
  checked; malformed datagrams return an error and never panic or read OOB.
- **Recursive resolver** (`resolver.go`): queries with RD=0 (we do the
  recursion), starts at the roots, reads NS records from AUTHORITY and glue A
  records from ADDITIONAL, recursively resolves NS hostnames when glue is
  missing, restarts on CNAME (bonus), and honors NXDOMAIN as definitive. A hop
  limit and recursion-depth caps guard against referral loops.

## Testing

The wire-format codec is tested **100% offline** with hand-computed byte
fixtures, including compression pointers and malicious inputs. Live recursive
resolution is integration-only and gated behind a build tag, so the default test
run never touches the network.

```sh
go test ./...                      # offline unit suite (no network)
go test -tags=integration ./...    # + live resolution from the real roots
go vet ./...
```

Offline coverage:

- `internal/message` — `encodeName` and `Pack` against exact hex fixtures;
  oversize label/name rejection; decode of a real `dns.google.com` response;
  compression-pointer reconstruction for record owner names and NS/CNAME RData;
  malicious fixtures (pointer loop, pointer past end, truncated record, zero
  buffer) all return errors without panicking; `Unpack(Pack(m))` round-trip.
- `internal/transport` — loopback UDP echo server (no internet) verifies
  `Exchange` and its timeout behavior; TC-bit detection.
- `internal/resolver` — a scripted, in-memory `Exchanger` drives the full
  root → TLD → authoritative walk, plus NXDOMAIN, CNAME restart, glueless-NS
  recursion, and an NS-type query — entirely offline.

## Notes / caveats

- `go test -race` requires a working cgo/64-bit gcc toolchain; on a host without
  one it fails to build the race runtime (an environment limitation, not a code
  issue). The non-race suite covers the same logic.
- The resolver queries nameservers over IPv4. AAAA/unknown RR types decode
  generically (RData kept raw) and are surfaced when requested.
