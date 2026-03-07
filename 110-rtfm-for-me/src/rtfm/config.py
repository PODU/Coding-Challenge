"""Settings, read from environment variables with defaults.

Only ``AnthropicClient`` ever uses ``ANTHROPIC_API_KEY``; tests don't need it.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

DEFAULT_MODEL = "claude-opus-4-8"


@dataclass(frozen=True)
class Config:
    model: str = DEFAULT_MODEL
    redis_url: str = "redis://localhost:6379"
    embed_model: str = "BAAI/bge-small-en-v1.5"
    embed_dim: int = 256  # HashEmbedder only
    top_k: int = 5
    chunk_tokens: int = 500
    chunk_overlap: int = 50
    cache_threshold: float = 0.95  # min cosine similarity for a cache hit
    session_ttl: int = 86_400  # 24h
    max_session_turns: int = 6  # recent turns included in the prompt
    max_output_tokens: int = 4096

    @classmethod
    def from_env(cls, env: dict[str, str] | None = None) -> "Config":
        e = os.environ if env is None else env

        def _int(name: str, default: int) -> int:
            raw = e.get(name)
            return int(raw) if raw not in (None, "") else default

        def _float(name: str, default: float) -> float:
            raw = e.get(name)
            return float(raw) if raw not in (None, "") else default

        return cls(
            model=e.get("RTFM_MODEL") or DEFAULT_MODEL,
            redis_url=e.get("REDIS_URL") or "redis://localhost:6379",
            embed_model=e.get("RTFM_EMBED_MODEL") or "BAAI/bge-small-en-v1.5",
            embed_dim=_int("RTFM_EMBED_DIM", 256),
            top_k=_int("RTFM_TOP_K", 5),
            chunk_tokens=_int("RTFM_CHUNK_TOKENS", 500),
            chunk_overlap=_int("RTFM_CHUNK_OVERLAP", 50),
            cache_threshold=_float("RTFM_CACHE_THRESHOLD", 0.95),
            session_ttl=_int("RTFM_SESSION_TTL", 86_400),
            max_session_turns=_int("RTFM_MAX_SESSION_TURNS", 6),
            max_output_tokens=_int("RTFM_MAX_OUTPUT_TOKENS", 4096),
        )

    @staticmethod
    def anthropic_api_key(env: dict[str, str] | None = None) -> str | None:
        e = os.environ if env is None else env
        return e.get("ANTHROPIC_API_KEY") or None
