# DHCP Client — Coding Challenge #94

A command-line **DHCP client** in Go that performs the full **DORA** handshake
(DISCOVER → OFFER → REQUEST → ACK) to lease an IPv4 address, with support for
NAK, DECLINE and RELEASE. The core is a clean, fully unit-tested
**BOOTP/DHCP codec** (RFC 2131 wire format, RFC 2132 TLV options with the magic
cookie) and a transport-agnostic **DORA state machine** that is testable
**offline** with byte fixtures and an in-process fake server — no real network,
broadcast privileges, or admin rights are needed for the test suite.

## Layout

```
cmd/dhcpclient/main.go        CLI: flag parsing, wiring, output
internal/dhcp/                RFC 2131/2132 message codec
  message.go                  Message struct, constants, accessors
  options.go                  Option type, codes, TLV constructors, magic cookie
  encode.go                   Message.Marshal() -> []byte
  decode.go                   Unmarshal([]byte) -> Message (bounds-checked)
  builders.go                 BuildDiscover/Request/Release/Decline
internal/client/              DORA state machine + UDP transport
  transport.go                Transport interface + ErrTimeout
  statemachine.go             DORA runner + pure Transition() table
  udp.go                      real broadcast transport (gated, needs admin)
  sockopt_*.go                per-OS SO_REUSEADDR / SO_BROADCAST
internal/netcfg/              apply/release the lease on the interface (gated)
```

## Build & test

```
go build ./...
go vet ./...
go test ./...
```

All codec and state-machine tests run offline and deterministically (injected
clock + fake transport). No `--live`, admin, or real broadcast is involved.

## Usage

By default the client is **offline-safe**: it builds and prints the DISCOVER it
*would* send and exits without touching the network.

```
$ dhcpclient -v --mac aa:bb:cc:dd:ee:ff --xid 956560166
[INIT]      built DISCOVER xid=0x3903f326 mac=aa:bb:cc:dd:ee:ff
[note] not running with --live; no packets sent (use --live as admin)
```

Real DORA and interface configuration are gated behind `--live` / `--apply` and
require admin/root (binding UDP :68 and broadcasting):

```
dhcpclient --live -v               # real DORA, print the lease
dhcpclient --live --json           # lease as JSON
dhcpclient --live --apply          # also configure the interface
dhcpclient --live --release ...    # send DHCPRELEASE
```

### Flags

| Flag | Description |
|------|-------------|
| `--live` | Perform real UDP broadcast DORA (needs admin/root). |
| `--apply` | After ACK, apply the IP to the interface (implies `--live`). |
| `--release` | Send DHCPRELEASE for the bound lease and exit. |
| `--iface` | Interface name (default: first non-loopback up). |
| `--mac` | Override client MAC (hex, colon-separated). |
| `--xid` | Fixed transaction id (default: random). |
| `--timeout` | Per-message wait before retransmit (default 4s). |
| `--retries` | Max retransmissions per phase (default 4). |
| `--hostname` | Send Option 12 (host name). |
| `--request-ip` | Preferred address (Option 50) in DISCOVER. |
| `--json` | Emit the resulting lease as JSON. |
| `-v`, `--verbose` | Log each DORA step. |
| `--hexdump` | Hex dump every sent/received packet. |

Exit codes: `0` ok, `1` usage error, `2` timeout/no offer, `3` NAK, `4`
privilege error (live mode without admin).

## Protocol notes

- Fixed 236-byte BOOTP header at exact offsets, big-endian integers, 16-byte
  `chaddr`, followed by the magic cookie `0x63825363` and TLV options.
- `op` = BOOTREQUEST/BOOTREPLY, `htype` = 1, `hlen` = 6, broadcast flag
  `0x8000` set in DISCOVER/REQUEST since the client is unconfigured.
- `xid` is constant across a DORA cycle; replies with a different `xid`, no
  Option 53, or a bad magic cookie are discarded.
- Retransmission uses randomized exponential backoff (4s, 8s, 16s, … capped),
  bounded by `--retries`; NAK restarts DORA a bounded number of times.

## Live testing (manual, not in CI)

Run as admin against a lab DHCP server (e.g. `dnsmasq` / ISC `dhcpd` in a VM):

```
sudo ./dhcpclient --live -v          # Linux
# Windows: run an elevated shell, then dhcpclient.exe --live -v
```
