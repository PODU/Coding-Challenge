"""Settings read from CONTRACT_* environment variables.

The API key is deliberately not part of this: ClaudeClient reads
``ANTHROPIC_API_KEY`` only when it makes its first request, so the tests
don't need one.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

DEFAULT_MODEL = "claude-opus-4-8"


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw is None or not raw.strip():
        return default
    try:
        return int(raw)
    except ValueError:
        return default


@dataclass
class Config:
    provider: str = "claude"
    model: str = DEFAULT_MODEL
    max_chunk_tokens: int = 6000
    chunk_overlap: int = 400
    analysis_workers: int = 4
    batch_size: int = 8
    store_dir: str = "./.contract_sessions"

    @classmethod
    def from_env(cls) -> "Config":
        return cls(
            provider=os.environ.get("CONTRACT_LLM_PROVIDER", "claude").strip() or "claude",
            model=os.environ.get("CONTRACT_LLM_MODEL", DEFAULT_MODEL).strip() or DEFAULT_MODEL,
            max_chunk_tokens=_env_int("CONTRACT_MAX_CHUNK_TOKENS", 6000),
            chunk_overlap=_env_int("CONTRACT_CHUNK_OVERLAP", 400),
            analysis_workers=_env_int("CONTRACT_ANALYSIS_WORKERS", 4),
            batch_size=_env_int("CONTRACT_BATCH_SIZE", 8),
            store_dir=os.environ.get("CONTRACT_STORE_DIR", "./.contract_sessions").strip()
            or "./.contract_sessions",
        )
