"""Reads local challenge specs in Markdown, JSON or YAML.

PyYAML is used if installed. Otherwise a small built-in parser handles the
subset our spec files use: flat keys, string lists and ``|`` blocks.
"""

from __future__ import annotations

import json
from pathlib import Path

from ..llm import ToolSpec
from . import ToolError, ToolRegistry

SUPPORTED = {".md", ".markdown", ".json", ".yaml", ".yml"}


def _mini_yaml(text: str) -> dict:
    data: dict[str, object] = {}
    key: str | None = None
    block: list[str] | None = None
    items: list[str] | None = None
    for raw in text.splitlines():
        if block is not None:
            if raw.startswith("  ") or raw.strip() == "":
                block.append(raw[2:])
                continue
            data[key] = "\n".join(block).rstrip() + "\n"
            key, block = None, None
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("- ") and items is not None:
            items.append(stripped[2:].strip().strip("'\""))
            continue
        if ":" not in stripped:
            raise ToolError(f"unparseable YAML line: {stripped!r}")
        k, _, v = stripped.partition(":")
        key, v = k.strip(), v.strip()
        items = None
        if v == "|":
            block = []
        elif v == "":
            items = []
            data[key] = items
        else:
            data[key] = v.strip("'\"")
    if block is not None:
        data[key] = "\n".join(block).rstrip() + "\n"
    return data


def _load_yaml(text: str) -> dict:
    try:
        import yaml  # type: ignore

        return yaml.safe_load(text)
    except ImportError:
        return _mini_yaml(text)


class SpecStore:
    def __init__(self, root: Path | str) -> None:
        self.root = Path(root)

    def list(self) -> list[str]:
        if not self.root.is_dir():
            return []
        return sorted(
            p.name for p in self.root.iterdir() if p.suffix.lower() in SUPPORTED
        )

    def read(self, name: str) -> dict:
        path = (self.root / name).resolve()
        if self.root.resolve() not in path.parents:
            raise ToolError(f"spec path escapes the spec directory: {name}")
        if not path.is_file():
            raise ToolError(f"no such spec: {name} (available: {', '.join(self.list()) or 'none'})")
        text = path.read_text(encoding="utf-8")
        suffix = path.suffix.lower()
        if suffix == ".json":
            body = json.loads(text)
        elif suffix in (".yaml", ".yml"):
            body = _load_yaml(text)
        else:  # Markdown is returned as-is, titled by its first heading.
            title = next(
                (l.lstrip("# ").strip() for l in text.splitlines() if l.startswith("#")),
                name,
            )
            body = {"title": title, "content": text}
        return {"name": name, "format": suffix.lstrip("."), "spec": body}


def register(registry: ToolRegistry, store: SpecStore) -> None:
    registry.register(
        ToolSpec(
            name="list_challenges",
            description="List the challenge specification files available locally.",
            parameters={"type": "object", "properties": {}},
        ),
        lambda: {"challenges": store.list()},
    )
    registry.register(
        ToolSpec(
            name="read_challenge_spec",
            description=(
                "Read a local challenge specification file (markdown, JSON or "
                "YAML) and return its parsed content."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "name": {"type": "string", "description": "Spec file name"}
                },
                "required": ["name"],
            },
        ),
        lambda name: store.read(name),
    )
