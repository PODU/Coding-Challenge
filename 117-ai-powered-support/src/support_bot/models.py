"""Data types shared across the package."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Literal

Role = Literal["user", "assistant", "system"]
Vector = list[float]


@dataclass(frozen=True)
class Chunk:
    chunk_id: str
    doc_id: str
    title: str
    source: str
    text: str


@dataclass
class Hit:
    chunk: Chunk
    score: float


@dataclass
class Message:
    role: Role
    content: str


@dataclass
class Prompt:
    system: str
    messages: list[Message]

    def last_user_text(self) -> str:
        for m in reversed(self.messages):
            if m.role == "user":
                return m.content
        return ""


@dataclass
class LLMResponse:
    text: str
    model: str
    input_tokens: int
    output_tokens: int
    finish_reason: str
    latency_ms: float
    provider: str = "mock"
    fallback_used: bool = False
    attempts: int = 1


@dataclass
class Citation:
    title: str
    source: str
    chunk_id: str


@dataclass
class Answer:
    text: str
    citations: list[Citation]
    request_id: str
    model: str


class EscalationReason(str, Enum):
    NO_CONTEXT = "NO_CONTEXT"
    LOW_CONFIDENCE = "LOW_CONFIDENCE"
    OFF_TOPIC = "OFF_TOPIC"
    HUMAN_REQUEST = "HUMAN_REQUEST"
    REPEAT_LIMIT = "REPEAT_LIMIT"
    PROVIDER_ERROR = "PROVIDER_ERROR"
    EMPTY_INPUT = "EMPTY_INPUT"


# Reasons that go to a human. The others (off-topic, empty input) just get a
# reply from the bot.
HANDOFF_REASONS = frozenset(
    {
        EscalationReason.NO_CONTEXT,
        EscalationReason.LOW_CONFIDENCE,
        EscalationReason.HUMAN_REQUEST,
        EscalationReason.REPEAT_LIMIT,
        EscalationReason.PROVIDER_ERROR,
    }
)


@dataclass
class Escalation:
    """Returned instead of an :class:`Answer` when the bot won't answer."""

    reason: EscalationReason
    message: str
    request_id: str
    human_handoff: bool
    transcript: list[Message] = field(default_factory=list)
    context: list[Hit] = field(default_factory=list)
