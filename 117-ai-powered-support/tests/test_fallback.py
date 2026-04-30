"""Multi-provider fallback + graceful degradation through the orchestrator."""

from __future__ import annotations

import pytest

from support_bot.config import Config
from support_bot.factory import build_bot
from support_bot.llm.base import AllProvidersFailed, TransientLLMError
from support_bot.llm.fallback import FallbackLLMClient
from support_bot.llm.mock import MockLLMClient, scripted
from support_bot.models import Escalation, EscalationReason, Message, Prompt
from support_bot.observability import InMemoryMetricsSink


def _prompt() -> Prompt:
    return Prompt(system="sys", messages=[Message("user", "hi")])


def test_primary_fails_secondary_serves_and_normalises():
    primary = MockLLMClient(error=TransientLLMError("boom"))
    secondary = scripted("served by secondary")
    chain = FallbackLLMClient([primary, secondary])

    resp = chain.complete(_prompt())
    assert resp.text == "served by secondary"
    assert resp.fallback_used is True
    assert resp.attempts == 2


def test_all_providers_failing_raises_all_providers_failed():
    chain = FallbackLLMClient(
        [MockLLMClient(error=TransientLLMError("a")), MockLLMClient(error=RuntimeError("b"))]
    )
    with pytest.raises(AllProvidersFailed):
        chain.complete(_prompt())


def test_bot_reports_fallback_used_in_turnlog(kb_chunks):
    metrics = InMemoryMetricsSink()
    chain = FallbackLLMClient([MockLLMClient(error=TransientLLMError("down")), MockLLMClient()])
    bot = build_bot(config=Config(), llm=chain, chunks=kb_chunks, metrics=metrics)

    bot.turn("How do I build my own Redis server with the RESP protocol?")
    assert metrics.logs[-1].fallback_used is True


def test_all_providers_failing_escalates_gracefully(kb_chunks):
    metrics = InMemoryMetricsSink()
    chain = FallbackLLMClient(
        [MockLLMClient(error=TransientLLMError("x")), MockLLMClient(error=TransientLLMError("y"))]
    )
    bot = build_bot(config=Config(), llm=chain, chunks=kb_chunks, metrics=metrics)

    result = bot.turn("How do I build my own Redis server with the RESP protocol?")
    assert isinstance(result, Escalation)  # graceful — no crash
    assert result.reason is EscalationReason.PROVIDER_ERROR
    assert metrics.logs[-1].escalated is True
