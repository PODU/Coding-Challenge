"""Embed the question and find the nearest chunks, optionally filtered."""

from __future__ import annotations

from typing import Optional

from .embedder import Embedder
from .models import RetrievedChunk, Vector
from .vector_store import VectorStore


class Retriever:
    def __init__(self, embedder: Embedder, store: VectorStore) -> None:
        self._embedder = embedder
        self._store = store

    def embed_question(self, question: str) -> Vector:
        return self._embedder.embed_text(question)

    def retrieve(
        self,
        question: str,
        *,
        k: int = 5,
        filters: Optional[dict[str, str]] = None,
        embedding: Optional[Vector] = None,
    ) -> list[RetrievedChunk]:
        """Return up to ``k`` chunks. Pass ``embedding`` if the question has
        already been embedded (e.g. for the cache lookup).
        """
        vec = embedding if embedding is not None else self.embed_question(question)
        return self._store.knn(vec, k, filters=filters)
