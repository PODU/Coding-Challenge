"""Embedder interface."""

from __future__ import annotations

from abc import ABC, abstractmethod

from ..models import Vector


class Embedder(ABC):
    """Maps text to fixed-size vectors.

    Must be deterministic within a process, otherwise query vectors won't be
    comparable with the ones stored at ingest time.
    """

    dim: int

    @abstractmethod
    def embed(self, texts: list[str]) -> list[Vector]:
        """Embed a batch of texts, preserving order."""

    def embed_one(self, text: str) -> Vector:
        return self.embed([text])[0]
