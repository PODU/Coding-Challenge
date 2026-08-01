"""Per-user notes that outlive sessions.

Past advice is fed back into the system prompt with an instruction not to
repeat it (see persona.py).
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class MemoryNote:
    kind: str  # "profile" | "advice"
    text: str
    session_id: str


def _normalize(text: str) -> str:
    return " ".join(text.lower().split())


class MemoryBank:
    def __init__(self, root: Path | str) -> None:
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)

    def _path(self, user_id: str) -> Path:
        return self.root / f"{user_id}.json"

    def _load(self, user_id: str) -> list[MemoryNote]:
        path = self._path(user_id)
        if not path.exists():
            return []
        return [MemoryNote(**n) for n in json.loads(path.read_text(encoding="utf-8"))]

    def remember(self, user_id: str, note: MemoryNote) -> bool:
        """Store a note, unless an identical one exists. Returns whether it was stored."""
        notes = self._load(user_id)
        if any(_normalize(n.text) == _normalize(note.text) for n in notes):
            return False
        notes.append(note)
        self._path(user_id).write_text(
            json.dumps([n.__dict__ for n in notes], indent=2), encoding="utf-8"
        )
        return True

    def recall(self, user_id: str, *, kind: str | None = None) -> list[MemoryNote]:
        notes = self._load(user_id)
        if kind is not None:
            notes = [n for n in notes if n.kind == kind]
        return notes

    def already_advised(self, user_id: str, text: str) -> bool:
        target = _normalize(text)
        return any(
            _normalize(n.text) == target for n in self.recall(user_id, kind="advice")
        )
