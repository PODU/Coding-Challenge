"""Shared fixtures — every test is hermetic: in-memory SQLite, deterministic
hashing embedder, scripted LLM. No network, no Docker, no API key.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from llmwiki import HashingEmbedder, ScriptedLLM, SqliteStore, Wiki

CORPUS = Path(__file__).parent / "fixtures" / "corpus"


@pytest.fixture
def store() -> SqliteStore:
    return SqliteStore(":memory:")


@pytest.fixture
def embedder() -> HashingEmbedder:
    return HashingEmbedder(dim=64)


@pytest.fixture
def llm() -> ScriptedLLM:
    return ScriptedLLM()


@pytest.fixture
def wiki(store, embedder, llm, tmp_path) -> Wiki:
    return Wiki(store=store, embedder=embedder, llm=llm, projects_root=tmp_path)


@pytest.fixture
def corpus_dir() -> Path:
    return CORPUS


# A few well-known fixture documents for focused tests.
ADA_DOC = (
    "Ada Lovelace was a pioneer of computing. "
    "Ada Lovelace is credited with the first algorithm in 1843. "
    "The Analytical Engine was designed by Charles Babbage."
)
TURING_DOC = (
    "Alan Turing was a pioneer of computing. "
    "Alan Turing is credited with the Turing Machine model in 1936. "
    "Alan Turing influenced theoretical computer science."
)
