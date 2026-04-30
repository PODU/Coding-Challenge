"""Builds a :class:`SupportBot`. Tests pass in mocks; the CLI gets the real
Claude client when ``ANTHROPIC_API_KEY`` is set."""

from __future__ import annotations

import os

from .bot import SupportBot
from .config import Config
from .conversation import Conversation
from .embeddings import EmbeddingProvider, MockEmbeddingProvider
from .escalation import EscalationPolicy
from .ingest import build_chunks, load_kb
from .llm.base import LLMClient
from .llm.fallback import FallbackLLMClient
from .llm.mock import MockLLMClient
from .llm.routing import QueryClassifier, RoutingLLMClient
from .models import Chunk
from .observability import InMemoryMetricsSink, MetricsSink
from .prompt_builder import PromptBuilder
from .retriever import Retriever
from .vectorstore import InMemoryVectorStore, VectorStore


def default_llm(config: Config) -> LLMClient:
    """Routed Claude with a fallback model if there's an API key, else the mock."""
    if os.environ.get("ANTHROPIC_API_KEY"):
        from .llm.claude import ClaudeClient

        primary = ClaudeClient(model=config.model, max_tokens=config.max_tokens)
        secondary = ClaudeClient(model=config.cheap_model, max_tokens=config.max_tokens)
        chain = FallbackLLMClient([primary, secondary])
        return RoutingLLMClient(
            chain, simple_model=config.cheap_model, complex_model=config.model
        )
    return MockLLMClient()


def build_bot(
    *,
    config: Config | None = None,
    llm: LLMClient | None = None,
    embedder: EmbeddingProvider | None = None,
    store: VectorStore | None = None,
    metrics: MetricsSink | None = None,
    conversation: Conversation | None = None,
    chunks: list[Chunk] | None = None,
    kb_path: str | None = None,
) -> SupportBot:
    config = config or Config.from_env()
    embedder = embedder or MockEmbeddingProvider(dim=config.embedding_dim)
    store = store or InMemoryVectorStore()
    retriever = Retriever(
        embedder,
        store,
        top_k=config.retrieval_top_k,
        threshold=config.similarity_threshold,
    )
    if chunks is None:
        docs = load_kb(kb_path or config.kb_path)
        chunks = build_chunks(docs, config)
    retriever.index(chunks)

    metrics = metrics or InMemoryMetricsSink()
    llm = llm if llm is not None else default_llm(config)

    return SupportBot(
        retriever=retriever,
        prompt_builder=PromptBuilder(config),
        llm=llm,
        policy=EscalationPolicy(config),
        classifier=QueryClassifier(),
        metrics=metrics,
        config=config,
        conversation=conversation,
    )
