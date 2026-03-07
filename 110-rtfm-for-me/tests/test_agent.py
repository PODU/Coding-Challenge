"""End-to-end agent behavior, including graceful degradation."""

from __future__ import annotations

from rtfm.agent import RTFMAgent
from rtfm.ingest import ingest_documents
from rtfm.llm import MockLLMClient
from rtfm.models import Answer
from rtfm.prompt_builder import NOT_FOUND_SENTINEL
from rtfm.vector_store import InMemoryVectorStore


def test_ask_returns_grounded_cited_answer(agent, mock_llm):
    result = agent.ask("How do I commit changes?")
    assert result.answer.found
    assert result.answer.text
    assert result.answer.citations  # the scripted answer cites [1]
    assert result.retrieved  # real chunks were retrieved
    assert mock_llm.calls  # the LLM was consulted


def test_empty_retrieval_never_calls_llm(cfg, embedder):
    mock = MockLLMClient("should never be called")
    a = RTFMAgent.in_memory(mock, cfg=cfg, embedder=embedder)
    # No documents ingested -> empty store.
    result = a.ask("anything at all")
    assert result.answer.found is False
    assert result.answer.text == NOT_FOUND_SENTINEL
    assert result.answer.citations == []
    assert mock.calls == []  # the LLM was NOT called with fabricated context


def test_not_in_docs_sentinel_flows_through(cfg, embedder):
    mock = MockLLMClient(NOT_FOUND_SENTINEL)
    a = RTFMAgent.in_memory(mock, cfg=cfg, embedder=embedder)
    ingest_documents(
        {"git.md": "## COMMIT\ngit commit records changes to the repository."},
        embedder=a.embedder,
        store=a.store,
        cfg=cfg,
    )
    result = a.ask("what is the airspeed velocity of an unladen swallow?")
    assert result.answer.found is False
    assert result.answer.citations == []


def test_graceful_degradation_when_cache_raises(cfg, embedder):
    class RaisingCache:
        def lookup(self, embedding):
            raise RuntimeError("cache down")

        def store(self, embedding, answer):
            raise RuntimeError("cache down")

        @property
        def stats(self):  # pragma: no cover - not used in this test
            raise RuntimeError("cache down")

        def clear(self):  # pragma: no cover
            pass

    mock = MockLLMClient("Use git commit. [1]")
    a = RTFMAgent(
        cfg=cfg,
        embedder=embedder,
        store=InMemoryVectorStore(dim=embedder.dim),
        llm=mock,
        cache=RaisingCache(),
    )
    ingest_documents(
        {"git.md": "## COMMIT\ngit commit records changes to the repository."},
        embedder=a.embedder,
        store=a.store,
        cfg=cfg,
    )
    result = a.ask("how do I commit?")
    assert result.answer.found  # still answered via the normal path
    assert "cache" in result.degraded  # degradation recorded, not fatal


def test_session_memory_is_used_for_followups(agent):
    captured: list[str] = []

    def record(system: str, user: str) -> str:
        captured.append(user)
        return "Yes, use git log --oneline. [1]"

    agent.llm = MockLLMClient(record)
    agent.ask("How do I view the commit log?", session_id="s1")
    agent.ask("what about its oneline flag?", session_id="s1")

    # The second prompt should carry the recent conversation turn.
    assert "Recent conversation" in captured[1]
    assert "How do I view the commit log?" in captured[1]


def test_long_term_memory_personalizes_prompt(agent):
    captured: list[str] = []

    def record(system: str, user: str) -> str:
        captured.append(user)
        return "Here is how. [1]"

    agent.llm = MockLLMClient(record)
    # First ask states a durable fact; it is remembered for the user.
    agent.ask("I prefer Go and I want to commit", user_id="u1")
    agent.ask("how do I commit?", user_id="u1")

    assert any("prefers Go" in u for u in captured)


def test_reset_clears_everything(agent):
    agent.ask("How do I commit changes?")
    assert agent.store.count() > 0
    agent.reset()
    assert agent.store.count() == 0
    assert agent.cache.stats.hits == 0 and agent.cache.stats.misses == 0


def test_cached_answer_marked_cached(agent):
    first = agent.ask("How do I commit changes?")
    assert first.cached is False
    second = agent.ask("How do I commit changes?")
    assert second.cached is True
    assert isinstance(second.answer, Answer)
