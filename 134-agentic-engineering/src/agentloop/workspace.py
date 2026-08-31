"""Copying projects and writing the agent's file output into them.

Agents return whole files in blocks like this:

    === FILE: relative/path.py ===
    <content>
    === END FILE ===

Whole files rather than diffs, so re-running a node after a crash just writes
the same files again.
"""

from __future__ import annotations

import re
import shutil
from pathlib import Path

IGNORE = shutil.ignore_patterns(
    "__pycache__", "*.pyc", ".git", ".pytest_cache", "runs", "*.egg-info",
    ".scannerwork", "node_modules")

FILE_BLOCK = re.compile(
    r"^===\s*FILE:\s*(?P<path>[^\r\n=]+?)\s*===\r?\n"
    r"(?P<body>.*?)"
    r"^===\s*END FILE\s*===",
    re.MULTILINE | re.DOTALL)


def copy_project(src: Path, dst: Path) -> None:
    shutil.copytree(src, dst, ignore=IGNORE, dirs_exist_ok=True)


def parse_file_blocks(text: str) -> dict[str, str]:
    files: dict[str, str] = {}
    for m in FILE_BLOCK.finditer(text):
        body = m.group("body")
        if not body.endswith("\n"):
            body += "\n"
        files[m.group("path").strip()] = body
    return files


def write_files(workspace: Path, files: dict[str, str]) -> list[str]:
    """Write agent output into the workspace; refuse paths that escape it."""
    workspace = workspace.resolve()
    written = []
    for rel, content in files.items():
        target = (workspace / rel).resolve()
        if workspace not in target.parents and target != workspace:
            raise ValueError(f"agent tried to write outside workspace: {rel}")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")
        written.append(rel)
    return written


def file_listing(workspace: Path, max_files: int = 200) -> str:
    """Relative paths of workspace files, for prompt context."""
    paths = []
    for p in sorted(workspace.rglob("*")):
        if p.is_file() and "__pycache__" not in p.parts:
            paths.append(p.relative_to(workspace).as_posix())
        if len(paths) >= max_files:
            break
    return "\n".join(paths)


def read_sources(workspace: Path, max_bytes: int = 60_000) -> str:
    """Concatenated source files as FILE blocks, for prompt context."""
    chunks, total = [], 0
    for p in sorted(workspace.rglob("*")):
        if not p.is_file() or "__pycache__" in p.parts:
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        rel = p.relative_to(workspace).as_posix()
        block = f"=== FILE: {rel} ===\n{text}=== END FILE ===\n"
        total += len(block)
        if total > max_bytes:
            break
        chunks.append(block)
    return "\n".join(chunks)
