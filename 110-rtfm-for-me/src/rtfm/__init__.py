"""RTFM For Me: answers questions about a set of docs, with citations.

The LLM and the embedding model sit behind small interfaces so the tests can
run offline against fakes.
"""

from __future__ import annotations

from .config import Config
from .models import (
    Answer,
    AskResult,
    CacheStats,
    Chunk,
    Citation,
    LLMResponse,
    RetrievedChunk,
    Turn,
)

__all__ = [
    "Config",
    "Answer",
    "AskResult",
    "CacheStats",
    "Chunk",
    "Citation",
    "LLMResponse",
    "RetrievedChunk",
    "Turn",
    "__version__",
]

__version__ = "0.1.0"
