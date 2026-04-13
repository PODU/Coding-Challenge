"""Shared fixtures: wires the fully-offline mock trio + a fixture repo.

Everything here is deterministic and network-free: MockEmbedder, InMemoryVector
Store, MockLLMClient, InMemoryMemoryStore.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from code_sherpa.config import SherpaConfig
from code_sherpa.core.sherpa import Sherpa
from code_sherpa.embed.base import Embedder
from code_sherpa.embed.mock import MockEmbedder
from code_sherpa.ingest.chunker import CodeChunker
from code_sherpa.ingest.walker import RepoWalker
from code_sherpa.llm.mock import MockLLMClient
from code_sherpa.memory.in_memory import InMemoryMemoryStore
from code_sherpa.models import Vector
from code_sherpa.project.registry import ProjectRegistry
from code_sherpa.prompt.builder import PromptBuilder
from code_sherpa.prompt.select import ContextSelector
from code_sherpa.retrieve.classify import QueryClassifier
from code_sherpa.retrieve.hybrid import HybridRetriever
from code_sherpa.store.memory_store import InMemoryVectorStore


class SpyEmbedder(Embedder):
    """Wraps an embedder and records how many texts it embedded."""

    def __init__(self, inner: Embedder) -> None:
        self._inner = inner
        self.dim = inner.dim
        self.texts: list[str] = []
        self.batches = 0

    def embed(self, texts: list[str]) -> list[Vector]:
        self.batches += 1
        self.texts.extend(texts)
        return self._inner.embed(texts)


@pytest.fixture
def config() -> SherpaConfig:
    cfg = SherpaConfig(llm="mock", embedder="mock", store="memory", embed_dim=128)
    cfg.validate()
    return cfg


@pytest.fixture
def embedder() -> MockEmbedder:
    return MockEmbedder(dim=128)


@pytest.fixture
def store() -> InMemoryVectorStore:
    return InMemoryVectorStore()


@pytest.fixture
def memory() -> InMemoryMemoryStore:
    return InMemoryMemoryStore()


@pytest.fixture
def make_sherpa(config, tmp_path):
    """Factory returning a Sherpa wired with offline components.

    Optional overrides let a test inject e.g. a scripted/erroring MockLLMClient or
    a SpyEmbedder.
    """

    def _make(llm=None, embedder=None, store=None, memory=None) -> Sherpa:
        emb = embedder or MockEmbedder(dim=config.embed_dim)
        st = store or InMemoryVectorStore()
        mem = memory or InMemoryMemoryStore()
        return Sherpa(
            walker=RepoWalker(max_file_bytes=config.max_file_bytes),
            chunker=CodeChunker(
                max_chunk_chars=config.max_chunk_chars,
                min_chunk_chars=config.min_chunk_chars,
                window_size=config.window_size,
                window_overlap=config.window_overlap,
            ),
            embedder=emb,
            store=st,
            retriever=HybridRetriever(store=st, embedder=emb, rrf_k=config.rrf_k),
            selector=ContextSelector(char_budget=config.context_char_budget),
            builder=PromptBuilder(),
            llm=llm or MockLLMClient(),
            memory=mem,
            registry=ProjectRegistry(str(tmp_path / "registry.json")),
            classifier=QueryClassifier(base_top_k=config.top_k),
            config=config,
        )

    return _make


PAYMENTS_PY = '''\
"""Payment processing."""

import logging

LOGGER = logging.getLogger(__name__)
RETRY_LIMIT = 3


def charge(amount, card):
    """Charge a card for an amount."""
    return {"ok": True, "amount": amount}


class PaymentService:
    """Coordinates charges and refunds."""

    def __init__(self, gateway):
        self.gateway = gateway

    def refund(self, transaction_id):
        """Issue a refund for a transaction."""
        return self.gateway.refund(transaction_id)

    def handle_payment_failure(self, error):
        """Handle a failed payment by logging and scheduling a retry."""
        LOGGER.error("payment failed: %s", error)
        return self.schedule_retry(error)

    def schedule_retry(self, error):
        return {"retry": True, "reason": str(error)}
'''

AUTH_PY = '''\
"""Authentication helpers."""


def login(username, password):
    """Authenticate a user with username and password."""
    if not username or not password:
        raise ValueError("missing credentials")
    return {"user": username, "token": "abc"}


def logout(token):
    """Invalidate a session token."""
    return True
'''

NOTES_TXT = "This project handles payments and authentication.\nSee the docs.\n"

UNKNOWN_SRC = "BEGIN\n  do_thing()\n  do_other()\nEND\n"


@pytest.fixture
def fixture_repo(tmp_path) -> Path:
    """A tiny multi-file repo: two Python modules, a text file, an unknown-lang
    file, and a binary file that must be skipped."""

    root = tmp_path / "repo"
    (root / "src").mkdir(parents=True)
    (root / "src" / "payments.py").write_text(PAYMENTS_PY, encoding="utf-8")
    (root / "src" / "auth.py").write_text(AUTH_PY, encoding="utf-8")
    (root / "NOTES.txt").write_text(NOTES_TXT, encoding="utf-8")
    (root / "weird.xyz").write_text(UNKNOWN_SRC, encoding="utf-8")
    (root / "blob.bin").write_bytes(b"\x00\x01\x02binary\x00stuff")
    (root / ".gitignore").write_text("ignored_dir/\n*.log\n", encoding="utf-8")
    ignored = root / "ignored_dir"
    ignored.mkdir()
    (ignored / "secret.py").write_text("SECRET = 1\n", encoding="utf-8")
    (root / "build").mkdir()
    (root / "build" / "artifact.py").write_text("x = 1\n", encoding="utf-8")
    (root / "app.log").write_text("noise\n", encoding="utf-8")
    return root
