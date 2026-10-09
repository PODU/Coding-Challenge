import io
import json

from ccctx.mcp_server import TOOLS, Server, serve


def rpc(server, method, params=None, mid=1):
    msg = {"jsonrpc": "2.0", "id": mid, "method": method}
    if params is not None:
        msg["params"] = params
    return server.handle(msg)


def call(server, tool, **args):
    resp = rpc(server, "tools/call", {"name": tool, "arguments": args})
    return resp["result"]


def test_initialize_negotiates_version(indexed):
    s = Server(indexed)
    r = rpc(s, "initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                              "clientInfo": {"name": "t", "version": "1"}})["result"]
    assert r["protocolVersion"] == "2025-06-18"
    assert r["capabilities"] == {"tools": {"listChanged": False}}
    assert r["serverInfo"]["name"] == "ccctx" and r["instructions"]
    old = rpc(s, "initialize", {"protocolVersion": "2024-11-05"})["result"]
    assert old["protocolVersion"] == "2024-11-05"
    unknown = rpc(s, "initialize", {"protocolVersion": "1999-01-01"})["result"]
    assert unknown["protocolVersion"] == "2025-06-18"
    assert s.handle({"jsonrpc": "2.0", "method": "notifications/initialized"}) is None
    assert rpc(s, "ping")["result"] == {}


def test_tools_list_has_required_tools_with_guiding_descriptions(indexed):
    tools = rpc(Server(indexed), "tools/list")["result"]["tools"]
    names = {t["name"] for t in tools}
    assert {"search_code", "get_symbol", "find_callers", "impact_of", "read_wiki", "recall"} <= names
    for t in tools:
        assert len(t["description"]) > 80
        assert t["inputSchema"]["type"] == "object"
    search = next(t for t in TOOLS if t["name"] == "search_code")
    assert set(search["inputSchema"]["properties"]) >= {"query", "repo", "language", "k"}
    assert "FIRST" in search["description"]
    assert "get_symbol" in search["description"]  # steers the model to the follow-up tool


def test_search_code_tool(indexed):
    r = call(Server(indexed), "search_code", query="build_url", repo="sample", k=3)
    assert not r["isError"]
    data = r["structuredContent"]
    assert data["results"][0]["qualname"] == "build_url" and len(data["results"]) <= 3
    assert json.loads(r["content"][0]["text"]) == json.loads(json.dumps(data, default=str))


def test_get_symbol_tool_returns_source(indexed):
    data = call(Server(indexed), "get_symbol", name="Router.dispatch")["structuredContent"]
    m = data["matches"][0]
    assert m["source"].lstrip().startswith("def dispatch") and m["location"].endswith(":14")


def test_find_callers_and_impact_tools(indexed):
    s = Server(indexed)
    callers = call(s, "find_callers", symbol="make_response")["structuredContent"]
    assert {c["qualname"] for c in callers["callers"]} == {"not_found", "index", "profile"}
    impact = call(s, "impact_of", symbol="make_response", depth=2)["structuredContent"]
    assert impact["summary"]["by_hop"][1] == 3
    assert any(a["hops"] == 2 for a in impact["affected"])


def test_read_wiki_and_recall_tools(indexed):
    from ccctx import wiki
    wiki.build(indexed, "sample")
    s = Server(indexed)
    page = call(s, "read_wiki", slug="app")["structuredContent"]
    assert page["body"].startswith("# app") and page["stale"] is False
    assert call(s, "read_wiki")["structuredContent"]["pages"]
    rec = call(s, "recall", query="router")["structuredContent"]
    assert "facts" in rec and "recent_runs" in rec


def test_errors(indexed):
    s = Server(indexed)
    assert rpc(s, "nope")["error"]["code"] == -32601
    assert rpc(s, "tools/call", {"name": "nope", "arguments": {}})["error"]["code"] == -32602
    assert rpc(s, "tools/call", {"name": "get_symbol", "arguments": {}})["error"]["code"] == -32602
    bad_repo = call(s, "search_code", query="x", repo="missing")
    assert bad_repo["isError"] and "unknown repository" in bad_repo["content"][0]["text"]
    assert s.handle({"id": 3, "method": "ping"})["error"]["code"] == -32600


def test_serve_over_stdio_streams(indexed):
    lines = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18"}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
         "params": {"name": "search_code", "arguments": {"query": "router"}}},
    ]
    stdin = io.StringIO("\n".join(json.dumps(l) for l in lines) + "\nnot json\n")
    stdout = io.StringIO()
    serve(indexed, stdin, stdout)
    out = [json.loads(l) for l in stdout.getvalue().splitlines()]
    assert [o.get("id") for o in out] == [1, 2, None]
    assert out[2]["error"]["code"] == -32700
    assert out[1]["result"]["structuredContent"]["results"]


def test_batch_requests(indexed):
    stdin = io.StringIO(json.dumps([{"jsonrpc": "2.0", "id": 1, "method": "ping"},
                                    {"jsonrpc": "2.0", "id": 2, "method": "tools/list"}]) + "\n")
    stdout = io.StringIO()
    serve(indexed, stdin, stdout)
    batch = json.loads(stdout.getvalue())
    assert [b["id"] for b in batch] == [1, 2]
