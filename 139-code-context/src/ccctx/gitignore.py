"""A .gitignore matcher and an ignore-aware repository walker.

Implements the parts of gitignore(5) that matter in practice: per-directory
.gitignore files, `#` comments, `!` negation, trailing `/` for directories,
leading or embedded `/` anchoring, and `*`, `?`, `[...]` and `**` globs.
Later rules override earlier ones, and rules in deeper directories override
rules from their parents, exactly as git evaluates them.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class Rule:
    regex: re.Pattern
    negate: bool
    dir_only: bool
    base: str  # repo-relative directory the .gitignore lives in ("" for root)
    pattern: str = ""


def _glob_to_regex(glob: str) -> str:
    out = []
    i, n = 0, len(glob)
    while i < n:
        c = glob[i]
        if c == "*":
            if glob[i:i + 2] == "**":
                # "**/" matches zero or more directories; a trailing "/**" everything inside.
                if glob[i:i + 3] == "**/":
                    out.append("(?:.*/)?")
                    i += 3
                    continue
                out.append(".*")
                i += 2
                continue
            out.append("[^/]*")
        elif c == "?":
            out.append("[^/]")
        elif c == "[":
            j = glob.find("]", i + 1)
            if j == -1:
                out.append(re.escape(c))
            else:
                body = glob[i + 1:j]
                if body.startswith("!"):
                    body = "^" + body[1:]
                out.append("[" + body.replace("\\", "\\\\") + "]")
                i = j
        elif c == "\\" and i + 1 < n:
            i += 1
            out.append(re.escape(glob[i]))
        else:
            out.append(re.escape(c))
        i += 1
    return "".join(out)


def parse_gitignore(text: str, base: str = "") -> list[Rule]:
    rules = []
    for raw in text.splitlines():
        line = raw.rstrip("\r")
        # Trailing spaces are ignored unless escaped.
        if not line.endswith("\\ "):
            line = line.rstrip(" ")
        if not line or line.startswith("#"):
            continue
        negate = line.startswith("!")
        if negate:
            line = line[1:]
        elif line.startswith("\\"):
            line = line[1:]
        dir_only = line.endswith("/")
        line = line.rstrip("/")
        if not line:
            continue
        anchored = "/" in line
        line = line.lstrip("/")
        body = _glob_to_regex(line)
        regex = "^" + body + "$" if anchored else "^(?:.*/)?" + body + "$"
        rules.append(Rule(re.compile(regex), negate, dir_only, base, raw))
    return rules


class IgnoreMatcher:
    """Accumulates rules as the walk descends; answers 'is this path ignored?'."""

    def __init__(self, extra_patterns: list[str] | None = None):
        self.rules: list[Rule] = []
        if extra_patterns:
            self.rules.extend(parse_gitignore("\n".join(extra_patterns)))

    def add_file(self, gitignore: Path, base: str) -> None:
        try:
            text = gitignore.read_text(encoding="utf-8", errors="replace")
        except OSError:
            return
        self.rules.extend(parse_gitignore(text, base))

    def ignored(self, relpath: str, is_dir: bool) -> bool:
        result = False
        for rule in self.rules:
            if rule.dir_only and not is_dir:
                continue
            if rule.base:
                if not relpath.startswith(rule.base + "/"):
                    continue
                sub = relpath[len(rule.base) + 1:]
            else:
                sub = relpath
            if rule.regex.match(sub):
                result = not rule.negate
        return result


@dataclass
class WalkStats:
    scanned: int = 0
    ignored: int = 0
    binary: int = 0
    too_large: int = 0
    files: list[str] = field(default_factory=list)

    @property
    def skipped(self) -> int:
        return self.ignored + self.binary + self.too_large


ALWAYS_SKIP_DIRS = {".git", ".hg", ".svn"}


def is_binary(data: bytes) -> bool:
    return b"\x00" in data[:8192]


def walk_repo(root: Path, max_bytes: int = 1_000_000) -> tuple[WalkStats, dict[str, bytes]]:
    """Walk `root`, honouring .gitignore files. Returns stats and {relpath: bytes}.

    "Scanned" counts every file the walker looked at (including ones it then
    skipped as binary or too large); "ignored" counts files and directories
    pruned by ignore rules (a pruned directory counts once, its contents are
    never visited).
    """
    root = Path(root)
    stats = WalkStats()
    contents: dict[str, bytes] = {}
    matcher = IgnoreMatcher()
    info_exclude = root / ".git" / "info" / "exclude"
    if info_exclude.is_file():
        matcher.add_file(info_exclude, "")

    for dirpath, dirnames, filenames in os.walk(root, topdown=True):
        rel_dir = Path(dirpath).relative_to(root).as_posix()
        rel_dir = "" if rel_dir == "." else rel_dir
        if ".gitignore" in filenames:
            matcher.add_file(Path(dirpath) / ".gitignore", rel_dir)

        keep = []
        for d in sorted(dirnames):
            rel = f"{rel_dir}/{d}" if rel_dir else d
            if d in ALWAYS_SKIP_DIRS:
                continue
            if matcher.ignored(rel, True):
                stats.ignored += 1
                continue
            keep.append(d)
        dirnames[:] = keep

        for f in sorted(filenames):
            rel = f"{rel_dir}/{f}" if rel_dir else f
            if matcher.ignored(rel, False):
                stats.ignored += 1
                continue
            full = Path(dirpath) / f
            if not full.is_file():
                continue
            stats.scanned += 1
            try:
                size = full.stat().st_size
            except OSError:
                continue
            if size > max_bytes:
                stats.too_large += 1
                continue
            data = full.read_bytes()
            if is_binary(data):
                stats.binary += 1
                continue
            stats.files.append(rel)
            contents[rel] = data
    return stats, contents
