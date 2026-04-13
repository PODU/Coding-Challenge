"""Hybrid retrieval: vector / text / fused rankings, isolation, fallback."""

from __future__ import annotations

import pytest

from code_sherpa.embed.mock import MockEmbedder
from code_sherpa.ingest.chunker import CodeChunker
from code_sherpa.retrieve.fusion import reciprocal_rank_fusion
from code_sherpa.retrieve.hybrid import HybridRetriever
from code_sherpa.store.memory_store import InMemoryVectorStore

from conftest import AUTH_PY, PAYMENTS_PY


def index(store, embedder, project, files):
    chunker = CodeChunker()
    for path, lang, src in files:
        chunks = chunker.chunk_source(project, path, lang, src, "sha-" + path)
        vectors = embedder.embed([c.code for c in chunks])
        store.upsert(chunks, vectors)


@pytest.fixture
def indexed():
    store = InMemoryVectorStore()
    embedder = MockEmbedder(dim=128)
    index(
        store,
        embedder,
        "demo",
        [("src/payments.py", "python", PAYMENTS_PY), ("src/auth.py", "python", AUTH_PY)],
    )
    return store, embedder


def test_text_search_finds_payment_failure(indexed):
    store, _ = indexed
    hits = store.text_search("demo", "handle payment failure", 5)
    assert hits
    assert any(h.chunk.symbol == "PaymentService.handle_payment_failure" for h in hits)


def test_vector_search_returns_results(indexed):
    store, embedder = indexed
    qv = embedder.embed_one("authenticate a user login password")
    hits = store.vector_search("demo", qv, 5)
    assert hits
    assert any(h.chunk.symbol == "login" for h in hits)


def test_hybrid_ranks_relevant_chunk_first(indexed):
    store, embedder = indexed
    retriever = HybridRetriever(store=store, embedder=embedder)
    results = retriever.retrieve("demo", "where do we handle payment failures?", k=5)
    assert results
    top_symbols = [r.chunk.symbol for r in results[:3]]
    assert "PaymentService.handle_payment_failure" in top_symbols


def test_text_only_fallback_when_no_embedder(indexed):
    store, _ = indexed
    retriever = HybridRetriever(store=store, embedder=None)
    results = retriever.retrieve("demo", "login user password", k=5)
    assert results
    assert all(r.vector_rank is None for r in results)
    assert any(r.chunk.symbol == "login" for r in results)


def test_project_isolation():
    store = InMemoryVectorStore()
    embedder = MockEmbedder(dim=128)
    index(store, embedder, "A", [("a.py", "python", PAYMENTS_PY)])
    index(store, embedder, "B", [("b.py", "python", AUTH_PY)])
    retriever = HybridRetriever(store=store, embedder=embedder)
    res_a = retriever.retrieve("A", "refund payment failure", k=10)
    assert res_a
    assert all(r.chunk.project == "A" for r in res_a)
    assert all(r.chunk.path == "a.py" for r in res_a)


def test_rrf_fusion_math():
    fused = reciprocal_rank_fusion(
        {"vector": ["x", "y", "z"], "text": ["y", "x"]}, k=60
    )
    # 'y' is rank2 vector + rank1 text; 'x' is rank1 vector + rank2 text.
    scores = {it.key: it.score for it in fused}
    assert scores["y"] == pytest.approx(1 / 62 + 1 / 61)
    assert scores["x"] == pytest.approx(1 / 61 + 1 / 62)
    assert fused[0].key in {"x", "y"}


def test_empty_query_returns_nothing(indexed):
    store, embedder = indexed
    retriever = HybridRetriever(store=store, embedder=embedder)
    assert retriever.retrieve("demo", "   ", k=5) == []
