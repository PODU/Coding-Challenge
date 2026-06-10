"""Embedders.

``HashingEmbedder`` (the default) hashes tokens into ``dim`` signed buckets, so
texts that share words get a positive cosine. ``OllamaEmbedder`` calls a local
Ollama server running ``nomic-embed-text``.
"""

from __future__ import annotations

import numpy as np

from .text import stable_hash, tokenize


class HashingEmbedder:
    def __init__(self, dim: int = 256) -> None:
        self.dim = dim

    def _embed_one(self, text: str) -> list[float]:
        vec = np.zeros(self.dim, dtype=np.float64)
        for tok in tokenize(text):
            sign = 1.0 if stable_hash("sign:" + tok) & 1 else -1.0
            vec[stable_hash(tok) % self.dim] += sign
        norm = float(np.linalg.norm(vec))
        if norm > 0:
            vec /= norm
        return vec.tolist()

    def embed(self, texts: list[str]) -> list[list[float]]:
        return [self._embed_one(t) for t in texts]


class OllamaEmbedder:
    """Embeddings from a local Ollama server. Needs the ``[ollama]`` extra."""

    def __init__(self, model: str = "nomic-embed-text", host: str | None = None, dim: int = 768) -> None:
        import os

        self.model = model
        self.host = host or os.environ.get("OLLAMA_HOST", "http://localhost:11434")
        self.dim = dim

    def embed(self, texts: list[str]) -> list[list[float]]:  # pragma: no cover - prod seam
        import httpx

        out: list[list[float]] = []
        with httpx.Client(base_url=self.host, timeout=60) as client:
            for text in texts:
                resp = client.post("/api/embeddings", json={"model": self.model, "prompt": text})
                resp.raise_for_status()
                out.append(resp.json()["embedding"])
        return out
