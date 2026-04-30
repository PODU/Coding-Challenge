"""Embedding providers.

``MockEmbeddingProvider`` hashes words into a fixed-size vector. It needs no
network and is stable across runs, and texts that share words still end up
close under cosine similarity, which is all the tests need.
"""

from __future__ import annotations

import hashlib
from typing import Protocol, runtime_checkable

import numpy as np

from .models import Vector
from .text import tokenize


@runtime_checkable
class EmbeddingProvider(Protocol):
    @property
    def dim(self) -> int: ...

    def embed(self, texts: list[str]) -> list[Vector]: ...


class MockEmbeddingProvider:
    def __init__(self, dim: int = 256) -> None:
        self._dim = dim

    @property
    def dim(self) -> int:
        return self._dim

    def embed(self, texts: list[str]) -> list[Vector]:
        return [self._embed_one(t) for t in texts]

    def _embed_one(self, text: str) -> Vector:
        vec = np.zeros(self._dim, dtype=float)
        for tok in tokenize(text):
            digest = hashlib.md5(tok.encode("utf-8")).hexdigest()
            sign = 1.0 if int(digest[0], 16) % 2 == 0 else -1.0
            vec[int(digest, 16) % self._dim] += sign
        norm = float(np.linalg.norm(vec))
        if norm > 0.0:
            vec /= norm  # unit length, so a dot product is the cosine similarity
        return vec.tolist()
