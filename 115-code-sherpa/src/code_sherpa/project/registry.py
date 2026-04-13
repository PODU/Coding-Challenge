"""Known projects and the file hashes from their last ingest.

Comparing a fresh walk against the stored hashes tells us which files were
added, changed or removed, so re-ingest only touches those. The CLI saves the
registry as JSON.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from ..models import FileRecord


@dataclass(frozen=True)
class Changes:
    added: list[FileRecord] = field(default_factory=list)
    changed: list[FileRecord] = field(default_factory=list)
    removed: list[str] = field(default_factory=list)
    unchanged: list[FileRecord] = field(default_factory=list)

    @property
    def to_ingest(self) -> list[FileRecord]:
        return self.added + self.changed

    @property
    def is_empty(self) -> bool:
        return not (self.added or self.changed or self.removed)


@dataclass
class ProjectInfo:
    name: str
    root: str
    file_shas: dict[str, str] = field(default_factory=dict)
    chunk_count: int = 0

    @property
    def file_count(self) -> int:
        return len(self.file_shas)


class ProjectRegistry:
    def __init__(self, path: str | Path | None = None) -> None:
        self._projects: dict[str, ProjectInfo] = {}
        self._path = Path(path) if path else None
        if self._path and self._path.is_file():
            self.load()

    def register(self, name: str, root: str) -> ProjectInfo:
        info = self._projects.get(name)
        if info is None:
            info = ProjectInfo(name=name, root=str(root))
            self._projects[name] = info
        else:
            info.root = str(root)
        return info

    def has(self, name: str) -> bool:
        return name in self._projects

    def get(self, name: str) -> ProjectInfo | None:
        return self._projects.get(name)

    def list(self) -> list[ProjectInfo]:
        return sorted(self._projects.values(), key=lambda p: p.name)

    def remove(self, name: str) -> bool:
        return self._projects.pop(name, None) is not None

    def diff(self, name: str, records: list[FileRecord]) -> Changes:
        info = self._projects.get(name)
        prior = dict(info.file_shas) if info else {}
        added: list[FileRecord] = []
        changed: list[FileRecord] = []
        unchanged: list[FileRecord] = []
        for rec in records:
            old = prior.get(rec.path)
            if old is None:
                added.append(rec)
            elif old != rec.sha256:
                changed.append(rec)
            else:
                unchanged.append(rec)
        removed = sorted(set(prior) - {rec.path for rec in records})
        return Changes(added=added, changed=changed, removed=removed, unchanged=unchanged)

    def commit(
        self,
        name: str,
        records: list[FileRecord],
        removed: list[str] | None = None,
        chunk_count: int | None = None,
    ) -> None:
        info = self._projects.setdefault(name, ProjectInfo(name=name, root=""))
        for rec in records:
            info.file_shas[rec.path] = rec.sha256
        for path in removed or []:
            info.file_shas.pop(path, None)
        if chunk_count is not None:
            info.chunk_count = chunk_count

    def to_dict(self) -> dict:
        return {
            "projects": {
                name: {
                    "root": info.root,
                    "file_shas": info.file_shas,
                    "chunk_count": info.chunk_count,
                }
                for name, info in self._projects.items()
            }
        }

    def save(self, path: str | Path | None = None) -> None:
        target = Path(path) if path else self._path
        if target is None:
            return
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(json.dumps(self.to_dict(), indent=2), encoding="utf-8")

    def load(self, path: str | Path | None = None) -> None:
        target = Path(path) if path else self._path
        if target is None or not target.is_file():
            return
        data = json.loads(target.read_text(encoding="utf-8"))
        self._projects = {
            name: ProjectInfo(
                name=name,
                root=pdata.get("root", ""),
                file_shas=dict(pdata.get("file_shas", {})),
                chunk_count=pdata.get("chunk_count", 0),
            )
            for name, pdata in data.get("projects", {}).items()
        }
