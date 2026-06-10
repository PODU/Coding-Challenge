"""llmwiki — Database-Driven LLM Wiki (Coding Challenge #123).

Public API:

    from llmwiki import Wiki, SqliteStore, HashingEmbedder, ScriptedLLM
"""

from __future__ import annotations

from .domain import (
    Answer,
    Contradiction,
    EmbeddingDimMismatch,
    EmptyDocumentError,
    Finding,
    Frontmatter,
    IngestResult,
    Operation,
    Source,
    WikiPage,
)
from .embedders import HashingEmbedder, OllamaEmbedder
from .interfaces import Embedder, LLMProvider, LLMResult, Msg, Store
from .llm import AnthropicLLM, ScriptedLLM
from .store import SqliteStore
from .wiki import ProjectNotFound, Wiki

__version__ = "0.1.0"

__all__ = [
    "Wiki",
    "SqliteStore",
    "HashingEmbedder",
    "OllamaEmbedder",
    "ScriptedLLM",
    "AnthropicLLM",
    "Store",
    "Embedder",
    "LLMProvider",
    "LLMResult",
    "Msg",
    "Frontmatter",
    "WikiPage",
    "Source",
    "Operation",
    "Contradiction",
    "Finding",
    "IngestResult",
    "Answer",
    "EmbeddingDimMismatch",
    "EmptyDocumentError",
    "ProjectNotFound",
    "__version__",
]
