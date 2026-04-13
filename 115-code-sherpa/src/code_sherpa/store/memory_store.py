"""The default in-memory store. Cosine similarity for vector search (using
NumPy if it's installed) and BM25 for text search."""

from __future__ import annotations

import math
import re
from collections import defaultdict
from dataclasses import dataclass, field

from ..models import CodeChunk, Vector
from .base import SearchHit, VectorStore

try:
    import numpy as _np
except ImportError:  # pragma: no cover - exercised only when numpy absent
    _np = None  # type: ignore[assignment]

_TOKEN_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")

_BM25_K1 = 1.5
_BM25_B = 0.75


def _tokenize(text: str) -> list[str]:
    return [t.lower() for t in _TOKEN_RE.findall(text)]


def _cosine_py(q: Vector, v: Vector) -> float:
    dot = 0.0
    nq = 0.0
    nv = 0.0
    for a, b in zip(q, v):
        dot += a * b
        nq += a * a
        nv += b * b
    if nq == 0.0 or nv == 0.0:
        return 0.0
    return dot / (math.sqrt(nq) * math.sqrt(nv))


@dataclass
class _Entry:
    chunk: CodeChunk
    vector: Vector
    tokens: list[str]
    tf: dict[str, int]


@dataclass
class _Partition:
    """One project's index."""

    entries: dict[str, _Entry] = field(default_factory=dict)  # by chunk id
    by_path: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    df: dict[str, int] = field(default_factory=lambda: defaultdict(int))  # document frequency
    total_len: int = 0

    @property
    def n_docs(self) -> int:
        return len(self.entries)

    @property
    def avg_len(self) -> float:
        return (self.total_len / self.n_docs) if self.n_docs else 0.0


class InMemoryVectorStore(VectorStore):
    def __init__(self) -> None:
        self._parts: dict[str, _Partition] = defaultdict(_Partition)
        self._id_index: dict[str, str] = {}  # chunk id -> project, for get()

    def upsert(self, chunks: list[CodeChunk], vectors: list[Vector]) -> None:
        if len(chunks) != len(vectors):
            raise ValueError("chunks and vectors length mismatch")
        for chunk, vec in zip(chunks, vectors):
            part = self._parts[chunk.project]
            if chunk.id in part.entries:
                self._remove_entry(part, chunk.id)
            tokens = _tokenize(chunk.code) + _tokenize(chunk.symbol or "")
            tf: dict[str, int] = defaultdict(int)
            for tok in tokens:
                tf[tok] += 1
            entry = _Entry(chunk=chunk, vector=list(vec), tokens=tokens, tf=dict(tf))
            part.entries[chunk.id] = entry
            part.by_path[chunk.path].add(chunk.id)
            part.total_len += len(tokens)
            for tok in tf:
                part.df[tok] += 1
            self._id_index[chunk.id] = chunk.project

    def _remove_entry(self, part: _Partition, chunk_id: str) -> None:
        entry = part.entries.pop(chunk_id, None)
        if entry is None:
            return
        part.total_len -= len(entry.tokens)
        for tok in entry.tf:
            part.df[tok] -= 1
            if part.df[tok] <= 0:
                part.df.pop(tok, None)
        part.by_path[entry.chunk.path].discard(chunk_id)
        if not part.by_path[entry.chunk.path]:
            part.by_path.pop(entry.chunk.path, None)
        self._id_index.pop(chunk_id, None)

    def delete_by_file(self, project: str, path: str) -> int:
        part = self._parts.get(project)
        if not part:
            return 0
        ids = list(part.by_path.get(path, set()))
        for cid in ids:
            self._remove_entry(part, cid)
        return len(ids)

    def vector_search(self, project: str, qvec: Vector, k: int) -> list[SearchHit]:
        part = self._parts.get(project)
        if not part or not part.entries or k <= 0:
            return []
        entries = list(part.entries.values())
        if _np is not None:
            mat = _np.asarray([e.vector for e in entries], dtype=float)
            q = _np.asarray(qvec, dtype=float)
            qn = _np.linalg.norm(q)
            mn = _np.linalg.norm(mat, axis=1)
            denom = mn * (qn if qn else 1.0)
            denom[denom == 0.0] = 1.0
            sims = (mat @ q) / denom
            scores = [(float(s), e) for s, e in zip(sims, entries)]
        else:  # pragma: no cover
            scores = [(_cosine_py(qvec, e.vector), e) for e in entries]
        scores.sort(key=lambda x: (-x[0], x[1].chunk.id))
        return [SearchHit(chunk=e.chunk, score=s) for s, e in scores[:k]]

    def text_search(self, project: str, query: str, k: int) -> list[SearchHit]:
        part = self._parts.get(project)
        if not part or not part.entries or k <= 0:
            return []
        q_tokens = set(_tokenize(query))
        if not q_tokens:
            return []
        n = part.n_docs
        avg_len = part.avg_len or 1.0
        results: list[tuple[float, _Entry]] = []
        for entry in part.entries.values():
            doc_len = len(entry.tokens)
            score = 0.0
            for tok in q_tokens:
                tf = entry.tf.get(tok, 0)
                if tf == 0:
                    continue
                df = part.df.get(tok, 0) or 1
                idf = math.log(1 + (n - df + 0.5) / (df + 0.5))
                denom = tf + _BM25_K1 * (1 - _BM25_B + _BM25_B * doc_len / avg_len)
                score += idf * (tf * (_BM25_K1 + 1)) / (denom or 1.0)
            if score > 0.0:
                results.append((score, entry))
        results.sort(key=lambda x: (-x[0], x[1].chunk.id))
        return [SearchHit(chunk=e.chunk, score=s) for s, e in results[:k]]

    def get(self, chunk_id: str) -> CodeChunk | None:
        project = self._id_index.get(chunk_id)
        if project is None:
            return None
        entry = self._parts[project].entries.get(chunk_id)
        return entry.chunk if entry else None

    def paths(self, project: str) -> set[str]:
        part = self._parts.get(project)
        return set(part.by_path.keys()) if part else set()

    def count(self, project: str) -> int:
        part = self._parts.get(project)
        return part.n_docs if part else 0

    def projects(self) -> set[str]:
        return {p for p, part in self._parts.items() if part.entries}
