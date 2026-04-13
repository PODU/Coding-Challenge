"""Episodic + semantic memory behind a single interface."""

from __future__ import annotations

from .base import Episode, MemoryStore, SemanticFact
from .in_memory import InMemoryMemoryStore

__all__ = [
    "MemoryStore",
    "Episode",
    "SemanticFact",
    "InMemoryMemoryStore",
    "build_memory",
]


def build_memory(config) -> MemoryStore:
    return InMemoryMemoryStore()
