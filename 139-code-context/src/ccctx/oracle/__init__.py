"""Oracle AI Database 26ai SQL for the ccctx schema (printed by `ccctx oracle`)."""

from __future__ import annotations

from pathlib import Path

HERE = Path(__file__).parent
PARTS = {"model": "model.sql", "schema": "schema.sql", "search": "search.sql", "graph": "graph.sql"}


def script(part: str = "all") -> str:
    names = list(PARTS) if part == "all" else [part]
    return "\n\n".join((HERE / PARTS[n]).read_text(encoding="utf-8") for n in names)
