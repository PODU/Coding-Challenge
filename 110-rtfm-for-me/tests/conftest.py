"""Shared fixtures. The whole suite runs offline: no API key, no network, no Redis."""

from __future__ import annotations

import pytest

from rtfm.agent import RTFMAgent
from rtfm.config import Config
from rtfm.embedder import HashEmbedder
from rtfm.ingest import ingest_documents
from rtfm.llm import MockLLMClient

SAMPLE_DOCS = {
    "git.md": (
        "# git\n\n## COMMIT\n"
        "The git commit command records changes to the repository. "
        "Use git commit -m message to record a commit with the given message. "
        "The --amend flag replaces the tip of the current branch with a new commit.\n\n"
        "## LOG\n"
        "The git log command shows the commit history. "
        "The --oneline flag condenses each commit to a single line.\n"
    ),
    "grep.md": (
        "# grep\n\n## OPTIONS\n"
        "The -i flag ignores case distinctions in patterns and input data. "
        "The -r flag reads all files under each directory recursively. "
        "The -v flag inverts the sense of matching to select non-matching lines.\n"
    ),
}


@pytest.fixture(autouse=True)
def _no_api_key(monkeypatch):
    """Guard: the offline suite must never need a real key. Remove it if present
    so a stray real API call would fail loudly rather than silently bill."""
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    monkeypatch.delenv("ANTHROPIC_AUTH_TOKEN", raising=False)
    assert Config.anthropic_api_key() is None


@pytest.fixture
def cfg() -> Config:
    # Small chunks so the sample docs split into several chunks during tests.
    return Config(embed_dim=256, chunk_tokens=40, chunk_overlap=8, top_k=3, cache_threshold=0.95)


@pytest.fixture
def embedder(cfg) -> HashEmbedder:
    return HashEmbedder(dim=cfg.embed_dim)


@pytest.fixture
def mock_llm() -> MockLLMClient:
    return MockLLMClient("Use `git commit -m` to record a commit. [1]")


@pytest.fixture
def agent(cfg, embedder, mock_llm) -> RTFMAgent:
    a = RTFMAgent.in_memory(mock_llm, cfg=cfg, embedder=embedder)
    ingest_documents(SAMPLE_DOCS, embedder=a.embedder, store=a.store, cfg=cfg)
    return a
