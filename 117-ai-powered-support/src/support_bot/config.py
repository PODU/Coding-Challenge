"""Settings, overridable through SUPPORT_BOT_* environment variables.

The API key is deliberately not part of this: only the Claude client reads it.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

_DEFAULT_KB = Path(__file__).resolve().parent / "kb"

# (input, output) USD per million tokens.
PRICES: dict[str, tuple[float, float]] = {
    "claude-opus-4-8": (5.0, 25.0),
    "claude-opus-4-7": (5.0, 25.0),
    "claude-sonnet-4-6": (3.0, 15.0),
    "claude-haiku-4-5": (1.0, 5.0),
}


def _env(name: str, default: str) -> str:
    return os.environ.get(name, default)


@dataclass
class Config:
    model: str = "claude-opus-4-8"           # for complex queries
    cheap_model: str = "claude-haiku-4-5"    # for simple ones
    max_tokens: int = 1024

    retrieval_top_k: int = 4
    similarity_threshold: float = 0.18
    embedding_dim: int = 256

    # Prompt budgets, counted in whitespace-separated words.
    max_history_tokens: int = 800
    max_context_tokens: int = 1200

    escalation_repeat_limit: int = 3
    abstain_sentinel: str = "NO_ANSWER"

    chunk_max_tokens: int = 180
    chunk_overlap_tokens: int = 40

    kb_path: str = field(default_factory=lambda: str(_DEFAULT_KB))

    @classmethod
    def from_env(cls) -> "Config":
        return cls(
            model=_env("SUPPORT_BOT_MODEL", "claude-opus-4-8"),
            cheap_model=_env("SUPPORT_BOT_CHEAP_MODEL", "claude-haiku-4-5"),
            max_tokens=int(_env("SUPPORT_BOT_MAX_TOKENS", "1024")),
            retrieval_top_k=int(_env("SUPPORT_BOT_TOP_K", "4")),
            similarity_threshold=float(_env("SUPPORT_BOT_SIMILARITY_THRESHOLD", "0.18")),
            max_history_tokens=int(_env("SUPPORT_BOT_MAX_HISTORY_TOKENS", "800")),
            max_context_tokens=int(_env("SUPPORT_BOT_MAX_CONTEXT_TOKENS", "1200")),
            escalation_repeat_limit=int(_env("SUPPORT_BOT_REPEAT_LIMIT", "3")),
            kb_path=_env("SUPPORT_BOT_KB_PATH", str(_DEFAULT_KB)),
        )
