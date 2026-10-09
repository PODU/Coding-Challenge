"""Tiny git helpers (shelling out; every function tolerates a non-git directory)."""

from __future__ import annotations

import subprocess
from pathlib import Path


def _git(path: str | Path, *args: str) -> str | None:
    try:
        out = subprocess.run(["git", "-C", str(path), *args], capture_output=True, text=True,
                             timeout=30, encoding="utf-8", errors="replace")
    except (OSError, subprocess.TimeoutExpired):
        return None
    if out.returncode != 0:
        return None
    return out.stdout.strip()


def head(path) -> str | None:
    return _git(path, "rev-parse", "HEAD")


def is_dirty(path) -> bool:
    out = _git(path, "status", "--porcelain", "--untracked-files=normal")
    return bool(out)


def short(sha: str | None) -> str:
    return sha[:10] if sha else "-"
