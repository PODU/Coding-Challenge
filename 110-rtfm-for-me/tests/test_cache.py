"""Semantic cache hit/miss behavior (in-memory vector store + mock LLM)."""

from __future__ import annotations


def test_first_ask_misses_then_caches(agent, mock_llm):
    result = agent.ask("How do I commit changes in git?")
    assert result.cached is False
    assert len(mock_llm.calls) == 1
    assert agent.cache.stats.misses == 1
    assert agent.cache.stats.hits == 0


def test_duplicate_ask_hits_cache_and_skips_llm(agent, mock_llm):
    agent.ask("How do I commit changes in git?")
    calls_after_first = len(mock_llm.calls)

    # Near-duplicate (same words, different casing/punctuation) -> cosine 1.0 >= threshold.
    result = agent.ask("how do i commit changes in git")
    assert result.cached is True
    assert len(mock_llm.calls) == calls_after_first  # LLM not called again
    assert agent.cache.stats.hits == 1


def test_below_threshold_ask_misses_and_calls_llm(agent, mock_llm):
    agent.ask("How do I commit changes in git?")
    calls_after_first = len(mock_llm.calls)

    result = agent.ask("What does the grep recursive flag do?")
    assert result.cached is False
    assert len(mock_llm.calls) == calls_after_first + 1


def test_cache_does_not_store_no_answer(agent, mock_llm):
    # A question with no matching docs returns a no-answer and is not cached.
    agent.store.clear()
    result = agent.ask("totally unrelated question about nothing")
    assert result.answer.found is False
    # Nothing was stored, so a repeat is still a miss (and still hits no LLM
    # because retrieval is empty).
    assert agent.cache.stats.hits == 0
