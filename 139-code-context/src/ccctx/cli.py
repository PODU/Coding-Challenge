"""ccctx command-line interface."""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path

from . import __version__, evaluate, graph, memory, tools, wiki
from .ingest import ingest
from .search import search
from .store import Store


def _out(text: str = "") -> None:
    print(text)


def _json(data) -> None:
    print(json.dumps(data, indent=2, default=str))


def _store(args) -> Store:
    return Store(args.db) if args.db else Store()


def cmd_repo(args) -> int:
    with _store(args) as st:
        if args.repo_cmd == "add":
            path = Path(args.path).expanduser().resolve()
            if not path.is_dir():
                print(f"ccctx: {path} is not a directory", file=sys.stderr)
                return 2
            name = args.name or path.name
            if st.repo(name):
                print(f"ccctx: repository '{name}' already registered", file=sys.stderr)
                return 2
            st.add_repo(name, str(path))
            _out(f"Registered {name} -> {path}")
            _out(f"Next: ccctx ingest {name}")
        elif args.repo_cmd == "list":
            repos = st.repos()
            if args.json:
                _json(repos)
                return 0
            if not repos:
                _out("No repositories registered. Add one with: ccctx repo add <path> --name <name>")
                return 0
            _out(f"{'NAME':<16} {'FILES':>7} {'SYMBOLS':>9}  {'COMMIT':<10}  PATH")
            for r in repos:
                sha = (r["commit_sha"] or "-")[:10]
                _out(f"{r['name']:<16} {r['file_count']:>7,} {r['symbol_count']:>9,}  {sha:<10}  {r['path']}")
        elif args.repo_cmd == "remove":
            if not st.remove_repo(args.name):
                print(f"ccctx: unknown repository '{args.name}'", file=sys.stderr)
                return 1
            _out(f"Removed {args.name}")
    return 0


def cmd_ingest(args) -> int:
    with _store(args) as st:
        names = [r["name"] for r in st.repos()] if args.all else [args.name]
        if not names or names == [None]:
            print("ccctx: give a repository name or --all", file=sys.stderr)
            return 2
        for name in names:
            report = ingest(st, name, full=args.full, max_bytes=args.max_bytes)
            if len(names) > 1:
                _out(f"== {name}")
            _out(report.render())
            if args.verbose and not report.up_to_date:
                for p in report.added:
                    _out(f"  + {p}")
                for p in report.modified:
                    _out(f"  ~ {p}")
                for p in report.deleted:
                    _out(f"  - {p}")
                for a, b in report.renamed:
                    _out(f"  > {a} -> {b}")
            if not report.up_to_date:
                rebuilt = wiki.rebuild_stale(st, name)
                if rebuilt and rebuilt["built"]:
                    _out(f"Wiki     recompiled {len(rebuilt['built'])} stale page(s)")
    return 0


def cmd_symbols(args) -> int:
    with _store(args) as st:
        sql = ("SELECT s.name, s.qualname, s.kind, s.language, s.start_line, s.end_line, f.path, rp.name AS repo "
               "FROM symbols s JOIN files f ON f.id = s.file_id JOIN repos rp ON rp.id = s.repo_id WHERE 1=1")
        params: list = []
        for col, val in (("rp.name", args.repo), ("s.kind", args.kind), ("s.language", args.language)):
            if val:
                sql += f" AND {col} = ?"
                params.append(val)
        if args.file:
            sql += " AND f.path LIKE ?"
            params.append(f"%{args.file}%")
        if args.name:
            sql += " AND (s.name LIKE ? OR s.qualname LIKE ?)"
            params += [f"%{args.name}%"] * 2
        sql += " ORDER BY f.path, s.start_line LIMIT ?"
        params.append(args.limit)
        rows = [dict(r) for r in st.q(sql, params)]
        if args.json:
            _json(rows)
            return 0
        for r in rows:
            _out(f"{r['kind']:<9} {r['language']:<10} {r['qualname']:<40} {r['path']}:{r['start_line']}-{r['end_line']}")
    return 0


def cmd_langs(args) -> int:
    with _store(args) as st:
        sql = ("SELECT rp.name, f.language, f.parser, COUNT(DISTINCT f.id) AS files, COUNT(s.id) AS symbols "
               "FROM files f JOIN repos rp ON rp.id = f.repo_id LEFT JOIN symbols s ON s.file_id = f.id")
        params = []
        if args.repo:
            sql += " WHERE rp.name = ?"
            params.append(args.repo)
        sql += " GROUP BY rp.name, f.language, f.parser ORDER BY rp.name, files DESC"
        _out(f"{'REPO':<12} {'LANGUAGE':<12} {'PARSER':<11} {'FILES':>6} {'SYMBOLS':>8}")
        for r in st.q(sql, params):
            _out(f"{r[0]:<12} {r[1]:<12} {r[2]:<11} {r[3]:>6} {r[4]:>8}")
    return 0


