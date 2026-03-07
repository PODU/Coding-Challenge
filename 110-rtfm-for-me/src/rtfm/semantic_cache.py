"""Semantic cache: reuse the answer when a near-identical question comes in.

Lookups compare question embeddings by cosine similarity against a threshold.
Not-found answers and refusals are never stored.
"""

from __future__ import annotations

import math
from typing import Optional, Protocol, runtime_checkable

from .models import Answer, CacheStats, Vector


@runtime_checkable
class SemanticCache(Protocol):
    def lookup(self, embedding: Vector) -> Optional[Answer]: ...

    def store(self, embedding: Vector, answer: Answer) -> None: ...

    @property
    def stats(self) -> CacheStats: ...

    def clear(self) -> None: ...


def _cosine(a: list[float], b: list[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0.0 or nb == 0.0:
        return 0.0
    return dot / (na * nb)


class InMemorySemanticCache:
    """In-memory cache that also tracks hits, misses and estimated savings."""

    def __init__(
        self,
        *,
        threshold: float = 0.95,
        est_latency_ms: float = 800.0,
        est_cost_usd: float = 0.01,
    ) -> None:
        self._threshold = threshold
        self._entries: list[tuple[list[float], Answer]] = []
        self._stats = CacheStats()
        self._est_latency_ms = est_latency_ms
        self._est_cost_usd = est_cost_usd

    def lookup(self, embedding: Vector) -> Optional[Answer]:
        best: Optional[Answer] = None
        best_sim = self._threshold
        for vec, ans in self._entries:
            sim = _cosine(list(embedding), vec)
            if sim >= best_sim:
                best_sim = sim
                best = ans
        if best is not None:
            self._stats.hits += 1
            self._stats.latency_saved_ms += self._est_latency_ms
            self._stats.cost_saved_usd += self._est_cost_usd
            return best
        self._stats.misses += 1
        return None

    def store(self, embedding: Vector, answer: Answer) -> None:
        if not answer.found or answer.refused:
            return
        self._entries.append((list(embedding), answer))

    @property
    def stats(self) -> CacheStats:
        return self._stats

    def clear(self) -> None:
        self._entries = []
        self._stats = CacheStats()


class RedisSemanticCache:  # pragma: no cover - requires Redis Stack
    """Redis-backed cache. Lookups scan every stored vector, which is fine at
    this size and behaves exactly like the in-memory version.
    """

    def __init__(
        self,
        dim: int,
        *,
        url: str = "redis://localhost:6379",
        prefix: str = "rtfm:cache:",
        threshold: float = 0.95,
        client=None,
    ) -> None:
        self._dim = dim
        self._prefix = prefix
        self._threshold = threshold
        self._stats = CacheStats()
        if client is not None:
            self._r = client
        else:
            try:
                import redis  # type: ignore
            except Exception as exc:
                raise RuntimeError(
                    "RedisSemanticCache requires the optional 'redis' dependency."
                ) from exc
            self._r = redis.Redis.from_url(url)

    def lookup(self, embedding: Vector) -> Optional[Answer]:
        import json

        import numpy as np

        q = np.asarray(embedding, dtype=np.float64)
        qn = np.linalg.norm(q)
        if qn == 0.0:
            self._stats.misses += 1
            return None
        q = q / qn
        best: Optional[Answer] = None
        best_sim = self._threshold
        for key in self._r.keys(f"{self._prefix}*"):
            raw = self._r.hgetall(key)
            vec = np.frombuffer(raw[b"embedding"], dtype=np.float32).astype(np.float64)
            vn = np.linalg.norm(vec)
            if vn == 0.0:
                continue
            sim = float(q @ (vec / vn))
            if sim >= best_sim:
                best_sim = sim
                payload = json.loads(raw[b"answer"])
                best = Answer(
                    text=payload["text"],
                    citations=[],
                    found=payload["found"],
                    refused=payload["refused"],
                )
        if best is not None:
            self._stats.hits += 1
            return best
        self._stats.misses += 1
        return None

    def store(self, embedding: Vector, answer: Answer) -> None:
        import json
        import uuid

        import numpy as np

        if not answer.found or answer.refused:
            return
        key = f"{self._prefix}{uuid.uuid4().hex}"
        self._r.hset(
            key,
            mapping={
                "embedding": np.asarray(embedding, dtype=np.float32).tobytes(),
                "answer": json.dumps(
                    {"text": answer.text, "found": answer.found, "refused": answer.refused}
                ),
            },
        )

    @property
    def stats(self) -> CacheStats:
        return self._stats

    def clear(self) -> None:
        keys = self._r.keys(f"{self._prefix}*")
        if keys:
            self._r.delete(*keys)
        self._stats = CacheStats()
