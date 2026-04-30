from __future__ import annotations

import pytest

from support_bot.config import Config
from support_bot.embeddings import MockEmbeddingProvider
from support_bot.models import Chunk
from support_bot.retriever import Retriever
from support_bot.vectorstore import InMemoryVectorStore


@pytest.fixture
def config() -> Config:
    return Config()


@pytest.fixture
def kb_chunks() -> list[Chunk]:
    return [
        Chunk(
            chunk_id="c-redis",
            doc_id="challenges",
            title="Build your own Redis",
            source="challenges.md",
            text=(
                "Rebuild a Redis server that speaks the RESP protocol over TCP. "
                "Implement the PING SET GET and EXPIRE commands and add "
                "concurrency so the server handles many clients."
            ),
        ),
        Chunk(
            chunk_id="c-wc",
            doc_id="challenges",
            title="Build your own wc",
            source="challenges.md",
            text=(
                "Rebuild the Unix wc command line tool. Count bytes lines words "
                "and characters and support reading from standard input. This is "
                "a beginner challenge."
            ),
        ),
        Chunk(
            chunk_id="c-pricing",
            doc_id="faq",
            title="Pricing",
            source="faq.md",
            text=(
                "Creating an account and accessing the core challenges is free. "
                "A paid Pro subscription unlocks bonus challenges and official "
                "solutions."
            ),
        ),
    ]


@pytest.fixture
def make_retriever():
    def _make(chunks, *, threshold: float = 0.18, top_k: int = 4) -> Retriever:
        retriever = Retriever(
            MockEmbeddingProvider(),
            InMemoryVectorStore(),
            top_k=top_k,
            threshold=threshold,
        )
        retriever.index(chunks)
        return retriever

    return _make
