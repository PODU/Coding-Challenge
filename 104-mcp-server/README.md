# ccshared-mcp — Coding Challenges Shared Solutions MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server, written
in Go with a **hand-rolled JSON-RPC 2.0 stack (no SDK, no third-party deps)**. It
exposes tools and resources to an MCP client (Claude Desktop, the MCP Inspector,
or any LLM agent) over **stdio**.

The domain is the [Coding Challenges Shared Solutions](https://github.com/CodingChallengesFYI/SharedSolutions)
repository: an agent can say hello, search the shared `README.md` for challenge
solutions, and filter those solutions by programming language. The server itself
contains no LLM — it is a deterministic protocol endpoint that an LLM *calls*.

This is Coding Challenge [#104 "Build your own MCP server"](https://codingchallenges.fyi).

## Capabilities

**Tools** (`tools/list`, `tools/call`)

| Tool | Arguments | Behaviour |
|------|-----------|-----------|
| `hello` | `name` (required) | Returns `"Hello, <name>"`. |
| `CodingChallengesSolutionFinder` | `query` (required), `language` (optional) | Fetches the shared README, returns the solution links whose challenge matches `query`, optionally narrowed to `language`. |

**Resources** (`resources/list`, `resources/read`)

| URI | MIME | Content |
|-----|------|---------|
| `ccshared://readme` | `text/markdown` | The live shared README. |
| `mcp://server/info` | `application/json` | Server metadata. |

**Lifecycle:** `initialize` → `notifications/initialized`, `ping`, clean exit on EOF.

## Build & run

Requires Go 1.24+.

```sh
# From this directory:
go build -o mcp-server ./cmd/mcp-server      # produces a single static binary
# or run directly:
go run ./cmd/mcp-server
```

The server reads newline-delimited JSON-RPC 2.0 messages on **stdin** and writes
one JSON object per line on **stdout**. All logging goes to **stderr** so stdout
stays a clean protocol channel. It is normally launched as a subprocess by an MCP
host, not run interactively.

### Quick manual smoke test

```sh
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hello","arguments":{"name":"World"}}}' \
 | go run ./cmd/mcp-server
```

### Inspect with the MCP Inspector

```sh
go build -o mcp-server ./cmd/mcp-server
npx @modelcontextprotocol/inspector ./mcp-server
```

The Inspector drives the full handshake; from its UI you can list and call both
tools and read both resources.

## Add to an MCP client

The server is launched as a subprocess via a config entry. Use the **absolute
path** to the built binary (or to `go run`).

### Claude Desktop

Edit `claude_desktop_config.json`:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "ccshared": {
      "command": "C:\\path\\to\\104-mcp-server\\mcp-server.exe"
    }
  }
}
```

Or without pre-building, using the Go toolchain:

```json
{
  "mcpServers": {
    "ccshared": {
      "command": "go",
      "args": ["run", "./cmd/mcp-server"],
      "cwd": "C:\\path\\to\\104-mcp-server"
    }
  }
}
```

Restart Claude Desktop; the `hello` and `CodingChallengesSolutionFinder` tools
appear in the tools menu.

### Claude Code (CLI)

```sh
claude mcp add ccshared -- /absolute/path/to/mcp-server
```

## Error handling

- **Protocol errors** use JSON-RPC error objects: parse error (`-32700`), invalid
  request (`-32600`), method not found (`-32601`), invalid params / unknown tool /
  unknown resource URI (`-32602`), internal error (`-32603`), and server not
  initialised (`-32002`, when a tool/resource is called before `initialize`).
- **Tool failures** (GitHub unreachable, upstream format changed) are reported
  **in-band** as a normal `tools/call` result with `isError: true` and an
  explanatory text block, so the model can react. The server stays up.
- A no-match query returns a valid, non-error "no matching challenges" message.

## Project layout

```
104-mcp-server/
├── README.md
├── go.mod
├── cmd/mcp-server/main.go     # entrypoint: wire real fetcher, Serve over os.Stdin/Stdout
└── internal/mcp/
    ├── jsonrpc.go             # Request/Response/Error types, error codes, helpers
    ├── server.go             # Serve loop, dispatch, lifecycle state machine
    ├── tools.go              # tool registry, tools/list, tools/call, the 2 tools
    ├── resources.go          # resources/list, resources/read
    ├── fetcher.go            # Fetcher interface + GitHubFetcher (net/http, timeout, cache)
    ├── readme.go             # README markdown → solutions; query + language filter (pure)
    └── *_test.go             # in-process tests over pipes, offline (Fetcher mocked)
```

## Testing

All tests run **in-process over pipes with no MCP client and no network** — the
`Fetcher` is replaced by a fake returning a fixture README.

```sh
go vet ./...
go test ./...
```
