"""Embedding + retrieval over the in-memory vector store."""

from __future__ import annotations

import pytest

from rtfm.embedder import HashEmbedder
from rtfm.ingest import ingest_documents
from rtfm.models import Chunk
from rtfm.vector_store import DimMismatchError, InMemoryVectorStore


def test_relevant_chunk_is_retrieved(cfg, embedder):
    store = InMemoryVectorStore(dim=embedder.dim)
    docs = {
        "git.md": "## COMMIT\nThe git commit command records changes to the repository.",
        "grep.md": "## OPTIONS\nThe -r flag reads all files under each directory recursively.",
    }
    ingest_documents(docs, embedder=embedder, store=store, cfg=cfg)

    q = embedder.embed_text("how do I recursively search directories")
    hits = store.knn(q, k=1)
    assert hits
    assert hits[0].chunk.filename == "grep.md"

    q2 = embedder.embed_text("record changes with a commit message")
    hits2 = store.knn(q2, k=1)
    assert hits2[0].chunk.filename == "git.md"


def test_hybrid_filter_restricts_to_one_file(cfg, embedder):
    store = InMemoryVectorStore(dim=embedder.dim)
    ingest_documents(
        {
            "git.md": "## LOG\ngit log shows the commit history with author and date.",
            "grep.md": "## OPTIONS\ngrep -n prefixes output with line numbers.",
        },
        embedder=embedder,
        store=store,
        cfg=cfg,
    )
    q = embedder.embed_text("history")
    hits = store.knn(q, k=5, filters={"filename": "grep.md"})
    assert hits
    assert all(h.chunk.filename == "grep.md" for h in hits)


def test_empty_store_returns_no_hits(embedder):
    store = InMemoryVectorStore(dim=embedder.dim)
    assert store.knn(embedder.embed_text("anything"), k=3) == []
    assert store.count() == 0


def test_dim_mismatch_is_rejected():
    store = InMemoryVectorStore(dim=8)
    bad = Chunk("f", "", 0, "x").with_embedding([0.0] * 4)
    with pytest.raises(DimMismatchError):
        store.upsert([bad])


def test_embedder_is_deterministic():
    e1, e2 = HashEmbedder(dim=64), HashEmbedder(dim=64)
    assert e1.embed_text("the quick brown fox") == e2.embed_text("the quick brown fox")
