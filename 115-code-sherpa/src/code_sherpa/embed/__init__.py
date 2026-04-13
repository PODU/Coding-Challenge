"""Embedding providers behind a single interface."""

from __future__ import annotations

from .base import Embedder
from .mock import MockEmbedder

__all__ = ["Embedder", "MockEmbedder", "build_embedder"]


def build_embedder(config) -> Embedder:
    if config.embedder == "mock":
        return MockEmbedder(dim=config.embed_dim)
    if config.embedder == "coderank":
        from .coderank import CodeRankEmbedder

        return CodeRankEmbedder(dim=config.embed_dim)
    raise ValueError(f"Unknown embedder: {config.embedder!r}")
