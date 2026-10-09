"""Model Context Protocol server over stdio (JSON-RPC 2.0, newline-delimited).

Implements the lifecycle (initialize / notifications/initialized / ping) and
the tools capability (tools/list, tools/call) by hand, without an SDK.
Logs go to stderr; stdout carries only protocol messages.
"""

from __future__ import annotations

import json
import sys
import traceback

from . import __version__, tools
from .store import Store

SUPPORTED_VERSIONS = ("2025-06-18", "2025-03-26", "2024-11-05")

INSTRUCTIONS = (
    "ccctx indexes source repositories. Start with search_code for any question about where or how "
    "something is done; use get_symbol to read the full source of a result before relying on it; use "
    "find_callers / impact_of before changing a function to see who depends on it; use read_wiki for "
    "module-level overviews; use recall at the start of a task to see facts learned from earlier sessions."
)

_REPO = {"type": "string", "description": "Restrict to one registered repository (see `ccctx repo list`)."}

TOOLS = [
    {
        "name": "search_code",
        "title": "Search code",
        "description": (
            "Find code by meaning or by name across indexed repositories. Use this FIRST for questions like "
            "'where is X handled', 'how does Y work', or to locate an identifier. Combines semantic (embedding) "
            "and keyword search; exact identifier definitions rank first, and generated wiki pages are included. "
            "Returns ranked symbols (functions, methods, classes) with file paths, line ranges and a short "
            "snippet - not full source; call get_symbol for that."),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Natural-language question or identifier."},
                "repo": _REPO,
                "language": {"type": "string", "description": "e.g. python, go, javascript, typescript, java, rust"},
                "k": {"type": "integer", "minimum": 1, "maximum": 50, "default": 10,
                      "description": "Number of results."},
                "mode": {"type": "string", "enum": ["hybrid", "vector", "text"], "default": "hybrid"},
            },
            "required": ["query"],
        },
        "annotations": {"readOnlyHint": True, "openWorldHint": False},
    },
    {
        "name": "get_symbol",
        "title": "Get symbol source",
        "description": (
            "Return the full source code, signature, docstring and location of a named symbol. Use after "
            "search_code to read a result, or directly when you already know the name. Accepts a bare name "
            "('url_for'), a qualified name ('Flask.url_for') or a symbol key from a search result. Calling this "
            "tells ccctx the result was useful, which improves future rankings."),
        "inputSchema": {
            "type": "object",
            "properties": {"name": {"type": "string"}, "repo": _REPO,
                           "limit": {"type": "integer", "minimum": 1, "maximum": 20, "default": 5}},
            "required": ["name"],
        },
        "annotations": {"readOnlyHint": False, "openWorldHint": False},
    },
    {
        "name": "find_callers",
        "title": "Find callers",
        "description": (
            "List every function or method that calls the named symbol (one hop, via the call graph). Use to "
            "see how an API is used or to find call sites before editing a signature. Calls are matched by name, "
            "so same-named methods on different types are grouped together."),
        "inputSchema": {"type": "object", "properties": {"symbol": {"type": "string"}, "repo": _REPO},
                        "required": ["symbol"]},
        "annotations": {"readOnlyHint": True, "openWorldHint": False},
    },
    {
        "name": "impact_of",
        "title": "Change impact (blast radius)",
        "description": (
            "Estimate what could break if the named symbol changes: all symbols that reach it through up to "
            "`depth` call hops, with the hop count for each, plus a summary of affected files. Use before "
            "refactoring or changing behaviour; prefer find_callers when you only need direct call sites."),
        "inputSchema": {"type": "object", "properties": {
            "symbol": {"type": "string"}, "repo": _REPO,
            "depth": {"type": "integer", "minimum": 1, "maximum": 8, "default": 3}},
            "required": ["symbol"]},
        "annotations": {"readOnlyHint": True, "openWorldHint": False},
    },
    {
        "name": "read_wiki",
        "title": "Read module wiki",
        "description": (
            "Read a generated wiki page describing a module: purpose, key symbols, entry points and "
            "dependencies, with citations and a staleness flag. Call with no slug to list pages. Use for "
            "orientation in an unfamiliar area before diving into individual symbols."),
        "inputSchema": {"type": "object", "properties": {
            "slug": {"type": "string", "description": "Page slug such as 'flask/json'; omit to list pages."},
            "repo": _REPO}},
        "annotations": {"readOnlyHint": True, "openWorldHint": False},
    },
    {
        "name": "recall",
        "title": "Recall memory",
        "description": (
            "Return durable facts learned from earlier sessions (which symbols answered similar questions, "
            "frequently used code, known gaps) plus the most recent tool runs. Call at the start of a task."),
        "inputSchema": {"type": "object", "properties": {
            "query": {"type": "string"}, "repo": _REPO,
            "limit": {"type": "integer", "minimum": 1, "maximum": 50, "default": 5}},
            "required": ["query"]},
        "annotations": {"readOnlyHint": True, "openWorldHint": False},
    },
]

