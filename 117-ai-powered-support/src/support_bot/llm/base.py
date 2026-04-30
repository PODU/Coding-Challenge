"""LLM client interface and shared errors."""

from __future__ import annotations

from typing import Protocol, runtime_checkable

from ..models import LLMResponse, Prompt


class LLMError(Exception):
    """Base class for LLM client failures."""


class TransientLLMError(LLMError):
    """Network error, 5xx, timeout or rate limit. Worth trying another client."""


class AllProvidersFailed(LLMError):
    """Every client in a fallback chain failed."""


@runtime_checkable
class LLMClient(Protocol):
    name: str

    def complete(self, prompt: Prompt, *, model: str | None = None) -> LLMResponse: ...
