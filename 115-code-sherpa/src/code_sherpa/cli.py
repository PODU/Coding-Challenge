"""Command-line interface for Code Sherpa.

Commands:
    sherpa ingest   <project> <path>      walk, chunk, embed, index
    sherpa ask      <project> "<q>"       retrieve + explain with citations
    sherpa reingest <project>             incremental re-ingest of changed files
    sherpa projects                       list known projects
    sherpa memory   <project>             show recent episodes / facts
    sherpa fact     <project> <key> <val> add a semantic fact

The default in-memory store doesn't survive between runs, so ``ask``
re-ingests a known project's root when the index is empty. Set
``SHERPA_STORE=oracle`` for a persistent store.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from .config import ConfigError, SherpaConfig
from .core.sherpa import Sherpa

_DEFAULT_REGISTRY = str(Path.home() / ".sherpa" / "registry.json")


def _build(args: argparse.Namespace) -> Sherpa:
    config = SherpaConfig.from_env()
    return Sherpa.from_config(config, registry_path=args.registry)


def _cmd_ingest(sherpa: Sherpa, args: argparse.Namespace) -> int:
    report = sherpa.ingest(args.project, args.path, progress=_progress(args))
    sherpa.registry.save()
    print(
        f"Ingested '{report.project}': {report.files_ingested} file(s) indexed, "
        f"{report.files_unchanged} unchanged, {report.files_removed} removed, "
        f"{report.total_chunks} chunk(s) total."
    )
    return 0


def _cmd_reingest(sherpa: Sherpa, args: argparse.Namespace) -> int:
    try:
        report = sherpa.reingest(args.project, progress=_progress(args))
    except KeyError as exc:
        print(str(exc), file=sys.stderr)
        return 2
    sherpa.registry.save()
    print(
        f"Re-ingested '{report.project}': {report.files_ingested} changed, "
        f"{report.files_removed} removed, {report.total_chunks} chunk(s) total."
    )
    return 0


def _cmd_ask(sherpa: Sherpa, args: argparse.Namespace) -> int:
    project = args.project
    if sherpa.store.count(project) == 0:
        info = sherpa.registry.get(project)
        if info and info.root and Path(info.root).exists():
            sherpa.ingest(project, info.root, force=True)
    answer = sherpa.ask(project, args.question)

    print(answer.text)
    if answer.citations:
        print("\nCitations:")
        for c in answer.citations:
            print(f"  - {c.label()}")
    if answer.used_memory:
        print("\n(answer informed by memory from earlier sessions)")
    if answer.error:
        print(f"\n[warning] {answer.error}", file=sys.stderr)
    return 0


def _cmd_projects(sherpa: Sherpa, args: argparse.Namespace) -> int:
    projects = sherpa.registry.list()
    if not projects:
        print("No projects registered yet. Try: sherpa ingest <name> <path>")
        return 0
    for info in projects:
        print(f"{info.name}\t{info.file_count} file(s)\t{info.chunk_count} chunk(s)\t{info.root}")
    return 0


def _cmd_memory(sherpa: Sherpa, args: argparse.Namespace) -> int:
    episodes = sherpa.memory.recent_episodes(args.project, limit=args.limit)
    facts = sherpa.memory.facts(args.project)
    print(f"Facts for '{args.project}':")
    for f in facts:
        print(f"  - {f.key} = {f.value}")
    print(f"\nRecent questions for '{args.project}':")
    for ep in episodes:
        print(f"  - {ep.question}")
    if not facts and not episodes:
        print("  (none yet)")
    return 0


def _cmd_fact(sherpa: Sherpa, args: argparse.Namespace) -> int:
    sherpa.add_fact(args.project, args.key, args.value)
    print(f"Stored fact for '{args.project}': {args.key} = {args.value}")
    return 0


def _progress(args: argparse.Namespace):
    if getattr(args, "quiet", False):
        return None
    return lambda msg: print(f"  · {msg}", file=sys.stderr)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="sherpa", description="Code Sherpa CLI")
    parser.add_argument(
        "--registry", default=_DEFAULT_REGISTRY, help="path to the project registry JSON"
    )
    parser.add_argument("-q", "--quiet", action="store_true", help="suppress progress")
    sub = parser.add_subparsers(dest="command", required=True)

    p_ing = sub.add_parser("ingest", help="ingest a codebase")
    p_ing.add_argument("project")
    p_ing.add_argument("path")
    p_ing.set_defaults(func=_cmd_ingest)

    p_re = sub.add_parser("reingest", help="incremental re-ingest")
    p_re.add_argument("project")
    p_re.set_defaults(func=_cmd_reingest)

    p_ask = sub.add_parser("ask", help="ask a question")
    p_ask.add_argument("project")
    p_ask.add_argument("question")
    p_ask.set_defaults(func=_cmd_ask)

    p_proj = sub.add_parser("projects", help="list projects")
    p_proj.set_defaults(func=_cmd_projects)

    p_mem = sub.add_parser("memory", help="show memory for a project")
    p_mem.add_argument("project")
    p_mem.add_argument("--limit", type=int, default=10)
    p_mem.set_defaults(func=_cmd_memory)

    p_fact = sub.add_parser("fact", help="store a semantic fact")
    p_fact.add_argument("project")
    p_fact.add_argument("key")
    p_fact.add_argument("value")
    p_fact.set_defaults(func=_cmd_fact)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        sherpa = _build(args)
    except ConfigError as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        return 2
    return args.func(sherpa, args)


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
