"""Vector + full-text storage behind a single interface."""

from __future__ import annotations

from .base import SearchHit, VectorStore
from .memory_store import InMemoryVectorStore

__all__ = ["VectorStore", "SearchHit", "InMemoryVectorStore", "build_store"]


def build_store(config) -> VectorStore:
    if config.store == "memory":
        return InMemoryVectorStore()
    if config.store == "oracle":
        from .oracle import OracleVectorStore

        return OracleVectorStore(
            dsn=config.db_dsn, user=config.db_user, password=config.db_password
        )
    raise ValueError(f"Unknown store: {config.store!r}")
