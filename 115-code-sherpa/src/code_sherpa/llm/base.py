"""LLMClient interface + message/response value objects."""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Literal

Role = Literal["system", "user", "assistant"]


class LLMError(RuntimeError):
    """The LLM call failed (after any retries)."""


@dataclass(frozen=True)
class Message:
    role: Role
    content: str


@dataclass(frozen=True)
class LLMResponse:
    text: str
    model: str
    stop_reason: str | None = None
    input_tokens: int | None = None
    output_tokens: int | None = None


class LLMClient(ABC):
    """Chat completion. The first message may be ``system``; the rest
    alternate user/assistant. Failures raise :class:`LLMError`.
    """

    @abstractmethod
    def complete(
        self,
        messages: list[Message],
        *,
        model: str | None = None,
        max_tokens: int = 1024,
    ) -> LLMResponse:
        ...