def _print_results(results, show_scores: bool = True) -> None:
    if not results:
        _out("No results.")
        return
    for i, r in enumerate(results, 1):
        tags = []
        if r.get("similarity") is not None:
            tags.append(f"sim {r['similarity']:.2f}")
        if r.get("definition"):
            tags.append("definition")
        if r.get("boost"):
            tags.append(f"boost +{r['boost'] * 100:.0f}%")
        tag = f"  [{', '.join(tags)}]" if tags and show_scores else ""
        _out(f"{i:>2}. {r['score']:.4f}  {r['kind']:<8} {r['qualname']}{tag}")
        _out(f"      {r['location']}")


def cmd_search(args) -> int:
    with _store(args) as st:
        if args.compare:
            for mode in ("vector", "text", "hybrid"):
                res = search(st, args.query, mode=mode, repo=args.repo, language=args.language, k=args.k)
                _out(f"== {mode}")
                for i, r in enumerate(res, 1):
                    _out(f"{i:>2}. {r.kind:<8} {r.qualname:<42} {r.location}")
                if not res:
                    _out("   (no results)")
            return 0
        data = tools.search_code(st, args.query, args.repo, args.language, args.k, args.mode)
        if args.json:
            _json(data)
        else:
            _print_results(data["results"])
    return 0


def cmd_get(args) -> int:
    with _store(args) as st:
        data = tools.get_symbol(st, args.name, args.repo, args.limit)
        if args.json:
            _json(data)
            return 0
        if not data["matches"]:
            _out(f"No symbol named '{args.name}'.")
            return 1
        for m in data["matches"]:
            _out(f"## {m['qualname']} ({m['kind']}, {m['language']})  {m['location']}-{m['end_line']}")
            _out(m["source"])
            _out()
    return 0


def _print_hits(hits, show_hops=False) -> None:
    for h in hits:
        hop = f"[{h['hops']}] " if show_hops else ""
        _out(f"  {hop}{h['qualname']:<44} {h['kind']:<8} {h['location']}")


def cmd_callers(args) -> int:
    with _store(args) as st:
        data = tools.find_callers(st, args.symbol, args.repo)
        if args.json:
            _json(data)
            return 0
        _out(f"{len(data['callers'])} caller(s) of {args.symbol}:")
        _print_hits(data["callers"])
    return 0


def cmd_calls(args) -> int:
    with _store(args) as st:
        data = tools.find_calls(st, args.symbol, args.repo)
        if args.json:
            _json(data)
            return 0
        _out(f"{args.symbol} calls {len(data['calls'])} indexed symbol(s):")
        _print_hits(data["calls"])
        if data["external"]:
            _out(f"and {len(data['external'])} external/unresolved: {', '.join(data['external'])}")
    return 0


def cmd_impact(args) -> int:
    with _store(args) as st:
        data = tools.impact_of(st, args.symbol, args.repo, args.depth)
        if args.json:
            _json(data)
            return 0
        s = data["summary"]
        hops = ", ".join(f"{n} at hop {h}" for h, n in sorted(s["by_hop"].items()))
        _out(f"Changing {args.symbol} may affect {s['symbols']} symbol(s) in {s['files']} file(s)"
             + (f" ({hops})" if hops else "") + ":")
        _print_hits(data["affected"], show_hops=True)
    return 0


def cmd_mcp(args) -> int:
    from .mcp_server import serve
    for stream in (sys.stdin, sys.stdout):
        try:
            stream.reconfigure(encoding="utf-8", newline="\n")
        except (AttributeError, ValueError):
            pass
    with _store(args) as st:
        serve(st)
    return 0


def cmd_wiki(args) -> int:
    with _store(args) as st:
        if args.wiki_cmd == "build":
            repos = [args.repo] if args.repo else [r["name"] for r in st.repos()]
            for name in repos:
                res = wiki.build(st, name, force=args.force)
                _out(f"{name}: built {len(res['built'])}, fresh {len(res['fresh'])}, removed {len(res['removed'])}")
                for slug in res["built"]:
                    _out(f"  + {slug}")
        elif args.wiki_cmd == "list":
            pages = wiki.list_pages(st, args.repo)
            if not pages:
                _out("No wiki pages. Build them with: ccctx wiki build --repo <name>")
            for p in pages:
                _out(f"{p['repo']:<12} {p['slug']:<32} {'STALE' if p['stale'] else 'fresh'}")
        elif args.wiki_cmd == "show":
            data = tools.read_wiki(st, args.slug, args.repo)
            if "error" in data:
                print(f"ccctx: {data['error']}", file=sys.stderr)
                return 1
            if data["stale"]:
                _out(f"> STALE: {'; '.join(data['stale_reasons'][:3])}\n")
            _out(data["body"])
        elif args.wiki_cmd == "citations":
            page = wiki.find_page(st, args.slug, args.repo)
            if page is None:
                print(f"ccctx: no wiki page '{args.slug}'", file=sys.stderr)
                return 1
            cites = wiki.citations(st, page)
            if args.json:
                _json(cites)
                return 0
            _out(f"{len(cites)} citation(s) for {page['slug']} (built at commit {(page['commit_sha'] or '-')[:10]}):")
            for c in cites:
                _out(f"  {c['status']:<8} {c['role']:<6} #{c['symbol_id'] or '-':<6} {c['symbol_key']}")
    return 0


