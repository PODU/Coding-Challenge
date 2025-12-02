"""Message types, errors, and the :class:`LLMClient` protocol.

The app only talks to ``LLMClient``; the real client and the mock both
implement it. Nothing here imports the ``anthropic`` SDK.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterator, Literal, Protocol, Sequence, runtime_checkable

Role = Literal["user", "assistant"]


@dataclass(frozen=True)
class Message:
    role: Role
    content: str


@dataclass(frozen=True)
class LLMResult:
    text: str
    stop_reason: str | None = None


class LLMError(Exception):
    """Base class for every error surfaced by an :class:`LLMClient`."""


class LLMAuthError(LLMError):
    """Missing or invalid API key (HTTP 401/403)."""


class LLMRateLimitError(LLMError):
    """Rate limited by the provider (HTTP 429)."""


class LLMServiceError(LLMError):
    """Service-side or connection failure (HTTP 5xx / network)."""


class LLMEmptyResponse(LLMError):
    """The model returned no usable text."""


@runtime_checkable
class LLMClient(Protocol):
    """Implementations should raise only ``LLMError`` subclasses."""

    def complete(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> LLMResult:
        """Return a full completion for ``messages`` under ``system``."""
        ...

    def stream(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> Iterator[str]:
        """Yield response text deltas as they are produced."""
        ...
