"""Oracle 23ai store (stub).

The plan is VECTOR columns plus Oracle Text for hybrid search. Nothing is
implemented yet beyond connecting. ``oracledb`` is optional and only imported
here.
"""

from __future__ import annotations

from ..models import CodeChunk, Vector
from .base import SearchHit, VectorStore


class OracleVectorStore(VectorStore):  # pragma: no cover - requires a live DB
    def __init__(self, dsn: str | None, user: str | None, password: str | None) -> None:
        try:
            import oracledb
        except ImportError as exc:
            raise RuntimeError(
                "OracleVectorStore requires `oracledb`. Install it or set "
                "SHERPA_STORE=memory."
            ) from exc
        if not dsn:
            raise RuntimeError("Oracle store requires SHERPA_DB_DSN.")
        self._pool = oracledb.create_pool(user=user, password=password, dsn=dsn)
        self._ensure_schema()

    def _ensure_schema(self) -> None:
        raise NotImplementedError(
            "OracleVectorStore is an adapter stub. Implement DDL for a CHUNKS table "
            "with a VECTOR column + Oracle Text index, then VECTOR_DISTANCE / "
            "CONTAINS queries fused at the SQL layer."
        )

    def upsert(self, chunks: list[CodeChunk], vectors: list[Vector]) -> None:
        raise NotImplementedError

    def delete_by_file(self, project: str, path: str) -> int:
        raise NotImplementedError

    def vector_search(self, project: str, qvec: Vector, k: int) -> list[SearchHit]:
        raise NotImplementedError

    def text_search(self, project: str, query: str, k: int) -> list[SearchHit]:
        raise NotImplementedError

    def get(self, chunk_id: str) -> CodeChunk | None:
        raise NotImplementedError

    def paths(self, project: str) -> set[str]:
        raise NotImplementedError

    def count(self, project: str) -> int:
        raise NotImplementedError
