"""The question-answering pipeline.

    cache lookup -> retrieve -> memory -> prompt -> LLM -> format -> persist

The cache and both memories are optional extras. If one of them raises, the
question is still answered and the component is listed in
``AskResult.degraded``.
"""

from __future__ import annotations

import logging
from typing import Optional

from .answer_formatter import format_answer
from .config import Config
from .embedder import Embedder, HashEmbedder
from .llm import LLMClient
from .long_term_memory import InMemoryLongTermMemory, MemoryStore
from .models import Answer, AskResult, Turn
from .prompt_builder import build_prompt
from .retriever import Retriever
from .semantic_cache import InMemorySemanticCache, SemanticCache
from .session_memory import InMemorySessionMemory, SessionMemory
from .vector_store import InMemoryVectorStore, VectorStore

logger = logging.getLogger("rtfm.agent")

NO_DOCS_ANSWER = "I couldn't find this in the indexed documentation."


class RTFMAgent:

    def __init__(
        self,
        *,
        cfg: Config,
        embedder: Embedder,
        store: VectorStore,
        llm: LLMClient,
        cache: Optional[SemanticCache] = None,
        session_memory: Optional[SessionMemory] = None,
        long_term_memory: Optional[MemoryStore] = None,
    ) -> None:
        self.cfg = cfg
        self.embedder = embedder
        self.store = store
        self.llm = llm
        self.cache = cache
        self.session_memory = session_memory
        self.long_term_memory = long_term_memory
        self.retriever = Retriever(embedder, store)

    @classmethod
    def in_memory(
        cls,
        llm: LLMClient,
        *,
        cfg: Optional[Config] = None,
        embedder: Optional[Embedder] = None,
    ) -> "RTFMAgent":
        """Build an agent that keeps everything in memory (no Redis)."""
        cfg = cfg or Config()
        embedder = embedder or HashEmbedder(dim=cfg.embed_dim)
        store = InMemoryVectorStore(dim=embedder.dim)
        return cls(
            cfg=cfg,
            embedder=embedder,
            store=store,
            llm=llm,
            cache=InMemorySemanticCache(threshold=cfg.cache_threshold),
            session_memory=InMemorySessionMemory(ttl=cfg.session_ttl),
            long_term_memory=InMemoryLongTermMemory(),
        )

    def ask(
        self,
        question: str,
        *,
        session_id: str | None = None,
        user_id: str | None = None,
        filename: str | None = None,
    ) -> AskResult:
        """Answer ``question`` from the indexed docs.

        ``filename`` limits retrieval to one source file. Passing
        ``session_id`` or ``user_id`` turns on session or long-term memory.
        """
        degraded: list[str] = []
        embedding = self.retriever.embed_question(question)

        if self.cache is not None:
            try:
                cached = self.cache.lookup(embedding)
                if cached is not None:
                    self._record_user(user_id, question, degraded)
                    return AskResult(answer=cached, cached=True, degraded=degraded)
            except Exception:  # noqa: BLE001
                logger.warning("semantic cache unavailable; degrading", exc_info=True)
                degraded.append("cache")

        filters = {"filename": filename} if filename else None
        retrieved = self.retriever.retrieve(
            question, k=self.cfg.top_k, filters=filters, embedding=embedding
        )

        # Don't ask the LLM anything without context; it would just make
        # something up.
        if not retrieved:
            answer = Answer(text=NO_DOCS_ANSWER, citations=[], found=False)
            self._record_user(user_id, question, degraded)
            return AskResult(answer=answer, cached=False, degraded=degraded, retrieved=[])

        recent_turns: list[Turn] = []
        if session_id and self.session_memory is not None:
            try:
                recent_turns = self.session_memory.recent(session_id, self.cfg.max_session_turns)
            except Exception:  # noqa: BLE001
                logger.warning("session memory unavailable; degrading", exc_info=True)
                degraded.append("session_memory")

        memories: list[str] = []
        if user_id and self.long_term_memory is not None:
            try:
                memories = self.long_term_memory.recall(user_id)
            except Exception:  # noqa: BLE001
                logger.warning("long-term memory unavailable; degrading", exc_info=True)
                degraded.append("long_term_memory")

        prompt = build_prompt(
            question,
            retrieved,
            recent_turns=recent_turns,
            long_term_memories=memories,
        )
        response = self.llm.complete(system=prompt.system, user=prompt.user)
        answer = format_answer(response, retrieved)

        # Only remember answers that actually answered something.
        if answer.found and not answer.refused:
            if self.cache is not None and "cache" not in degraded:
                try:
                    self.cache.store(embedding, answer)
                except Exception:  # noqa: BLE001
                    logger.warning("cache store failed; degrading", exc_info=True)
                    degraded.append("cache")
            if session_id and self.session_memory is not None and "session_memory" not in degraded:
                try:
                    self.session_memory.append(session_id, Turn(question=question, answer=answer.text))
                except Exception:  # noqa: BLE001
                    logger.warning("session append failed; degrading", exc_info=True)
                    degraded.append("session_memory")

        self._record_user(user_id, question, degraded)
        return AskResult(answer=answer, cached=False, degraded=degraded, retrieved=retrieved)

    def _record_user(self, user_id: str | None, question: str, degraded: list[str]) -> None:
        """Pass the question to long-term memory to pick out user facts."""
        if not user_id or self.long_term_memory is None or "long_term_memory" in degraded:
            return
        try:
            self.long_term_memory.remember(user_id, question)
        except Exception:  # noqa: BLE001
            logger.warning("long-term memory write failed; degrading", exc_info=True)
            degraded.append("long_term_memory")

    def reset(self) -> None:
        """Clear the index, the cache and all memory."""
        self.store.clear()
        if self.cache is not None:
            try:
                self.cache.clear()
            except Exception:  # noqa: BLE001
                logger.warning("cache clear failed", exc_info=True)
        if self.session_memory is not None:
            try:
                self.session_memory.clear()
            except Exception:  # noqa: BLE001
                logger.warning("session clear failed", exc_info=True)
        if self.long_term_memory is not None:
            try:
                self.long_term_memory.clear()
            except Exception:  # noqa: BLE001
                logger.warning("long-term clear failed", exc_info=True)
