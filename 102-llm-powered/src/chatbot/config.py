"""Runtime configuration. The API key only ever comes from the environment."""

from __future__ import annotations

import os
from dataclasses import dataclass

from .llm import LLMAuthError

DEFAULT_MODEL = "claude-opus-4-8"


@dataclass(frozen=True)
class Config:
    api_key: str
    model: str = DEFAULT_MODEL
    max_tokens: int = 1024
    max_turns: int = 20

    @classmethod
    def from_env(cls, **overrides) -> "Config":
        """Raise :class:`LLMAuthError` if ``ANTHROPIC_API_KEY`` is unset."""
        key = os.environ.get("ANTHROPIC_API_KEY")
        if not key:
            raise LLMAuthError("ANTHROPIC_API_KEY is not set")
        return cls(api_key=key, **overrides)