_DISPATCH = {
    "search_code": lambda s, a: tools.search_code(s, a["query"], a.get("repo"), a.get("language"),
                                                  a.get("k", 10), a.get("mode", "hybrid")),
    "get_symbol": lambda s, a: tools.get_symbol(s, a["name"], a.get("repo"), a.get("limit", 5)),
    "find_callers": lambda s, a: tools.find_callers(s, a["symbol"], a.get("repo")),
    "impact_of": lambda s, a: tools.impact_of(s, a["symbol"], a.get("repo"), a.get("depth", 3)),
    "read_wiki": lambda s, a: tools.read_wiki(s, a.get("slug"), a.get("repo")),
    "recall": lambda s, a: tools.recall(s, a["query"], a.get("repo"), a.get("limit", 5)),
}


class Server:
    def __init__(self, store: Store):
        self.store = store
        self.initialized = False

    def handle(self, msg: dict) -> dict | None:
        """Handle one JSON-RPC message; returns the response (None for notifications)."""
        mid = msg.get("id")
        method = msg.get("method")
        is_request = "id" in msg
        if msg.get("jsonrpc") != "2.0" or not isinstance(method, str):
            return _error(mid, -32600, "Invalid Request") if is_request else None
        params = msg.get("params") or {}
        try:
            if method == "initialize":
                requested = params.get("protocolVersion")
                version = requested if requested in SUPPORTED_VERSIONS else SUPPORTED_VERSIONS[0]
                result = {"protocolVersion": version,
                          "capabilities": {"tools": {"listChanged": False}},
                          "serverInfo": {"name": "ccctx", "title": "Code Context", "version": __version__},
                          "instructions": INSTRUCTIONS}
            elif method == "notifications/initialized":
                self.initialized = True
                return None
            elif method.startswith("notifications/"):
                return None
            elif method == "ping":
                result = {}
            elif method == "tools/list":
                result = {"tools": TOOLS}
            elif method == "tools/call":
                result = self.call_tool(params.get("name"), params.get("arguments") or {})
            else:
                return _error(mid, -32601, f"Method not found: {method}") if is_request else None
        except _ParamsError as e:
            return _error(mid, -32602, str(e))
        except Exception as e:  # pragma: no cover - defensive
            traceback.print_exc(file=sys.stderr)
            return _error(mid, -32603, f"Internal error: {e}")
        return {"jsonrpc": "2.0", "id": mid, "result": result} if is_request else None

    def call_tool(self, name, args: dict) -> dict:
        if name not in _DISPATCH:
            raise _ParamsError(f"Unknown tool: {name}")
        schema = next(t for t in TOOLS if t["name"] == name)["inputSchema"]
        missing = [p for p in schema.get("required", []) if p not in args]
        if missing:
            raise _ParamsError(f"Missing required argument(s): {', '.join(missing)}")
        try:
            data = _DISPATCH[name](self.store, args)
        except (KeyError, ValueError) as e:
            msg = e.args[0] if e.args else str(e)
            return {"content": [{"type": "text", "text": f"Error: {msg}"}], "isError": True}
        return {"content": [{"type": "text", "text": json.dumps(data, indent=2, default=str)}],
                "structuredContent": data, "isError": bool(isinstance(data, dict) and data.get("error"))}


class _ParamsError(Exception):
    pass


def _error(mid, code: int, message: str) -> dict:
    return {"jsonrpc": "2.0", "id": mid, "error": {"code": code, "message": message}}


def serve(store: Store, stdin=None, stdout=None) -> None:
    stdin = stdin or sys.stdin
    stdout = stdout or sys.stdout
    server = Server(store)
    print(f"ccctx MCP server {__version__} ready (db: {store.path})", file=sys.stderr)
    for line in stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            resp = _error(None, -32700, "Parse error")
        else:
            if isinstance(msg, list):  # batch (2025-03-26)
                resps = [r for r in (server.handle(m) for m in msg if isinstance(m, dict)) if r]
                if resps:
                    stdout.write(json.dumps(resps) + "\n")
                    stdout.flush()
                continue
            resp = server.handle(msg) if isinstance(msg, dict) else _error(None, -32600, "Invalid Request")
        if resp is not None:
            stdout.write(json.dumps(resp, default=str) + "\n")
            stdout.flush()
