"""Application settings.

Settings are passed in explicitly rather than read from the environment deep in
the code. The Anthropic SDK reads ``ANTHROPIC_API_KEY`` itself.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, replace

LLM_MODEL = "claude-opus-4-8"


@dataclass(frozen=True)
class Settings:
    """Tunables. Tests build this directly; the CLI uses ``from_env``."""

    default_timeout_s: float = 60.0
    default_max_retries: int = 3
    backoff_base_s: float = 1.0
    max_backoff_s: float = 60.0
    email_max_retries: int = 3
    email_backoff_base_s: float = 1.0
    scheduler_tick_s: float = 1.0
    max_parallel_agents: int = 8
    db_path: str = "ai_agents.db"
    llm_model: str = LLM_MODEL
    health_failure_threshold: int = 3

    @classmethod
    def from_env(cls, **overrides: object) -> "Settings":
        """Build settings from environment variables, then apply ``overrides``."""

        def env_float(name: str, default: float) -> float:
            raw = os.environ.get(name)
            return float(raw) if raw else default

        def env_int(name: str, default: int) -> int:
            raw = os.environ.get(name)
            return int(raw) if raw else default

        base = cls(
            default_timeout_s=env_float("DEFAULT_TIMEOUT_S", 60.0),
            default_max_retries=env_int("DEFAULT_MAX_RETRIES", 3),
            backoff_base_s=env_float("BACKOFF_BASE_S", 1.0),
            max_backoff_s=env_float("MAX_BACKOFF_S", 60.0),
            email_max_retries=env_int("EMAIL_MAX_RETRIES", 3),
            email_backoff_base_s=env_float("EMAIL_BACKOFF_BASE_S", 1.0),
            scheduler_tick_s=env_float("SCHEDULER_TICK_S", 1.0),
            max_parallel_agents=env_int("MAX_PARALLEL_AGENTS", 8),
            db_path=os.environ.get("DB_PATH", "ai_agents.db"),
            llm_model=os.environ.get("LLM_MODEL", LLM_MODEL),
            health_failure_threshold=env_int("HEALTH_FAILURE_THRESHOLD", 3),
        )
        return replace(base, **overrides) if overrides else base  # type: ignore[arg-type]
