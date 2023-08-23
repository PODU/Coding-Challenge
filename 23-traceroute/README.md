# traceroute — Coding Challenge #23

A production-ready `traceroute` clone in Go. It maps the network path to a
destination by sending probes with an incrementing IP **TTL** and recording the
ICMP responses from each router along the way.

## How it works

1. Resolve the destination hostname to an IPv4 address and print the header.
2. For `ttl = 1..max-hops`, send `queries` probes per hop:
   - **UDP mode (default):** a UDP datagram to ports `33434 + (ttl-1)`; routers
     reply with **ICMP Time Exceeded**, and the destination replies with
     **ICMP Port Unreachable**.
   - **ICMP mode (`-I`):** an ICMP **Echo Request**; the destination replies
     with an **Echo Reply**. Recommended on Windows.
3. Record the responding hop address and per-query round-trip time, resolve the
   hop to a hostname (reverse DNS, falling back to the bare IP), and print one
   line per hop. Timed-out queries render as `*`. Probing stops when the
   destination answers or `max-hops` is reached.

## Build

```
go build ./cmd/traceroute
```

Produces a `traceroute` binary (`traceroute.exe` on Windows). Requires Go 1.25+
(the `golang.org/x/net` dependency sets the floor).

## Usage

```
traceroute [options] <host>

Options:
  -m, --max-hops   int       Maximum number of hops (TTL).        Default: 64
  -q, --queries    int       Probe queries per hop.               Default: 3
  -w, --timeout    duration  Per-query wait for a reply (e.g. 2s).Default: 3s
  -p, --port       int       Base destination UDP port.           Default: 33434
  -s, --size       int       Probe packet size in bytes.          Default: 32
  -n                         Numeric output; skip reverse DNS.
  -I, --icmp                 Use ICMP Echo probes instead of UDP.
  -h, --help                 Show usage.
```

Example:

```
$ traceroute -m 30 -q 3 -w 2s example.com
traceroute to example.com (93.184.216.34), 30 hops max, 32 byte packets
 1  router.lan (192.168.1.1)  1.204 ms  0.998 ms  1.011 ms
 2  * * *
 3  10.0.0.1 (10.0.0.1)  9.331 ms  8.872 ms  9.004 ms
 ...
30  example.com (93.184.216.34)  21.553 ms  20.901 ms  21.010 ms
```

## Privileges / elevation

Receiving ICMP requires a **raw socket** (`ip4:icmp`), which needs elevated
privileges:

- **Windows:** run from an **Administrator** terminal. ICMP mode (`-I`) is the
  most reliable path because the Windows firewall/stack can drop the
  UDP-probe + raw-ICMP-receive combination. Example (elevated PowerShell):

  ```
  .\traceroute.exe -I 1.1.1.1
  ```

- **Linux / macOS:** run with `sudo`, or grant the binary the
  `cap_net_raw` capability on Linux:

  ```
  sudo setcap cap_net_raw+ep ./traceroute
  ```

If the raw socket cannot be opened, the tool prints an actionable message
(`... requires elevated privileges (run as Administrator on Windows / root on
Unix)`) rather than a raw stack trace.

> **Windows alternative (not implemented here):** the Win32 `IcmpSendEcho` /
> `IcmpSendEcho2` API (`Iphlpapi.dll`) sends a TTL-limited ICMP echo without a
> raw socket and backs `tracert.exe`. It would make a good future Windows-specific backend behind the same `Prober` interface.

## Testing

The packet builder, ICMP parser, hop-table formatter, reverse-DNS fallback, and
the TTL loop are **pure / injectable** and unit-tested **offline** with byte
fixtures under `internal/trace/testdata/`. The default test run needs no
network and no elevation:

```
go test ./...
```

The live integration test actually probes the network and needs raw-socket
privileges. It is skipped unless `TRACEROUTE_LIVE=1` is set, so default CI stays
green:

```
# elevated terminal
TRACEROUTE_LIVE=1 go test ./internal/trace -run TestLiveTrace -v
```

### Regenerating fixtures

The ICMP test fixtures are committed. To regenerate them:

```
go run gen_fixtures.go
```

(`gen_fixtures.go` carries a `//go:build ignore` tag, so it is excluded from the
normal build and test.)

## Project layout

```
23-traceroute/
  README.md                 this file
  go.mod
  gen_fixtures.go           //go:build ignore — fixture generator
  cmd/traceroute/main.go    CLI: flags, DNS, header, drive the loop
  internal/trace/
    trace.go                config, types, Prober interface, TTL loop
    probe.go                UDP payload + ICMP Echo builders, dest-port math
    parse.go                pure ICMP parser + correlation/classification
    receiver.go             live raw-socket Prober (privileged)
    dns.go                  reverse-DNS resolver with cache + numeric mode
    print.go                classic hop-line formatter
    *_test.go               offline unit tests + gated live test
    testdata/               handcrafted ICMP byte fixtures
```

## Caveats / scope

- **IPv4 only.** IPv6 (ICMPv6) is a possible future extension.
- Probing is **synchronous** (one outstanding probe at a time) for simplicity
  and robust correlation; it is not parallelised across hops.
- `DestinationUnreachable` is treated as "reached" only for code 3 (port
  unreachable); other unreachable codes are not specially annotated (no `!H`,
  `!N` markers).
- The live raw-socket prober is covered by the gated live test, not the default
  offline suite.
