"""Default settings. The API key is not configured here; the Anthropic SDK
reads ``ANTHROPIC_API_KEY`` itself."""

from __future__ import annotations

from dataclasses import dataclass

DEFAULT_MODEL = "claude-opus-4-8"
DEFAULT_MAX_TOKENS = 16000
DEFAULT_PERMISSION_MODE = "prompt"
DEFAULT_MAX_ITERATIONS = 25


@dataclass
class Config:
    model: str = DEFAULT_MODEL
    max_tokens: int = DEFAULT_MAX_TOKENS
    permission_mode: str = DEFAULT_PERMISSION_MODE
    max_iterations: int = DEFAULT_MAX_ITERATIONS
