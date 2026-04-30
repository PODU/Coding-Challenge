"""Vector stores. Only the in-memory one exists so far."""

from __future__ import annotations

from typing import Protocol, runtime_checkable

import numpy as np

from .models import Chunk, Hit, Vector


@runtime_checkable
class VectorStore(Protocol):
    def upsert(self, chunks: list[Chunk], vectors: list[Vector]) -> None: ...

    def search(self, query_vec: Vector, k: int) -> list[Hit]: ...

    def __len__(self) -> int: ...


class InMemoryVectorStore:
    """Brute-force cosine search over numpy arrays, keyed by ``chunk_id``."""

    def __init__(self) -> None:
        self._chunks: dict[str, Chunk] = {}
        self._vectors: dict[str, np.ndarray] = {}

    def upsert(self, chunks: list[Chunk], vectors: list[Vector]) -> None:
        for chunk, vector in zip(chunks, vectors):
            self._chunks[chunk.chunk_id] = chunk
            self._vectors[chunk.chunk_id] = np.asarray(vector, dtype=float)

    def search(self, query_vec: Vector, k: int) -> list[Hit]:
        if not self._chunks or k <= 0:
            return []
        ids = list(self._chunks.keys())
        matrix = np.stack([self._vectors[i] for i in ids])
        query = np.asarray(query_vec, dtype=float)
        scores = matrix @ query  # vectors are unit length, so this is cosine similarity
        order = np.argsort(-scores)[:k]
        return [Hit(chunk=self._chunks[ids[i]], score=float(scores[i])) for i in order]

    def __len__(self) -> int:
        return len(self._chunks)
