"""The ``wiki`` command line and chat REPL.

Backends are picked from environment variables (see config.py); the defaults
need no external services. ``main(argv)`` returns the exit code.
"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Sequence

from .config import build_wiki
from .frontmatter import to_markdown
from .wiki import ProjectNotFound, Wiki


def _print_json(obj) -> None:
    print(json.dumps(obj, indent=2, ensure_ascii=False))


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="wiki", description="Database-Driven LLM Wiki")
    sub = p.add_subparsers(dest="cmd", required=True)

    proj = sub.add_parser("project", help="manage projects")
    proj_sub = proj.add_subparsers(dest="action", required=True)
    proj_sub.add_parser("list")
    pc = proj_sub.add_parser("create"); pc.add_argument("name")
    pd = proj_sub.add_parser("delete"); pd.add_argument("name"); pd.add_argument("--force", action="store_true")

    ing = sub.add_parser("ingest", help="ingest file(s)/dir into a project")
    ing.add_argument("name"); ing.add_argument("paths", nargs="+"); ing.add_argument("--json", action="store_true")

    ask = sub.add_parser("ask", help="ask a question")
    ask.add_argument("name"); ask.add_argument("question")
    ask.add_argument("--save", action="store_true"); ask.add_argument("--k", type=int, default=6)
    ask.add_argument("--json", action="store_true")

    lint = sub.add_parser("lint", help="health-check a project")
    lint.add_argument("name"); lint.add_argument("--fix", action="store_true"); lint.add_argument("--json", action="store_true")

    br = sub.add_parser("browse", help="list pages")
    br.add_argument("name"); br.add_argument("--type", choices=["entity", "topic", "summary", "answer"])
    br.add_argument("--json", action="store_true")

    pg = sub.add_parser("page", help="print a page")
    pg.add_argument("name"); pg.add_argument("page_id")

    lg = sub.add_parser("log", help="show the append-only operation log")
    lg.add_argument("name"); lg.add_argument("--json", action="store_true")

    ch = sub.add_parser("chat", help="interactive chat REPL"); ch.add_argument("name")
    sv = sub.add_parser("serve", help="optional web chat (requires [web])"); sv.add_argument("--port", type=int, default=8000)
    return p


def main(argv: Sequence[str] | None = None, wiki: Wiki | None = None) -> int:
    args = _build_parser().parse_args(argv)
    wiki = wiki or build_wiki()
    try:
        return _dispatch(args, wiki)
    except ProjectNotFound as e:
        print(f"error: project not found: {e}", file=sys.stderr)
        return 2
    except Exception as e:
        print(f"error: {e}", file=sys.stderr)
        return 1


def _dispatch(args, wiki: Wiki) -> int:
    if args.cmd == "project":
        return _cmd_project(args, wiki)
    if args.cmd == "ingest":
        res = wiki.ingest(args.name, args.paths[0]) if len(args.paths) == 1 else _ingest_many(wiki, args)
        if args.json:
            _print_json(res.to_dict())
        else:
            print(f"created={len(res.pages_created)} updated={len(res.pages_updated)} "
                  f"contradictions={len(res.contradictions)} skipped={len(res.skipped)} ops={res.ops}")
        return 0
    if args.cmd == "ask":
        ans = wiki.ask(args.name, args.question, k=args.k, save=args.save)
        if args.json:
            _print_json(ans.to_dict())
        else:
            print(ans.answer)
            for c in ans.citations:
                print(f"  ^{c['marker']} {c['page_id']} ({c.get('source_id')})")
            if ans.saved_as:
                print(f"saved as: {ans.saved_as}")
        return 0
    if args.cmd == "lint":
        if args.fix:
            actions = wiki.lint_fix(args.name)
            if args.json:
                _print_json({"actions": actions})
            else:
                print("\n".join(actions) or "no fixes applied")
            return 0
        findings = wiki.lint(args.name)
        if args.json:
            _print_json([f.to_dict() for f in findings])
        else:
            for f in findings:
                print(f"[{f.severity}] {f.kind}: {f.message}")
            print(f"{len(findings)} finding(s)")
        return 0
    if args.cmd == "browse":
        pages = wiki.browse(args.name, type=args.type)
        if args.json:
            _print_json([{"id": p.id, "type": p.type, "title": p.frontmatter.title} for p in pages])
        else:
            for p in pages:
                print(f"{p.type:8} {p.id}  —  {p.frontmatter.title}")
        return 0
    if args.cmd == "page":
        page = wiki.get_page(args.name, args.page_id)
        if page is None:
            print(f"error: page not found: {args.page_id}", file=sys.stderr)
            return 2
        print(to_markdown(page))
        return 0
    if args.cmd == "log":
        ops = wiki.read_log(args.name)
        if args.json:
            _print_json([o.to_dict() for o in ops])
        else:
            for o in ops:
                print(f"{o.seq:4} {o.ts} {o.op} {o.page_id or ''}")
        return 0
    if args.cmd == "chat":
        return _cmd_chat(args, wiki)
    if args.cmd == "serve":
        return _cmd_serve(args, wiki)
    return 1


def _ingest_many(wiki: Wiki, args):
    from .domain import IngestResult

    agg = IngestResult()
    for path in args.paths:
        Wiki._merge_result(agg, wiki.ingest(args.name, path))
    return agg


def _cmd_project(args, wiki: Wiki) -> int:
    if args.action == "list":
        for name in wiki.list_projects():
            print(name)
        return 0
    if args.action == "create":
        wiki.create_project(args.name)
        print(f"created project '{args.name}'")
        return 0
    if args.action == "delete":
        if not args.force:
            print("refusing to delete without --force", file=sys.stderr)
            return 1
        wiki.delete_project(args.name)
        print(f"deleted project '{args.name}'")
        return 0
    return 1


def _cmd_chat(args, wiki: Wiki) -> int:  # pragma: no cover - interactive
    print(f"Chat with project '{args.name}'. Type /quit to exit.")
    while True:
        try:
            line = input("> ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            return 0
        if line in {"/quit", "/exit"}:
            return 0
        if not line:
            continue
        ans = wiki.ask(args.name, line)
        print(ans.answer)
        for c in ans.citations:
            print(f"  ^{c['marker']} {c['page_id']}")


def _cmd_serve(args, wiki: Wiki) -> int:  # pragma: no cover - optional web
    try:
        from .web import run_server
    except Exception:
        print("error: web extra not installed (pip install 'llmwiki[web]')", file=sys.stderr)
        return 1
    run_server(wiki, port=args.port)
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
