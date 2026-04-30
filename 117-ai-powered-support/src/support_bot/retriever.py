"""Embeds queries and searches the vector store.

Hits below the similarity threshold are dropped; an empty result is how the
bot knows it has nothing to answer from.
"""

from __future__ import annotations

from .embeddings import EmbeddingProvider
from .models import Chunk, Hit
from .text import tokenize
from .vectorstore import VectorStore


class Retriever:
    def __init__(
        self,
        embedder: EmbeddingProvider,
        store: VectorStore,
        *,
        top_k: int = 4,
        threshold: float = 0.18,
    ) -> None:
        self.embedder = embedder
        self.store = store
        self.top_k = top_k
        self.threshold = threshold
        # Every word in the indexed KB, for the off-topic check.
        self.domain_terms: set[str] = set()

    def index(self, chunks: list[Chunk]) -> None:
        if not chunks:
            return
        vectors = self.embedder.embed([c.text for c in chunks])
        self.store.upsert(chunks, vectors)
        for c in chunks:
            self.domain_terms.update(tokenize(c.title))
            self.domain_terms.update(tokenize(c.text))

    def search(
        self,
        query: str,
        *,
        k: int | None = None,
        threshold: float | None = None,
    ) -> list[Hit]:
        if k is None:
            k = self.top_k
        if threshold is None:
            threshold = self.threshold
        query_vec = self.embedder.embed([query])[0]
        return [h for h in self.store.search(query_vec, k) if h.score >= threshold]
