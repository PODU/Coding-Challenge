"""Configuration from environment variables, with an optional sherpa.toml.

By default everything runs offline: mock embedder, in-memory store and mock
LLM. The real LLM is used when ``ANTHROPIC_API_KEY`` is set (or SHERPA_LLM asks
for it). Secrets only ever come from the environment.
"""

from __future__ import annotations

import os
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

MODEL_DEFAULT = "claude-opus-4-8"


class ConfigError(RuntimeError):
    """Invalid configuration, reported at startup."""


def _env(name: str, default: str | None = None) -> str | None:
    val = os.environ.get(name)
    return val if val not in (None, "") else default


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw in (None, ""):
        return default
    try:
        return int(raw)
    except ValueError as exc:  # pragma: no cover - defensive
        raise ConfigError(f"{name} must be an integer, got {raw!r}") from exc


def _env_float(name: str, default: float) -> float:
    raw = os.environ.get(name)
    if raw in (None, ""):
        return default
    try:
        return float(raw)
    except ValueError as exc:  # pragma: no cover - defensive
        raise ConfigError(f"{name} must be a number, got {raw!r}") from exc


@dataclass
class SherpaConfig:
    llm: str = "mock"  # "anthropic" | "mock"
    embedder: str = "mock"  # "coderank" | "mock"
    store: str = "memory"  # "oracle" | "memory"

    # llm
    model: str = MODEL_DEFAULT
    max_tokens: int = 1024
    llm_timeout: float = 60.0
    llm_max_retries: int = 2

    # embedder
    embed_dim: int = 256

    # ingest
    max_file_bytes: int = 1_000_000
    max_chunk_chars: int = 4000
    min_chunk_chars: int = 40
    window_size: int = 60  # lines
    window_overlap: int = 10  # lines

    # retrieval
    top_k: int = 8
    rrf_k: int = 60
    vector_weight: float = 1.0
    text_weight: float = 1.0
    min_vector_score: float = 0.2

    # measured in characters as a rough stand-in for tokens
    context_char_budget: int = 8000

    # These come from the environment only.
    anthropic_api_key: str | None = field(default=None, repr=False)
    db_dsn: str | None = field(default=None, repr=False)
    db_user: str | None = field(default=None, repr=False)
    db_password: str | None = field(default=None, repr=False)

    @classmethod
    def from_env(cls, toml_path: str | os.PathLike[str] | None = None) -> "SherpaConfig":
        """Read ``sherpa.toml`` if present; environment variables override it."""

        data: dict[str, Any] = {}
        path = Path(toml_path) if toml_path else Path("sherpa.toml")
        if path.is_file():
            with path.open("rb") as fh:
                data = tomllib.load(fh)

        def pick(key: str, default: Any) -> Any:
            return data.get(key, default)

        api_key = _env("ANTHROPIC_API_KEY")
        # Use the real LLM whenever a key is available, unless told otherwise.
        llm = _env("SHERPA_LLM") or pick("llm", "anthropic" if api_key else "mock")

        cfg = cls(
            llm=llm,
            embedder=_env("SHERPA_EMBEDDER") or pick("embedder", "mock"),
            store=_env("SHERPA_STORE") or pick("store", "memory"),
            model=_env("SHERPA_MODEL") or pick("model", MODEL_DEFAULT),
            max_tokens=_env_int("SHERPA_MAX_TOKENS", pick("max_tokens", 1024)),
            llm_timeout=_env_float("SHERPA_LLM_TIMEOUT", pick("llm_timeout", 60.0)),
            llm_max_retries=_env_int("SHERPA_LLM_MAX_RETRIES", pick("llm_max_retries", 2)),
            embed_dim=_env_int("SHERPA_EMBED_DIM", pick("embed_dim", 256)),
            max_file_bytes=_env_int("SHERPA_MAX_FILE_BYTES", pick("max_file_bytes", 1_000_000)),
            max_chunk_chars=_env_int("SHERPA_MAX_CHUNK_CHARS", pick("max_chunk_chars", 4000)),
            min_chunk_chars=_env_int("SHERPA_MIN_CHUNK_CHARS", pick("min_chunk_chars", 40)),
            window_size=_env_int("SHERPA_WINDOW_SIZE", pick("window_size", 60)),
            window_overlap=_env_int("SHERPA_WINDOW_OVERLAP", pick("window_overlap", 10)),
            top_k=_env_int("SHERPA_TOP_K", pick("top_k", 8)),
            rrf_k=_env_int("SHERPA_RRF_K", pick("rrf_k", 60)),
            vector_weight=_env_float("SHERPA_VECTOR_WEIGHT", pick("vector_weight", 1.0)),
            text_weight=_env_float("SHERPA_TEXT_WEIGHT", pick("text_weight", 1.0)),
            min_vector_score=_env_float(
                "SHERPA_MIN_VECTOR_SCORE", pick("min_vector_score", 0.2)
            ),
            context_char_budget=_env_int(
                "SHERPA_CONTEXT_BUDGET", pick("context_char_budget", 8000)
            ),
            anthropic_api_key=api_key,
            db_dsn=_env("SHERPA_DB_DSN"),
            db_user=_env("SHERPA_DB_USER"),
            db_password=_env("SHERPA_DB_PASSWORD"),
        )
        cfg.validate()
        return cfg

    def validate(self) -> None:
        if self.llm not in ("anthropic", "mock"):
            raise ConfigError(f"Unknown SHERPA_LLM={self.llm!r} (use 'anthropic' or 'mock')")
        if self.embedder not in ("coderank", "mock"):
            raise ConfigError(f"Unknown SHERPA_EMBEDDER={self.embedder!r}")
        if self.store not in ("oracle", "memory"):
            raise ConfigError(f"Unknown SHERPA_STORE={self.store!r}")
        if self.llm == "anthropic" and not self.anthropic_api_key:
            raise ConfigError(
                "SHERPA_LLM=anthropic requires ANTHROPIC_API_KEY in the environment."
            )
        if self.store == "oracle" and not self.db_dsn:
            raise ConfigError("SHERPA_STORE=oracle requires SHERPA_DB_DSN in the environment.")
        if self.window_overlap >= self.window_size:
            raise ConfigError("window_overlap must be smaller than window_size.")
        if self.embed_dim <= 0:
            raise ConfigError("embed_dim must be positive.")
