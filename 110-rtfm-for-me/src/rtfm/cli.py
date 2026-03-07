"""Command-line interface: ``rtfm ingest|ask|serve|reset``.

Without ``--redis`` nothing survives between runs, so a separate ``ingest``
followed by ``ask`` won't work; use ``ask --ingest ...`` or Redis.
"""

from __future__ import annotations

import argparse
import sys

from .config import Config
from .factory import build_agent


def _print_answer(result) -> None:
    ans = result.answer
    print(ans.text)
    if ans.citations:
        print("\nSources:")
        for cite in ans.citations:
            print(f"  - {cite.label}")
    flags = []
    if result.cached:
        flags.append("cache hit")
    if result.degraded:
        flags.append("degraded: " + ", ".join(result.degraded))
    if flags:
        print(f"\n[{'; '.join(flags)}]")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="rtfm", description="RTFM For Me Agent (RAG over docs).")
    parser.add_argument("--redis", action="store_true", help="Use Redis Stack backends.")
    parser.add_argument(
        "--mock-llm",
        action="store_true",
        help="Use the scripted mock LLM (no ANTHROPIC_API_KEY needed).",
    )
    parser.add_argument(
        "--local-embedder",
        action="store_true",
        help="Use the local offline embedding model instead of the hash embedder.",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    p_ingest = sub.add_parser("ingest", help="Ingest documentation files/dirs.")
    p_ingest.add_argument("paths", nargs="+", help="Files or directories to ingest.")
    p_ingest.add_argument("ask", nargs="?", help=argparse.SUPPRESS)

    p_ask = sub.add_parser("ask", help="Ask a question (ingest in the same call with --ingest).")
    p_ask.add_argument("question", help="The question to ask.")
    p_ask.add_argument("--ingest", nargs="+", default=None, help="Files/dirs to ingest first.")
    p_ask.add_argument("--session", default=None, help="Session id for short-term memory.")
    p_ask.add_argument("--user", default=None, help="User id for long-term memory.")
    p_ask.add_argument("--file", default=None, help="Restrict retrieval to one filename.")

    sub.add_parser("serve", help="Run the REST API (requires the 'api' extra).")
    sub.add_parser("reset", help="Clear the index, cache and memory (Redis only).")

    args = parser.parse_args(argv)
    cfg = Config.from_env()

    if args.command == "serve":
        return _serve(cfg, args)

    agent = build_agent(
        cfg,
        use_redis=args.redis,
        mock_llm=args.mock_llm,
        local_embedder=args.local_embedder,
    )

    if args.command == "ingest":
        report = _ingest(agent, cfg, args.paths)
        print(f"Ingested {report.chunks_added} chunks from {len(report.files_ok)} file(s).")
        for path, err in report.files_failed:
            print(f"  ! {path}: {err}", file=sys.stderr)
        return 0 if report.ok else 1

    if args.command == "ask":
        if args.ingest:
            _ingest(agent, cfg, args.ingest)
        result = agent.ask(
            args.question,
            session_id=args.session,
            user_id=args.user,
            filename=args.file,
        )
        _print_answer(result)
        return 0

    if args.command == "reset":
        agent.reset()
        print("Reset complete.")
        return 0

    return 1  # pragma: no cover - argparse guards this


def _ingest(agent, cfg: Config, paths: list[str]):
    from .ingest import ingest_paths

    return ingest_paths(paths, embedder=agent.embedder, store=agent.store, cfg=cfg)


def _serve(cfg: Config, args) -> int:
    try:
        import uvicorn  # type: ignore

        from .api import create_app
    except Exception as exc:  # pragma: no cover - requires the 'api' extra
        print(f"Cannot serve: {exc}. Install with: pip install 'rtfm[api]'", file=sys.stderr)
        return 1
    app = create_app(  # pragma: no cover - exercised under uvicorn
        use_redis=args.redis, mock_llm=args.mock_llm, local_embedder=args.local_embedder
    )
    uvicorn.run(app, host="0.0.0.0", port=8000)  # pragma: no cover
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