def cmd_memory(args) -> int:
    with _store(args) as st:
        if args.memory_cmd == "list":
            runs = memory.list_runs(st, args.repo, args.limit)
            if args.json:
                _json(runs)
                return 0
            import datetime as dt
            for r in runs:
                when = dt.datetime.fromtimestamp(r["ts"]).strftime("%Y-%m-%d %H:%M:%S")
                top = r["results"][0] if r["results"] else "-"
                _out(f"#{r['id']:<5} {when}  {r['tool']:<13} {r['repo'] or '*':<8} "
                     f"{(r['query'] or '')[:40]!r:<44} {r['result_count']:>3} -> {top}")
        elif args.memory_cmd == "consolidate":
            repos = [args.repo] if args.repo else [r["name"] for r in st.repos()]
            for name in repos:
                n = memory.consolidate(st, name)
                _out(f"{name}: {n} fact(s) written")
        elif args.memory_cmd == "facts":
            facts = memory.list_facts(st, args.repo, args.limit)
            if args.json:
                _json(facts)
                return 0
            if not facts:
                _out("No facts yet. Use the tools, then run: ccctx memory consolidate")
            for f in facts:
                _out(f"{f['confidence']:.2f}  {f['predicate']:<15} {f['assertion']}  (runs {f['source_runs'][:5]})")
        elif args.memory_cmd == "recall":
            data = tools.recall(st, args.query, args.repo, args.limit)
            _json(data) if args.json else [_out(f"{f['relevance']:.2f}  {f['assertion']}") for f in data["facts"]]
    return 0


def cmd_feedback(args) -> int:
    with _store(args) as st:
        hits = graph.resolve(st, args.symbol, args.repo)
        if not hits:
            print(f"ccctx: no symbol '{args.symbol}'", file=sys.stderr)
            return 1
        for h in hits[: args.limit]:
            memory.record_feedback(st, h.repo, h.key, args.kind)
            _out(f"recorded {args.kind} for {h.key}")
    return 0


def cmd_eval(args) -> int:
    with _store(args) as st:
        report = evaluate.evaluate(st, args.gold, with_feedback=args.with_feedback, k=args.k)
        if args.json:
            _json({"k": report["k"], "feedback_events": report.get("feedback_events"),
                   "modes": {m: v.summary() | {"questions_detail": v.per_question}
                             for m, v in report["modes"].items()}})
            return 0
        _out(evaluate.render(report))
        if args.verbose:
            for mode, m in report["modes"].items():
                _out(f"\n== {mode}")
                for q in m.per_question:
                    _out(f"  {str(q.get('rank', q.get('results'))):>4}  {q['q']}")
    return 0


def cmd_oracle(args) -> int:
    from . import oracle
    _out(oracle.script(args.part))
    return 0


