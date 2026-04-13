"""Finds the source files to index under a directory.

Skips the usual noise (``.git``, ``node_modules``, build output), anything
matched by the top-level ``.gitignore``, binaries and oversized files. Each
file's sha256 is recorded so re-ingest can tell what changed.
"""

from __future__ import annotations

import fnmatch
import hashlib
import os
from pathlib import Path
from typing import Iterator

from ..models import FileRecord

LANGUAGE_BY_EXT: dict[str, str] = {
    ".py": "python",
    ".pyi": "python",
    ".js": "javascript",
    ".jsx": "javascript",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".java": "java",
    ".go": "go",
    ".rb": "ruby",
    ".rs": "rust",
    ".c": "c",
    ".h": "c",
    ".cc": "cpp",
    ".cpp": "cpp",
    ".hpp": "cpp",
    ".cs": "csharp",
    ".php": "php",
    ".swift": "swift",
    ".kt": "kotlin",
    ".scala": "scala",
    ".sh": "shell",
    ".sql": "sql",
    ".md": "markdown",
    ".rst": "text",
    ".txt": "text",
    ".toml": "toml",
    ".yaml": "yaml",
    ".yml": "yaml",
    ".json": "json",
    ".cfg": "ini",
    ".ini": "ini",
}

DEFAULT_IGNORES: set[str] = {
    ".git",
    ".hg",
    ".svn",
    "node_modules",
    "__pycache__",
    ".venv",
    "venv",
    "env",
    "dist",
    "build",
    "target",
    ".mypy_cache",
    ".pytest_cache",
    ".idea",
    ".vscode",
    ".tox",
    ".codegraph",
    "vendor",
    ".eggs",
}

_BINARY_EXTS = {
    ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".webp", ".pdf",
    ".zip", ".gz", ".tar", ".tgz", ".7z", ".rar", ".jar", ".war",
    ".exe", ".dll", ".so", ".dylib", ".o", ".a", ".class", ".pyc",
    ".mp3", ".mp4", ".mov", ".avi", ".wav", ".ttf", ".woff", ".woff2",
    ".bin", ".dat", ".db", ".sqlite", ".lock",
}


def language_for(path: str) -> str:
    return LANGUAGE_BY_EXT.get(Path(path).suffix.lower(), "unknown")


def _looks_binary(path: Path, sniff_bytes: int = 4096) -> bool:
    try:
        with path.open("rb") as fh:
            chunk = fh.read(sniff_bytes)
    except OSError:
        return True
    return b"\x00" in chunk


class RepoWalker:
    def __init__(
        self,
        max_file_bytes: int = 1_000_000,
        extra_ignores: set[str] | None = None,
        use_gitignore: bool = True,
    ) -> None:
        self.max_file_bytes = max_file_bytes
        self.ignores = set(DEFAULT_IGNORES) | set(extra_ignores or set())
        self.use_gitignore = use_gitignore

    def walk(self, root: str | os.PathLike[str]) -> Iterator[FileRecord]:
        root_path = Path(root).resolve()
        if not root_path.exists():
            raise FileNotFoundError(f"Repo path does not exist: {root_path}")
        if root_path.is_file():
            rec = self._record(root_path, root_path.parent)
            if rec:
                yield rec
            return

        gi_patterns = self._load_gitignore(root_path) if self.use_gitignore else []

        for dirpath, dirnames, filenames in os.walk(root_path):
            dpath = Path(dirpath)
            # Editing dirnames in place stops os.walk from descending.
            dirnames[:] = [
                d
                for d in dirnames
                if not self._ignored_dir(d)
                and not self._gitignored(dpath / d, root_path, gi_patterns, is_dir=True)
            ]
            for name in sorted(filenames):
                fpath = dpath / name
                if self._gitignored(fpath, root_path, gi_patterns, is_dir=False):
                    continue
                rec = self._record(fpath, root_path)
                if rec:
                    yield rec

    def _ignored_dir(self, name: str) -> bool:
        return name in self.ignores or name.endswith((".egg-info", ".dist-info"))

    def _record(self, fpath: Path, root_path: Path) -> FileRecord | None:
        try:
            if not fpath.is_file() or fpath.is_symlink():
                return None
            size = fpath.stat().st_size
        except OSError:
            return None
        if size > self.max_file_bytes:
            return None
        if fpath.suffix.lower() in _BINARY_EXTS:
            return None
        if _looks_binary(fpath):
            return None
        try:
            data = fpath.read_bytes()
        except OSError:
            return None
        sha = hashlib.sha256(data).hexdigest()
        rel = fpath.relative_to(root_path).as_posix() if fpath != root_path else fpath.name
        return FileRecord(
            path=rel,
            abs_path=str(fpath),
            language=language_for(fpath.name),
            size=size,
            sha256=sha,
        )

    def _load_gitignore(self, root_path: Path) -> list[str]:
        gi = root_path / ".gitignore"
        if not gi.is_file():
            return []
        patterns: list[str] = []
        try:
            for line in gi.read_text(encoding="utf-8", errors="replace").splitlines():
                line = line.strip()
                if not line or line.startswith("#") or line.startswith("!"):
                    continue
                patterns.append(line.rstrip("/"))
        except OSError:
            return []
        return patterns

    def _gitignored(
        self,
        path: Path,
        root_path: Path,
        patterns: list[str],
        is_dir: bool,
    ) -> bool:
        if not patterns:
            return False
        try:
            rel = path.relative_to(root_path).as_posix()
        except ValueError:  # pragma: no cover
            return False
        name = path.name
        for pat in patterns:
            if fnmatch.fnmatch(name, pat) or fnmatch.fnmatch(rel, pat):
                return True
            if fnmatch.fnmatch(rel, f"{pat}/*") or rel == pat:
                return True
        return False
