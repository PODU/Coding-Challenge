# ccscan — TCP connect port scanner

`ccscan` is a small, nmap-style command-line **TCP connect port scanner** written in pure
Go (standard library only). Given one or more target hosts and a range of ports, it
concurrently attempts TCP connections and reports which ports are **open**, **closed**, or
**filtered**. It is fast (bounded worker-pool concurrency), predictable (per-connection
timeout), and **safe by default** — it scans `localhost` unless told otherwise.

This implements Steps 1–4 of John Crickett's
[Coding Challenge #45](https://codingchallenges.substack.com/p/coding-challenge-45-port-scanner).

---

## ⚠️ Legal / Ethical Notice — READ FIRST

> Port scanning a host you do **not** own or have **written permission** to test may be
> **illegal** in many jurisdictions (e.g. the U.S. Computer Fraud and Abuse Act, the UK
> Computer Misuse Act, and equivalents worldwide) and almost always violates the Acceptable
> Use Policy of ISPs and cloud providers. It can also trigger intrusion-detection systems and
> get you blocked or reported.
>
> **Only scan systems you own or are explicitly authorized to assess** (your own machine,
> your own LAN, a lab VM). `ccscan` defaults to `localhost` precisely so the safe path is the
> default path. The author and this tool accept **no liability for misuse**.

This is an educational, authorized security-tooling exercise. It performs a **TCP connect**
scan only (the OS completes a normal 3-way handshake) — no raw sockets, no SYN/half-open
scanning, no privilege escalation.

---

## Build & test

```sh
cd 45-port-scanner
go build ./...
go vet ./...
go test ./... -race      # all tests touch only localhost / in-process listeners
```

Install the binary:

```sh
go build -o ccscan ./cmd/ccscan
```

---

## Usage

```
ccscan [flags]

  -host string    Target host(s): single host/IP, comma-list, wildcard
                  (192.168.1.*), or CIDR (192.168.1.0/24). Default: "localhost"
  -port string    Port spec: single (80), list (22,80,443), range (1-1024),
                  combination (22,80,8000-8100), or empty for all 1-65535.
  -c int          Concurrency: number of worker goroutines. Default: 512
  -t duration     Per-connection timeout (e.g. 500ms, 1s). Default: 500ms
  -format string  Output format: "text" or "json". Default: "text"
  -v              Verbose: also list closed/filtered ports. Default: false
  -allow-large    Permit CIDR blocks larger than /24 (mass-scan guard).
  -h, -help       Show usage.
```

### Examples

```sh
ccscan -host=localhost -port=5000                      # Step 1: single port
ccscan -host=localhost                                 # Step 2: full vanilla scan (1-65535)
ccscan -host=localhost -port=1-1024 -c=1000 -t=300ms   # Step 3: optimized
ccscan -host=192.168.1.* -port=22,80,443               # Step 4: sweep (YOUR OWN LAN ONLY)
ccscan -host=10.0.0.0/24 -port=22 -format=json         # JSON for automation
```

### Example output (text)

```
Scanning 1 host(s) — 65535 port(s) each, concurrency 512, timeout 500ms

Host: 127.0.0.1
  PORT       STATE     SERVICE
  22/tcp     open      ssh
  80/tcp     open      http
  6379/tcp   open      redis

  65535 ports scanned: 3 open, 65532 closed, 0 filtered  (elapsed 1.8s)
```

### JSON output

`-format=json` emits a machine-readable report (per-host counts plus the open — or, with
`-v`, all — ports) suitable for piping into other tools.

---

## How it classifies ports

| Result      | Meaning |
|-------------|---------|
| `open`      | TCP handshake completed — a service is listening. |
| `closed`    | Host actively refused the connection (RST) — reachable, nothing listening. |
| `filtered`  | Connection timed out / unreachable — firewall drop or host down. |

---

## Design notes

- **Bounded concurrency**: a fixed worker pool (`-c`) caps the number of concurrent sockets,
  so memory and file-descriptor use stay flat regardless of how many ports are scanned.
- **Per-connection timeout**: every dial is bounded by `-t` and by the root context.
- **Graceful cancellation**: Ctrl-C cancels in-flight dials via `context`; partial results are
  printed and the process exits 130.
- **Pure stdlib**: no external dependencies; builds and runs on Windows, Linux, and macOS.

**Out of scope:** Step 5 (SYN / half-open scan) requires raw sockets and root/Administrator
privileges and platform-specific code; it is intentionally omitted to keep `ccscan` portable
and unprivileged.

## Layout

```
45-port-scanner/
├── go.mod
├── cmd/ccscan/main.go        # CLI: flags, signal handling, wiring
└── internal/scanner/
    ├── ports.go              # ParsePorts
    ├── hosts.go              # ParseHosts (list / wildcard / CIDR)
    ├── scan.go               # scanPort, worker pool, Job/Result types
    ├── services.go           # well-known port -> service map
    ├── report.go             # text + json reporters
    └── *_test.go             # localhost-only unit + integration tests
```
