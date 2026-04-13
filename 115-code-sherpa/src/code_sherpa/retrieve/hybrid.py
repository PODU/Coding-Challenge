"""Hybrid retrieval: vector and full-text search merged with RRF. Without an
embedder it falls back to text search only."""

from __future__ import annotations

from ..embed.base import Embedder
from ..models import ScoredChunk
from ..store.base import VectorStore
from .fusion import reciprocal_rank_fusion


class HybridRetriever:
    def __init__(
        self,
        store: VectorStore,
        embedder: Embedder | None,
        rrf_k: int = 60,
        vector_weight: float = 1.0,
        text_weight: float = 1.0,
        min_vector_score: float = 0.2,
    ) -> None:
        self.store = store
        self.embedder = embedder
        self.rrf_k = rrf_k
        self.vector_weight = vector_weight
        self.text_weight = text_weight
        # Vector search always returns *something*. Dropping weak matches means
        # an unanswerable question gets no context instead of the nearest
        # irrelevant chunk.
        self.min_vector_score = min_vector_score

    def retrieve(
        self,
        project: str,
        query: str,
        k: int = 8,
        *,
        vector_weight: float | None = None,
        text_weight: float | None = None,
        use_vector: bool = True,
    ) -> list[ScoredChunk]:
        if not query.strip() or k <= 0:
            return []

        # Fetch extra from each channel so fusion has overlap to work with.
        fetch = max(k * 2, k + 5)
        vec_weight = self.vector_weight if vector_weight is None else vector_weight
        txt_weight = self.text_weight if text_weight is None else text_weight

        text_hits = self.store.text_search(project, query, fetch)
        vector_hits = []
        if use_vector and self.embedder is not None:
            qvec = self.embedder.embed_one(query)
            vector_hits = [
                h
                for h in self.store.vector_search(project, qvec, fetch)
                if h.score >= self.min_vector_score
            ]

        ranked_lists: dict[str, list[str]] = {}
        chunk_by_id = {}
        if vector_hits:
            ranked_lists["vector"] = [h.chunk.id for h in vector_hits]
            chunk_by_id.update({h.chunk.id: h.chunk for h in vector_hits})
        if text_hits:
            ranked_lists["text"] = [h.chunk.id for h in text_hits]
            chunk_by_id.update({h.chunk.id: h.chunk for h in text_hits})

        if not ranked_lists:
            return []

        vec_rank = {cid: i + 1 for i, cid in enumerate(ranked_lists.get("vector", []))}
        txt_rank = {cid: i + 1 for i, cid in enumerate(ranked_lists.get("text", []))}

        fused = reciprocal_rank_fusion(
            ranked_lists,
            k=self.rrf_k,
            weights={"vector": vec_weight, "text": txt_weight},
        )
        results: list[ScoredChunk] = []
        for item in fused[:k]:
            results.append(
                ScoredChunk(
                    chunk=chunk_by_id[item.key],
                    score=item.score,
                    vector_rank=vec_rank.get(item.key),
                    text_rank=txt_rank.get(item.key),
                )
            )
        return results
