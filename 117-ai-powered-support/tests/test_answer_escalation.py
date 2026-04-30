"""Answer handling and the escalation decision matrix (fully offline)."""

from __future__ import annotations

import pytest

from support_bot.config import Config
from support_bot.factory import build_bot
from support_bot.llm.mock import MockLLMClient
from support_bot.models import Answer, Escalation, EscalationReason
from support_bot.observability import InMemoryMetricsSink


def _bot(chunks, llm, *, config=None, metrics=None):
    return build_bot(
        config=config or Config(),
        llm=llm,
        chunks=chunks,
        metrics=metrics or InMemoryMetricsSink(),
    )


def test_context_plus_confident_answer_returns_grounded_answer_with_citations(kb_chunks):
    llm = MockLLMClient()  # default = confident when context is present
    bot = _bot(kb_chunks, llm)
    result = bot.turn("How do I build my own Redis server with the RESP protocol?")
    assert isinstance(result, Answer)
    assert result.citations, "a grounded answer should carry citations"
    assert any(c.chunk_id == "c-redis" for c in result.citations)
    assert len(llm.calls) == 1


def test_zero_hits_escalates_no_context_without_calling_llm(kb_chunks):
    llm = MockLLMClient()
    # Strict threshold => an on-topic query retrieves nothing -> NO_CONTEXT.
    bot = _bot(kb_chunks, llm, config=Config(similarity_threshold=0.95))
    result = bot.turn("How do I build my own Redis server?")
    assert isinstance(result, Escalation)
    assert result.reason is EscalationReason.NO_CONTEXT
    assert result.human_handoff is True
    assert llm.calls == [], "the LLM must not be called when there is no context"


def test_abstention_sentinel_escalates_low_confidence(kb_chunks):
    llm = MockLLMClient(abstain=True)
    bot = _bot(kb_chunks, llm)
    result = bot.turn("What Redis commands do I need to implement?")
    assert isinstance(result, Escalation)
    assert result.reason is EscalationReason.LOW_CONFIDENCE
    assert len(llm.calls) == 1  # LLM was consulted, then abstained


def test_off_topic_query_is_politely_declined(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)
    result = bot.turn("What is the best recipe for a chocolate birthday cake?")
    assert isinstance(result, Escalation)
    assert result.reason is EscalationReason.OFF_TOPIC
    assert result.human_handoff is False  # soft decline, not a human handoff
    assert llm.calls == []


def test_explicit_human_request_hands_off(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)
    result = bot.turn("This isn't working, I want to talk to a human agent.")
    assert isinstance(result, Escalation)
    assert result.reason is EscalationReason.HUMAN_REQUEST
    assert result.human_handoff is True
    assert llm.calls == []


def test_repeated_low_confidence_hits_repeat_limit(kb_chunks):
    llm = MockLLMClient(abstain=True)
    bot = _bot(kb_chunks, llm, config=Config(escalation_repeat_limit=3))
    queries = [
        "What Redis commands should I implement first?",
        "How does the Redis challenge handle concurrency?",
        "Which Redis protocol does the challenge use?",
    ]
    reasons = [bot.turn(q).reason for q in queries]
    assert reasons[0] is EscalationReason.LOW_CONFIDENCE
    assert reasons[1] is EscalationReason.LOW_CONFIDENCE
    assert reasons[2] is EscalationReason.REPEAT_LIMIT


def test_empty_input_does_not_call_llm(kb_chunks):
    llm = MockLLMClient()
    bot = _bot(kb_chunks, llm)
    result = bot.turn("   ")
    assert isinstance(result, Escalation)
    assert result.reason is EscalationReason.EMPTY_INPUT
    assert llm.calls == []
