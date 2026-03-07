"""Vector stores: an in-memory numpy one and one on Redis Stack (RediSearch)."""

from __future__ import annotations

from typing import Optional, Protocol, runtime_checkable

import numpy as np

from .models import Chunk, RetrievedChunk, Vector


class DimMismatchError(ValueError):
    """A vector's length doesn't match the index."""


@runtime_checkable
class VectorStore(Protocol):
    @property
    def dim(self) -> int: ...

    def upsert(self, chunks: list[Chunk]) -> None: ...

    def knn(
        self,
        query: Vector,
        k: int,
        filters: Optional[dict[str, str]] = None,
    ) -> list[RetrievedChunk]: ...

    def count(self) -> int: ...

    def clear(self) -> None: ...


class InMemoryVectorStore:
    """Brute-force cosine search over a numpy matrix."""

    def __init__(self, dim: int) -> None:
        self._dim = dim
        self._chunks: list[Chunk] = []
        self._matrix: Optional[np.ndarray] = None  # (n, dim), rows L2-normalized

    @property
    def dim(self) -> int:
        return self._dim

    def upsert(self, chunks: list[Chunk]) -> None:
        for chunk in chunks:
            if chunk.embedding is None:
                raise ValueError(f"chunk {chunk.chunk_id!r} has no embedding")
            if len(chunk.embedding) != self._dim:
                raise DimMismatchError(
                    f"chunk {chunk.chunk_id!r} has dim {len(chunk.embedding)}, "
                    f"index expects {self._dim}. Re-ingest after changing the embedding model."
                )
        if not chunks:
            return
        self._chunks.extend(chunks)
        self._rebuild()

    def _rebuild(self) -> None:
        if not self._chunks:
            self._matrix = None
            return
        mat = np.asarray([c.embedding for c in self._chunks], dtype=np.float64)
        norms = np.linalg.norm(mat, axis=1, keepdims=True)
        norms[norms == 0.0] = 1.0
        self._matrix = mat / norms

    def knn(
        self,
        query: Vector,
        k: int,
        filters: Optional[dict[str, str]] = None,
    ) -> list[RetrievedChunk]:
        if self._matrix is None or not self._chunks:
            return []
        q = np.asarray(query, dtype=np.float64)
        if q.shape[0] != self._dim:
            raise DimMismatchError(
                f"query has dim {q.shape[0]}, index expects {self._dim}."
            )
        qn = np.linalg.norm(q)
        if qn == 0.0:
            return []
        q = q / qn

        # Apply metadata filters before ranking.
        idxs = list(range(len(self._chunks)))
        if filters:
            idxs = [
                i
                for i in idxs
                if all(getattr(self._chunks[i], key, None) == val for key, val in filters.items())
            ]
            if not idxs:
                return []

        sims = self._matrix[idxs] @ q
        order = np.argsort(-sims)[:k]
        return [
            RetrievedChunk(chunk=self._chunks[idxs[int(row)]], score=float(sims[int(row)]))
            for row in order
        ]

    def count(self) -> int:
        return len(self._chunks)

    def clear(self) -> None:
        self._chunks = []
        self._matrix = None


class RedisVectorStore:
    """Chunks stored as Redis hashes under a FLAT cosine RediSearch index."""

    def __init__(
        self,
        dim: int,
        *,
        url: str = "redis://localhost:6379",
        index_name: str = "rtfm_chunks",
        prefix: str = "rtfm:chunk:",
        client=None,
    ) -> None:
        self._dim = dim
        self._index_name = index_name
        self._prefix = prefix
        if client is not None:
            self._r = client
        else:
            try:
                import redis  # type: ignore
            except Exception as exc:  # pragma: no cover
                raise RuntimeError(
                    "RedisVectorStore requires the optional 'redis' dependency. "
                    "Install it with: pip install 'rtfm[redis]'."
                ) from exc
            self._r = redis.Redis.from_url(url)
        self._ensure_index()

    @property
    def dim(self) -> int:
        return self._dim

    def _ensure_index(self) -> None:  # pragma: no cover - requires Redis Stack
        from redis.commands.search.field import NumericField, TagField, TextField, VectorField
        from redis.commands.search.indexDefinition import IndexDefinition, IndexType

        try:
            self._r.ft(self._index_name).info()
            return
        except Exception:
            pass
        schema = (
            TextField("text"),
            TagField("filename"),
            TextField("section"),
            NumericField("position"),
            VectorField(
                "embedding",
                "FLAT",
                {"TYPE": "FLOAT32", "DIM": self._dim, "DISTANCE_METRIC": "COSINE"},
            ),
        )
        definition = IndexDefinition(prefix=[self._prefix], index_type=IndexType.HASH)
        self._r.ft(self._index_name).create_index(schema, definition=definition)

    def upsert(self, chunks: list[Chunk]) -> None:  # pragma: no cover - requires Redis Stack
        pipe = self._r.pipeline()
        for chunk in chunks:
            if chunk.embedding is None:
                raise ValueError(f"chunk {chunk.chunk_id!r} has no embedding")
            if len(chunk.embedding) != self._dim:
                raise DimMismatchError(
                    f"chunk {chunk.chunk_id!r} has dim {len(chunk.embedding)}, "
                    f"index expects {self._dim}. Re-ingest after changing the embedding model."
                )
            key = f"{self._prefix}{chunk.chunk_id}"
            pipe.hset(
                key,
                mapping={
                    "text": chunk.text,
                    "filename": chunk.filename,
                    "section": chunk.section,
                    "position": chunk.position,
                    "embedding": np.asarray(chunk.embedding, dtype=np.float32).tobytes(),
                },
            )
        pipe.execute()

    def knn(
        self,
        query: Vector,
        k: int,
        filters: Optional[dict[str, str]] = None,
    ) -> list[RetrievedChunk]:  # pragma: no cover - requires Redis Stack
        from redis.commands.search.query import Query

        prefilter = "*"
        if filters and "filename" in filters:
            prefilter = f"@filename:{{{filters['filename']}}}"
        q = (
            Query(f"({prefilter})=>[KNN {k} @embedding $vec AS score]")
            .sort_by("score")
            .return_fields("text", "filename", "section", "position", "score")
            .dialect(2)
        )
        params = {"vec": np.asarray(query, dtype=np.float32).tobytes()}
        res = self._r.ft(self._index_name).search(q, query_params=params)
        out: list[RetrievedChunk] = []
        for doc in res.docs:
            chunk = Chunk(
                filename=doc.filename,
                section=getattr(doc, "section", ""),
                position=int(doc.position),
                text=doc.text,
                chunk_id=doc.id.removeprefix(self._prefix),
            )
            # RediSearch reports cosine distance, not similarity.
            out.append(RetrievedChunk(chunk=chunk, score=1.0 - float(doc.score)))
        return out

    def count(self) -> int:  # pragma: no cover - requires Redis Stack
        return len(self._r.keys(f"{self._prefix}*"))

    def clear(self) -> None:  # pragma: no cover - requires Redis Stack
        keys = self._r.keys(f"{self._prefix}*")
        if keys:
            self._r.delete(*keys)
