"""Sessions saved as one JSON file each, so they can be resumed by id.

Locally the files are plain JSON; encryption at rest is left to the deployment
platform (see deploy/agent_config.yaml).
"""

from __future__ import annotations

import json
import uuid
from dataclasses import asdict, dataclass, field
from pathlib import Path

from .llm import Message


@dataclass
class Session:
    id: str
    user_id: str
    history: list[Message] = field(default_factory=list)
    meta: dict = field(default_factory=dict)

    def append(self, message: Message) -> None:
        self.history.append(message)


class SessionStore:
    def __init__(self, root: Path | str) -> None:
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)

    def _path(self, session_id: str) -> Path:
        return self.root / f"{session_id}.json"

    def create(self, user_id: str, session_id: str | None = None) -> Session:
        session = Session(id=session_id or uuid.uuid4().hex[:12], user_id=user_id)
        self.save(session)
        return session

    def load(self, session_id: str) -> Session:
        data = json.loads(self._path(session_id).read_text(encoding="utf-8"))
        return Session(
            id=data["id"],
            user_id=data["user_id"],
            history=[Message(**m) for m in data["history"]],
            meta=data.get("meta", {}),
        )

    def load_or_create(self, user_id: str, session_id: str) -> Session:
        if self._path(session_id).exists():
            return self.load(session_id)
        return self.create(user_id, session_id)

    def save(self, session: Session) -> None:
        payload = {
            "id": session.id,
            "user_id": session.user_id,
            "history": [asdict(m) for m in session.history],
            "meta": session.meta,
        }
        self._path(session.id).write_text(
            json.dumps(payload, indent=2), encoding="utf-8"
        )

    def list(self) -> list[str]:
        return sorted(p.stem for p in self.root.glob("*.json"))
