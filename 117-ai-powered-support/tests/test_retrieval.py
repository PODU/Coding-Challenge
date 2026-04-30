"""KB ingest + retrieval: relevant chunk for a query, threshold, top-k, idempotency."""

from __future__ import annotations

from support_bot.config import Config
from support_bot.embeddings import MockEmbeddingProvider
from support_bot.ingest import build_chunks, chunk_document, load_kb
from support_bot.vectorstore import InMemoryVectorStore


def test_relevant_query_returns_expected_chunk_ranked_first(make_retriever, kb_chunks):
    retriever = make_retriever(kb_chunks)
    hits = retriever.search("How do I build my own Redis server with the RESP protocol?")
    assert hits, "expected at least one relevant hit"
    assert hits[0].chunk.chunk_id == "c-redis"
    # ranked by descending score
    scores = [h.score for h in hits]
    assert scores == sorted(scores, reverse=True)


def test_below_threshold_query_returns_no_hits(make_retriever, kb_chunks):
    retriever = make_retriever(kb_chunks)
    hits = retriever.search("I enjoy baking chocolate cake and eating pizza on weekends")
    assert hits == []


def test_top_k_is_honoured(make_retriever, kb_chunks):
    retriever = make_retriever(kb_chunks, threshold=0.0)
    hits = retriever.search("challenge", k=1)
    assert len(hits) == 1


def test_threshold_is_honoured(make_retriever, kb_chunks):
    loose = make_retriever(kb_chunks, threshold=0.0)
    strict = make_retriever(kb_chunks, threshold=0.95)
    query = "Tell me about the Redis challenge"
    assert len(loose.search(query)) >= len(strict.search(query))
    assert strict.search(query) == []  # nothing clears a 0.95 bar


def test_ingestion_is_idempotent(make_retriever, kb_chunks):
    retriever = make_retriever(kb_chunks)
    assert len(retriever.store) == len(kb_chunks)
    retriever.index(kb_chunks)  # re-ingest identical content
    assert len(retriever.store) == len(kb_chunks)  # no duplicates


def test_chunking_is_deterministic_and_idempotent():
    config = Config()
    docs = load_kb(config.kb_path)
    assert docs, "packaged KB should load some documents"
    first = build_chunks(docs, config)
    second = build_chunks(docs, config)
    assert [c.chunk_id for c in first] == [c.chunk_id for c in second]

    # Upserting twice does not duplicate.
    store = InMemoryVectorStore()
    embedder = MockEmbeddingProvider()
    vectors = embedder.embed([c.text for c in first])
    store.upsert(first, vectors)
    store.upsert(first, vectors)
    assert len(store) == len(first)


def test_chunk_carries_source_metadata():
    config = Config()
    docs = load_kb(config.kb_path)
    chunks = build_chunks(docs, config)
    sample = chunks[0]
    assert sample.title and sample.source.endswith(".md") and sample.doc_id
