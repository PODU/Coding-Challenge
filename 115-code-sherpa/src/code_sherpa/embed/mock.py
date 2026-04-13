"""Offline embedder for tests and local runs.

Each identifier is hashed into one of ``dim`` buckets and the result is
normalised. Text that shares identifiers ends up close together, which is all
the retrieval tests need.
"""

from __future__ import annotations

import hashlib
import math
import re

from ..models import Vector
from .base import Embedder

_TOKEN_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def _tokenize(text: str) -> list[str]:
    return [t.lower() for t in _TOKEN_RE.findall(text)]


def _bucket(token: str, dim: int) -> int:
    digest = hashlib.sha1(token.encode("utf-8")).digest()
    return int.from_bytes(digest[:4], "big") % dim


class MockEmbedder(Embedder):
    def __init__(self, dim: int = 256) -> None:
        if dim <= 0:
            raise ValueError("dim must be positive")
        self.dim = dim

    def embed(self, texts: list[str]) -> list[Vector]:
        return [self._embed_one(t) for t in texts]

    def _embed_one(self, text: str) -> Vector:
        vec = [0.0] * self.dim
        for tok in _tokenize(text):
            # A second hash picks the sign, so bucket collisions tend to cancel
            # rather than pile up.
            sign = 1.0 if hashlib.sha1((tok + "#s").encode()).digest()[0] & 1 else -1.0
            vec[_bucket(tok, self.dim)] += sign
        norm = math.sqrt(sum(v * v for v in vec))
        if norm == 0.0:
            return vec
        return [v / norm for v in vec]
