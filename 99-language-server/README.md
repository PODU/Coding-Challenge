# greetls — Coding Challenge #99: Build Your Own Language Server (LSP)

A production-ready **Language Server** written in Go that speaks the
[Language Server Protocol](https://microsoft.github.io/language-server-protocol/)
over **stdio**, using **JSON-RPC 2.0** framed with `Content-Length` headers.

It targets a tiny, self-contained toy language called **`.greet`** so the
language-feature logic stays simple while still exercising every interesting
part of LSP: transport framing, dispatch, the lifecycle handshake, document
synchronization, **diagnostics**, **hover**, and **completion**.

## Quick start

```bash
cd 99-language-server
go build ./cmd/greetls      # produces greetls(.exe)
./greetls                   # speaks LSP over stdio; logs to stderr
go test ./...               # full suite — no editor required
```

`greetls` reads JSON-RPC from **stdin**, writes replies/notifications to
**stdout**, and logs to **stderr**. It exits `0` after a clean
`shutdown`/`exit`, and `1` if `exit` arrives without a prior `shutdown`.

## The `.greet` language

A `.greet` document is a line-oriented config:

```greet
# a comment
greet  "World"   @style=formal
lang   en
repeat 3
```

- **Keywords**: `greet`, `lang`, `repeat`, and `style` (after `@`).
- **Diagnostics** published on open/change/save:
  | Rule | Condition | Severity |
  |------|-----------|----------|
  | R-D1 | Unknown keyword at line start | Warning |
  | R-D2 | `greet` without a quoted argument | Error |
  | R-D3 | `repeat` value is not a positive integer | Error |
  | R-D4 | The marker `TODO` appears anywhere | Information |
  | R-D5 | Duplicate `lang` directive | Warning |
- **Hover**: hovering a keyword shows its Markdown documentation.
- **Completion**: keyword set at a normal position; attribute names after `@`.

## Architecture

```
stdin ─▶ Transport (Content-Length framing) ─▶ Dispatcher ─┬─▶ Lifecycle handlers
stdout ◀─ (single-writer mutex) ◀──────────────────────────┼─▶ Text-sync handlers ─▶ DocumentStore
                                                           └─▶ Feature handlers ──▶ Analyzer (.greet)
```

| Package | Responsibility |
|---------|----------------|
| `cmd/greetls` | Wires stdin/stdout into the server and runs the loop. |
| `internal/jsonrpc` | Message types + `Content-Length` stream framing (transport). |
| `internal/lsp` | LSP request/result/param data structures. |
| `internal/server` | Dispatcher, lifecycle state machine, and handlers. |
| `internal/document` | In-memory document store keyed by URI (text + version). |
| `internal/analyzer` | The `.greet` language: diagnostics, hover, completion. |

**Concurrency**: one reader goroutine owns stdin; handlers run synchronously in
the read loop (deterministic ordering); a single writer mutex serializes all
outgoing frames so a `publishDiagnostics` notification never interleaves with a
response. The document store is `RWMutex`-guarded.

## Advertised capabilities

```jsonc
{
  "textDocumentSync": 1,                                   // Full document sync
  "hoverProvider": true,
  "completionProvider": { "triggerCharacters": [".", "@"] }
}
```

## Methods handled

| Method | Kind | Behavior |
|--------|------|----------|
| `initialize` | request | returns `ServerCapabilities` |
| `initialized` | notification | marks the session ready |
| `shutdown` | request | returns `null`, enters shutting-down |
| `exit` | notification | terminates (code 0 if shutdown was received, else 1) |
| `textDocument/didOpen` | notification | store doc + publish diagnostics |
| `textDocument/didChange` | notification | replace text (full sync) + publish diagnostics |
| `textDocument/didSave` | notification | re-publish diagnostics |
| `textDocument/didClose` | notification | drop doc + clear diagnostics |
| `textDocument/hover` | request | docs for the keyword under the cursor |
| `textDocument/completion` | request | keyword / attribute completions |
| `$/*`, unknown notifications | notification | ignored (per spec) |
| unknown request | request | `-32601 MethodNotFound` |

## Edge cases handled

Malformed header / missing `Content-Length` (resync); truncated body (clean
EOF); invalid JSON body (`-32700` with recovered id); request before
`initialize` (`-32002`); request after `shutdown` (`-32600`); `exit` without
`shutdown` (exit code 1); unknown URIs (empty hover/completion, defensive
create on change); position past end of line (clamped, `null` hover); multibyte
UTF-8 read by byte count with UTF-16 column positions; a 16 MiB body cap.

## Editor integration (optional)

Point any LSP client at the `greetls` binary as the server command for files
matching `*.greet`, sync mode *full*. No editor is needed to run the tests —
the whole server is driven in-process over byte streams.

## Tests

`go test ./...` covers: frame encode/decode (round-trip, extra headers,
garbage/missing `Content-Length`, back-to-back messages, multibyte bodies,
oversized-body cap, truncated body, parse-error id recovery); request routing
and the `initialize → initialized → shutdown → exit` handshake (incl. exit
codes); document versioning; the analyzer's diagnostics/hover/completion on
sample `.greet` docs; and an end-to-end `didOpen → publishDiagnostics` loop over
pipes.

> Note: `go test -race` requires a cgo-capable C toolchain. On this machine the
> bundled C compiler cannot target 64-bit, so the race detector is unavailable;
> the suite is otherwise fully green with `go test ./...`.
