"""Embedder backed by Nomic CodeRankEmbed (or any sentence-transformer).

``sentence_transformers`` is an optional dependency, so it's imported only
when this class is used.
"""

from __future__ import annotations

from ..models import Vector
from .base import Embedder

_MODEL_NAME = "nomic-ai/CodeRankEmbed"


class CodeRankEmbedder(Embedder):
    def __init__(self, dim: int = 768, model_name: str = _MODEL_NAME) -> None:
        try:
            from sentence_transformers import SentenceTransformer
        except ImportError as exc:  # pragma: no cover - optional dep
            raise RuntimeError(
                "CodeRankEmbedder requires `sentence-transformers`. "
                "Install it or set SHERPA_EMBEDDER=mock."
            ) from exc
        self._model = SentenceTransformer(model_name, trust_remote_code=True)
        self.dim = self._model.get_sentence_embedding_dimension() or dim

    def embed(self, texts: list[str]) -> list[Vector]:  # pragma: no cover - needs model
        vectors = self._model.encode(
            texts, normalize_embeddings=True, convert_to_numpy=True
        )
        return [v.tolist() for v in vectors]
