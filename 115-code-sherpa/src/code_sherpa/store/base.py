"""VectorStore interface + the SearchHit value object."""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass

from ..models import CodeChunk, Vector


@dataclass(frozen=True)
class SearchHit:
    chunk: CodeChunk
    score: float  # cosine similarity or BM25, depending on the search


class VectorStore(ABC):
    """Chunk storage with vector and full-text search, partitioned by project."""

    @abstractmethod
    def upsert(self, chunks: list[CodeChunk], vectors: list[Vector]) -> None:
        """Insert or replace chunks (and their vectors) by chunk id."""

    @abstractmethod
    def delete_by_file(self, project: str, path: str) -> int:
        """Remove all chunks for ``path`` and return how many there were."""

    @abstractmethod
    def vector_search(self, project: str, qvec: Vector, k: int) -> list[SearchHit]:
        """Top k chunks by cosine similarity."""

    @abstractmethod
    def text_search(self, project: str, query: str, k: int) -> list[SearchHit]:
        """Top k chunks by BM25."""

    @abstractmethod
    def get(self, chunk_id: str) -> CodeChunk | None:
        ...

    @abstractmethod
    def paths(self, project: str) -> set[str]:
        ...

    @abstractmethod
    def count(self, project: str) -> int:
        ...

    def projects(self) -> set[str]:  # pragma: no cover
        raise NotImplementedError
