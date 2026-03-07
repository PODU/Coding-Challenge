"""Embedders: a cheap deterministic ``HashEmbedder`` (used by the tests) and
``LocalEmbedder``, which wraps a real offline embedding model.
"""

from __future__ import annotations

import hashlib
import math
import re
from typing import Protocol, runtime_checkable

from .models import Vector

_WORD = re.compile(r"[a-z0-9]+")


@runtime_checkable
class Embedder(Protocol):
    @property
    def dim(self) -> int: ...

    def embed_texts(self, texts: list[str]) -> list[Vector]: ...

    def embed_text(self, text: str) -> Vector: ...


def _l2_normalize(vec: list[float]) -> list[float]:
    norm = math.sqrt(sum(v * v for v in vec))
    if norm == 0.0:
        return vec
    return [v / norm for v in vec]


class HashEmbedder:
    """Bag-of-words embedder: each word is hashed into one of ``dim`` buckets.

    Texts sharing vocabulary end up close together, which is enough for
    retrieval and the cache to behave sensibly in tests.
    """

    def __init__(self, dim: int = 256) -> None:
        if dim <= 0:
            raise ValueError("dim must be positive")
        self._dim = dim

    @property
    def dim(self) -> int:
        return self._dim

    def _bucket(self, token: str) -> int:
        digest = hashlib.sha1(token.encode("utf-8")).digest()
        return int.from_bytes(digest[:4], "big") % self._dim

    def embed_text(self, text: str) -> Vector:
        vec = [0.0] * self._dim
        for token in _WORD.findall(text.lower()):
            vec[self._bucket(token)] += 1.0
        return _l2_normalize(vec)

    def embed_texts(self, texts: list[str]) -> list[Vector]:
        return [self.embed_text(t) for t in texts]


class LocalEmbedder:
    """Wraps a fastembed model. fastembed is only imported when this is built."""

    def __init__(self, model_name: str = "BAAI/bge-small-en-v1.5") -> None:
        try:
            from fastembed import TextEmbedding  # type: ignore
        except Exception as exc:  # pragma: no cover - exercised only with the dep installed
            raise RuntimeError(
                "LocalEmbedder requires the optional 'fastembed' dependency. "
                "Install it with: pip install 'rtfm[local-embed]', or use HashEmbedder."
            ) from exc
        self._model = TextEmbedding(model_name=model_name)
        probe = next(iter(self._model.embed(["dimension probe"])))
        self._dim = int(len(probe))

    @property
    def dim(self) -> int:
        return self._dim

    def embed_texts(self, texts: list[str]) -> list[Vector]:
        return [list(map(float, v)) for v in self._model.embed(list(texts))]

    def embed_text(self, text: str) -> Vector:
        return self.embed_texts([text])[0]
