"""LLM clients behind one interface."""

from __future__ import annotations

from .base import AllProvidersFailed, LLMClient, LLMError, TransientLLMError
from .fallback import FallbackLLMClient
from .mock import MockLLMClient, scripted
from .routing import QueryClassifier, RoutingLLMClient

__all__ = [
    "LLMClient",
    "LLMError",
    "TransientLLMError",
    "AllProvidersFailed",
    "MockLLMClient",
    "scripted",
    "FallbackLLMClient",
    "RoutingLLMClient",
    "QueryClassifier",
]


def __getattr__(name: str):  # pragma: no cover - lazy import to avoid hard SDK dep
    if name == "ClaudeClient":
        from .claude import ClaudeClient

        return ClaudeClient
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
