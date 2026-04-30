"""Conversation history for one chat."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from uuid import uuid4

from .models import Citation, Message, Role


@dataclass
class Turn:
    role: Role
    content: str
    timestamp: str
    citations: list[Citation] = field(default_factory=list)


@dataclass
class Conversation:
    conversation_id: str = field(default_factory=lambda: uuid4().hex)
    turns: list[Turn] = field(default_factory=list)
    # Low-confidence answers in a row; enough of them triggers a handoff.
    consecutive_low_conf: int = 0

    def add(self, role: Role, content: str, citations: list[Citation] | None = None) -> Turn:
        turn = Turn(
            role=role,
            content=content,
            timestamp=datetime.now(timezone.utc).isoformat(),
            citations=list(citations or []),
        )
        self.turns.append(turn)
        return turn

    def messages(self) -> list[Message]:
        return [Message(role=t.role, content=t.content) for t in self.turns]

    def reset(self) -> None:
        """Clear history but keep the ``conversation_id``."""
        self.turns.clear()
        self.consecutive_low_conf = 0