def cmd_stats(args) -> int:
    with _store(args) as st:
        out = {}
        for t in ("repos", "files", "symbols", "calls", "imports", "wiki_pages", "wiki_citations", "runs",
                  "facts", "feedback"):
            out[t] = st.q1(f"SELECT COUNT(*) FROM {t}")[0]
        out["kinds"] = dict(Counter(r[0] for r in st.q("SELECT kind FROM symbols")).most_common())
        out["db"] = str(st.path)
        _json(out)
    return 0


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="ccctx", description="Code Context: index code for AI tools (MCP).")
    p.add_argument("--db", help="database file (default: $CCCTX_HOME/ccctx.db or ~/.ccctx/ccctx.db)")
    p.add_argument("--version", action="version", version=f"ccctx {__version__}")
    sub = p.add_subparsers(dest="cmd", required=True)

    def add(name, fn, help_):
        sp = sub.add_parser(name, help=help_)
        sp.set_defaults(fn=fn)
        return sp

    rp = add("repo", cmd_repo, "register / list / remove repositories")
    rsub = rp.add_subparsers(dest="repo_cmd", required=True)
    a = rsub.add_parser("add")
    a.add_argument("path")
    a.add_argument("--name")
    l = rsub.add_parser("list")
    l.add_argument("--json", action="store_true")
    r = rsub.add_parser("remove")
    r.add_argument("name")

    i = add("ingest", cmd_ingest, "index (or incrementally re-index) a repository")
    i.add_argument("name", nargs="?")
    i.add_argument("--full", action="store_true", help="drop and rebuild the repository's index")
    i.add_argument("--all", action="store_true", help="ingest every registered repository")
    i.add_argument("--max-bytes", type=int, default=1_000_000, help="skip files larger than this")
    i.add_argument("-v", "--verbose", action="store_true", help="list changed files")

    s = add("symbols", cmd_symbols, "list indexed symbols")
    s.add_argument("--repo")
    s.add_argument("--kind")
    s.add_argument("--language")
    s.add_argument("--file")
    s.add_argument("--name")
    s.add_argument("--limit", type=int, default=20)
    s.add_argument("--json", action="store_true")

    lg = add("langs", cmd_langs, "files and symbols per detected language")
    lg.add_argument("--repo")

    q = add("search", cmd_search, "search code (hybrid by default)")
    q.add_argument("query")
    q.add_argument("--mode", choices=["hybrid", "vector", "text"], default="hybrid")
    q.add_argument("--repo")
    q.add_argument("--language")
    q.add_argument("-k", type=int, default=10)
    q.add_argument("--compare", action="store_true", help="show vector, text and hybrid side by side")
    q.add_argument("--json", action="store_true")

    g = add("get", cmd_get, "print a symbol's full source")
    g.add_argument("name")
    g.add_argument("--repo")
    g.add_argument("--limit", type=int, default=5)
    g.add_argument("--json", action="store_true")

    for name, fn, help_ in (("callers", cmd_callers, "who calls a symbol"),
                            ("calls", cmd_calls, "what a symbol calls")):
        c = add(name, fn, help_)
        c.add_argument("symbol")
        c.add_argument("--repo")
        c.add_argument("--json", action="store_true")
    im = add("impact", cmd_impact, "blast radius of changing a symbol")
    im.add_argument("symbol")
    im.add_argument("--repo")
    im.add_argument("--depth", type=int, default=3)
    im.add_argument("--json", action="store_true")

    add("mcp", cmd_mcp, "run the MCP server on stdio")

    w = add("wiki", cmd_wiki, "generated module wiki")
    wsub = w.add_subparsers(dest="wiki_cmd", required=True)
    wb = wsub.add_parser("build")
    wb.add_argument("--repo")
    wb.add_argument("--force", action="store_true")
    wl = wsub.add_parser("list")
    wl.add_argument("--repo")
    for name in ("show", "citations"):
        ws = wsub.add_parser(name)
        ws.add_argument("slug")
        ws.add_argument("--repo")
        ws.add_argument("--json", action="store_true")

    m = add("memory", cmd_memory, "episodic runs and distilled facts")
    msub = m.add_subparsers(dest="memory_cmd", required=True)
    for name in ("list", "consolidate", "facts"):
        ms = msub.add_parser(name)
        ms.add_argument("--repo")
        ms.add_argument("--limit", type=int, default=10 if name == "list" else 50)
        ms.add_argument("--json", action="store_true")
    mr = msub.add_parser("recall")
    mr.add_argument("query")
    mr.add_argument("--repo")
    mr.add_argument("--limit", type=int, default=5)
    mr.add_argument("--json", action="store_true")

    fb = add("feedback", cmd_feedback, "record that a symbol was useful")
    fb.add_argument("symbol")
    fb.add_argument("--repo")
    fb.add_argument("--kind", default="cite")
    fb.add_argument("--limit", type=int, default=1)

    e = add("eval", cmd_eval, "evaluate retrieval against a gold question set")
    e.add_argument("--gold", required=True)
    e.add_argument("--with-feedback", action="store_true")
    e.add_argument("-k", type=int)
    e.add_argument("--json", action="store_true")
    e.add_argument("-v", "--verbose", action="store_true")

    o = add("oracle", cmd_oracle, "print the Oracle AI Database 26ai SQL for this schema")
    o.add_argument("part", nargs="?", default="all", choices=["all", "schema", "model", "search", "graph"])

    add("stats", cmd_stats, "table counts")
    return p


def main(argv=None) -> int:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(errors="replace")
        except (AttributeError, ValueError):
            pass
    args = build_parser().parse_args(argv)
    try:
        return args.fn(args) or 0
    except (KeyError, FileNotFoundError, ValueError) as e:
        msg = e.args[0] if e.args else str(e)
        print(f"ccctx: {msg}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
