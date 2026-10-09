"""The tool surface shared by the CLI and the MCP server.

Every call is recorded as an episodic-memory run; get_symbol also records
usage feedback for the symbols it returns (an agent opening a result is the
strongest signal that the result was useful).
"""

from __future__ import annotations

import time

from . import graph, memory, wiki
from .search import search
from .store import Store

MAX_SOURCE_LINES = 400


def _timed(fn):
    t0 = time.perf_counter()
    out = fn()
    return out, (time.perf_counter() - t0) * 1000.0


def search_code(store: Store, query: str, repo: str | None = None, language: str | None = None,
                k: int = 10, mode: str = "hybrid", record: bool = True) -> dict:
    k = max(1, min(int(k), 50))
    results, ms = _timed(lambda: search(store, query, mode=mode, repo=repo, language=language, k=k))
    keys = [r.key if r.doc == "symbol" else f"wiki:{r.key}" for r in results]
    run_id = memory.record_run(store, "search_code", repo, query,
                               {"language": language, "k": k, "mode": mode}, keys, ms) if record else None
    return {"query": query, "mode": mode, "run_id": run_id,
            "results": [r.to_dict() | {"location": r.location} for r in results]}


def get_symbol(store: Store, name: str, repo: str | None = None, limit: int = 5) -> dict:
    t0 = time.perf_counter()
    hits = graph.resolve(store, name, repo)
    if not hits:
        # Fall back to a case-insensitive / suffix match so near-misses still resolve.
        hits = [h for h in _fuzzy(store, name, repo)]
    out = []
    for h in hits[:limit]:
        row = store.q1("SELECT s.source, s.signature, s.doc, s.language FROM symbols s WHERE s.key = ? "
                       "AND s.repo_id = (SELECT id FROM repos WHERE name = ?)", (h.key, h.repo))
        lines = row["source"].splitlines()
        source = "\n".join(lines[:MAX_SOURCE_LINES])
        if len(lines) > MAX_SOURCE_LINES:
            source += f"\n... ({len(lines) - MAX_SOURCE_LINES} more lines)"
        out.append(h.to_dict() | {"location": h.location, "language": row["language"],
                                  "signature": row["signature"], "doc": row["doc"], "source": source})
    ms = (time.perf_counter() - t0) * 1000.0
    run_id = memory.record_run(store, "get_symbol", repo or (out[0]["repo"] if out else None), name,
                               {"limit": limit}, [o["key"] for o in out], ms)
    for o in out:
        memory.record_feedback(store, o["repo"], o["key"], "open")
    return {"symbol": name, "run_id": run_id, "matches": out}


def _fuzzy(store: Store, name: str, repo: str | None):
    params: list = [name.lower(), "%." + name.lower()]
    sql = (graph._HIT_SQL + "WHERE (lower(s.name) = ? OR lower(s.qualname) LIKE ?) AND s.kind != 'chunk'")
    if repo:
        sql += " AND rp.name = ?"
        params.append(repo)
    return [graph.GraphHit(*r, hops=0) for r in store.q(sql + " LIMIT 10", params)]


def find_callers(store: Store, symbol: str, repo: str | None = None) -> dict:
    (defs, hits), ms = _timed(lambda: (graph.resolve(store, symbol, repo), graph.callers(store, symbol, repo)))
    memory.record_run(store, "find_callers", repo, symbol, {}, [d.key for d in defs] + [h.key for h in hits], ms)
    return {"symbol": symbol, "definitions": [d.to_dict() for d in defs],
            "callers": [h.to_dict() | {"location": h.location} for h in hits]}


def find_calls(store: Store, symbol: str, repo: str | None = None) -> dict:
    (resolved, external), ms = _timed(lambda: graph.calls(store, symbol, repo))
    memory.record_run(store, "find_calls", repo, symbol, {}, [h.key for h in resolved], ms)
    return {"symbol": symbol, "calls": [h.to_dict() | {"location": h.location} for h in resolved],
            "external": external}


def impact_of(store: Store, symbol: str, repo: str | None = None, depth: int = 3) -> dict:
    depth = max(1, min(int(depth), 8))
    (defs, hits), ms = _timed(lambda: (graph.resolve(store, symbol, repo),
                                       graph.impact(store, symbol, repo, depth)))
    memory.record_run(store, "impact_of", repo, symbol, {"depth": depth},
                      [d.key for d in defs] + [h.key for h in hits], ms)
    by_hop: dict[int, int] = {}
    for h in hits:
        by_hop[h.hops] = by_hop.get(h.hops, 0) + 1
    files = sorted({f"{h.repo}/{h.path}" for h in hits})
    return {"symbol": symbol, "depth": depth, "definitions": [d.to_dict() for d in defs],
            "affected": [h.to_dict() | {"location": h.location} for h in hits],
            "summary": {"symbols": len(hits), "files": len(files), "by_hop": by_hop}}


def read_wiki(store: Store, slug: str | None = None, repo: str | None = None) -> dict:
    t0 = time.perf_counter()
    if not slug:
        pages = wiki.list_pages(store, repo)
        memory.record_run(store, "read_wiki", repo, None, {}, [], (time.perf_counter() - t0) * 1000.0)
        return {"pages": pages}
    page = wiki.find_page(store, slug, repo)
    if page is None:
        return {"error": f"no wiki page '{slug}'", "pages": [p["slug"] for p in wiki.list_pages(store, repo)]}
    st = wiki.page_status(store, page)
    memory.record_run(store, "read_wiki", page["repo_name"], slug, {}, [f"wiki:{page['slug']}"],
                      (time.perf_counter() - t0) * 1000.0)
    return {"repo": page["repo_name"], "slug": page["slug"], "title": page["title"], "body": page["body"],
            "stale": st.stale, "stale_reasons": st.reasons, "commit_sha": page["commit_sha"],
            "citations": wiki.citations(store, page)}


def recall(store: Store, query: str, repo: str | None = None, limit: int = 5) -> dict:
    out, ms = _timed(lambda: memory.recall(store, query, repo, limit))
    memory.record_run(store, "recall", repo, query, {"limit": limit}, [], ms)
    return out
