"""Pick backends from environment variables.

The defaults (SQLite, hashing embedder, scripted LLM) need nothing installed.
The others are imported only when selected.
"""

from __future__ import annotations

import os
from pathlib import Path

from .embedders import HashingEmbedder
from .interfaces import Embedder, LLMProvider, Store
from .llm import ScriptedLLM
from .store import SqliteStore
from .wiki import Wiki

DEFAULT_DIM = 256

# Relative to the repo rather than the CWD, so running the CLI from elsewhere
# doesn't leave wiki.db files lying around.
DEFAULT_PROJECTS_ROOT = Path(__file__).resolve().parents[2] / "projects"


def _projects_root() -> Path:
    return Path(os.environ.get("WIKI_PROJECTS_ROOT", str(DEFAULT_PROJECTS_ROOT)))


def build_store() -> Store:
    kind = os.environ.get("WIKI_STORE", "sqlite").lower()
    if kind == "sqlite":
        root = _projects_root()
        root.mkdir(parents=True, exist_ok=True)
        return SqliteStore(str(root / "wiki.db"))
    if kind == "oracle":  # pragma: no cover - production seam
        from .store.oracle_store import OracleStore

        return OracleStore()
    raise ValueError(f"unknown WIKI_STORE={kind!r}")


def build_embedder() -> Embedder:
    kind = os.environ.get("WIKI_EMBEDDER", "hashing").lower()
    if kind == "hashing":
        return HashingEmbedder(dim=int(os.environ.get("WIKI_EMBED_DIM", DEFAULT_DIM)))
    if kind == "ollama":  # pragma: no cover - production seam
        from .embedders import OllamaEmbedder

        return OllamaEmbedder()
    raise ValueError(f"unknown WIKI_EMBEDDER={kind!r}")


def build_llm() -> LLMProvider:
    kind = os.environ.get("WIKI_LLM", "scripted").lower()
    if kind == "scripted":
        return ScriptedLLM()
    if kind == "anthropic":  # pragma: no cover - production seam
        from .llm import AnthropicLLM

        return AnthropicLLM(model=os.environ.get("WIKI_LLM_MODEL", "claude-opus-4-8"))
    raise ValueError(f"unknown WIKI_LLM={kind!r}")


def build_wiki() -> Wiki:
    return Wiki(
        store=build_store(),
        embedder=build_embedder(),
        llm=build_llm(),
        projects_root=_projects_root(),
    )
