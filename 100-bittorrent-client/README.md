# bittorrent — a single-file BitTorrent download client

Coding Challenge #100. A production-ready leech client for **single-file
torrents over an HTTP tracker**, written in Go with the standard library only
(zero third-party dependencies). It implements BEP 3: bencode, the SHA-1 info
hash, tracker announce with compact peers, the peer wire handshake and message
codec, concurrent piece download with 16 KiB block pipelining and per-piece
SHA-1 verification, and file assembly at correct byte offsets.

## Build

```
cd 100-bittorrent-client
go build -o bittorrent ./cmd/bittorrent
```

Requires Go 1.24+. No external modules.

## Usage

```
bittorrent info     <path-to-.torrent>            # print metainfo (no network)
bittorrent peers     --net <path-to-.torrent>     # announce + list peers
bittorrent download  --net [flags] <.torrent>     # download the file
```

Network access is **off by default** so tests and CI never touch the network;
`peers` and `download` require the explicit `--net` flag.

### Download flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-o` | `info.name` | Output file path |
| `-p` | `6881` | Local port advertised to the tracker |
| `--peers` | `30` | Max concurrent peer connections |
| `--net` | off | Enable tracker announce + live peer download |
| `--timeout` | `15s` | Per-peer dial/read timeout |
| `-v` | off | Verbose per-piece progress logging |

Exit codes: `0` success, `1` usage error, `2` parse/metainfo error,
`3` tracker error, `4` download incomplete/verification failure.

### Example

```
bittorrent info testdata/sample.torrent
bittorrent download --net -v -o ./out.bin debian.torrent
```

## Architecture

```
cmd/bittorrent/          CLI entrypoint and orchestration
internal/bencode/        recursive-descent decoder + canonical (sorted) encoder
internal/metainfo/       .torrent parsing; SHA-1 info hash over verbatim info dict
internal/tracker/        HTTP announce; compact (and dict-model) peer parsing
internal/peer/           handshake, message codec, bitfield, per-conn client state
internal/download/       piece queue, worker pool, block pipelining, verify, assemble
internal/testutil/       deterministic in-memory torrent + in-process fake peer
testdata/sample.torrent  committed fixture (single-file, 64000 bytes, 4 pieces)
```

The info hash is computed over the **verbatim byte span** of the bencoded
`info` dictionary (captured during decode via `DecodeWithRaw`), so it is
correct even when the source key order is non-canonical.

Downloading uses one goroutine per peer pulling from a shared buffered work
queue. Each worker pipelines up to 5 outstanding 16 KiB block requests,
respecting choke/unchoke. A completed piece is SHA-1-verified against the
metainfo hash; on mismatch (or peer disconnect) the piece is requeued for
another worker. A collector goroutine writes each verified piece at
`index * pieceLength` and finishes when all pieces are present.

## Testing

```
go test ./...                 # offline unit + fake-peer integration tests
go vet ./...
gofmt -l .                    # should print nothing
```

Offline coverage:

- **bencode**: round-trip of ints/strings/lists/dicts, canonical key sorting,
  binary strings, malformed-input rejection, raw info-span capture.
- **metainfo**: info hash on `testdata/sample.torrent` matches a committed
  golden SHA-1; piece count, last-partial-piece sizing, multi-file rejection.
- **peer**: handshake codec (68 bytes), every message id encode/decode,
  keep-alive, `FormatRequest`/`ParsePiece`/`ParseHave`, bitfield high-bit-first.
- **tracker**: compact + dictionary peer parsing, `failure reason`, announce
  URL construction with raw `info_hash` percent-encoding.
- **download**: SHA-1 verify (good + corrupt), and a full download-loop
  integration test against an **in-process fake peer** over `net.Pipe`
  (including a deliberate bad-block → requeue → recovery path), asserting the
  assembled bytes equal the source.

### Network-gated tests (opt-in)

Live tracker tests are behind the `net` build tag and a `-net` flag:

```
go test -tags net ./internal/tracker/ -net -torrent path/to/file.torrent
```

## Caveats

- Single-file torrents over **HTTP** trackers only. Multi-file torrents are
  detected and rejected; UDP trackers, DHT, magnet links, PEX, and seeding are
  out of scope (per the challenge brief).
- `go test -race` requires a 64-bit C toolchain (cgo). On a Windows box without
  one it fails to build the race runtime; the logic itself is race-clean
  (state is confined to channels and disjoint per-piece file offsets). Run
  `go test -race ./internal/download/` on Linux/macOS or with a 64-bit gcc.
