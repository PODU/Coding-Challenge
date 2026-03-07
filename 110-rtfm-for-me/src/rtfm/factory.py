"""Build an agent from config, using Redis and the real LLM when asked to."""

from __future__ import annotations

import logging

from .agent import RTFMAgent
from .config import Config
from .embedder import Embedder, HashEmbedder, LocalEmbedder
from .llm import AnthropicClient, LLMClient, MockLLMClient
from .long_term_memory import InMemoryLongTermMemory, RedisLongTermMemory
from .semantic_cache import InMemorySemanticCache, RedisSemanticCache
from .session_memory import InMemorySessionMemory, RedisSessionMemory
from .vector_store import InMemoryVectorStore, RedisVectorStore

logger = logging.getLogger("rtfm.factory")


def build_embedder(cfg: Config, *, local: bool = False) -> Embedder:
    if local:
        try:
            return LocalEmbedder(cfg.embed_model)
        except Exception:  # noqa: BLE001
            logger.warning("local embedder unavailable; using HashEmbedder", exc_info=True)
    return HashEmbedder(dim=cfg.embed_dim)


def build_llm(cfg: Config, *, mock: bool = False) -> LLMClient:
    if mock:
        return MockLLMClient("(mock answer)")
    return AnthropicClient(cfg)


def build_agent(
    cfg: Config | None = None,
    *,
    use_redis: bool = False,
    mock_llm: bool = False,
    local_embedder: bool = False,
) -> RTFMAgent:
    """Build an agent. The default is HashEmbedder, in-memory storage and the
    Anthropic LLM; ``mock_llm`` is handy for a demo without an API key.
    """
    cfg = cfg or Config.from_env()
    embedder = build_embedder(cfg, local=local_embedder)
    llm = build_llm(cfg, mock=mock_llm)

    if use_redis:
        store = RedisVectorStore(embedder.dim, url=cfg.redis_url)
        cache = RedisSemanticCache(embedder.dim, url=cfg.redis_url, threshold=cfg.cache_threshold)
        session_memory = RedisSessionMemory(url=cfg.redis_url, ttl=cfg.session_ttl)
        long_term = RedisLongTermMemory(url=cfg.redis_url)
    else:
        store = InMemoryVectorStore(dim=embedder.dim)
        cache = InMemorySemanticCache(threshold=cfg.cache_threshold)
        session_memory = InMemorySessionMemory(ttl=cfg.session_ttl)
        long_term = InMemoryLongTermMemory()

    return RTFMAgent(
        cfg=cfg,
        embedder=embedder,
        store=store,
        llm=llm,
        cache=cache,
        session_memory=session_memory,
        long_term_memory=long_term,
    )
