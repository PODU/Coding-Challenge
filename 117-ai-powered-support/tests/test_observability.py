"""Observability: per-turn TurnLog fields and the /stats aggregation."""

from __future__ import annotations

from support_bot.config import Config
from support_bot.factory import build_bot
from support_bot.llm.mock import MockLLMClient
from support_bot.observability import InMemoryMetricsSink, estimate_cost_usd


def _bot(chunks, llm, metrics):
    return build_bot(config=Config(), llm=llm, chunks=chunks, metrics=metrics)


def test_turnlog_is_populated_for_an_answered_turn(kb_chunks):
    metrics = InMemoryMetricsSink()
    bot = _bot(kb_chunks, MockLLMClient(default_model="claude-opus-4-8"), metrics)
    bot.turn("How do I build my own Redis server with the RESP protocol?")

    assert len(metrics.logs) == 1
    log = metrics.logs[0]
    assert log.request_id and log.timestamp
    assert log.model == "claude-opus-4-8"
    assert log.input_tokens > 0 and log.output_tokens > 0
    assert log.latency_ms >= 0.0
    assert log.estimated_cost_usd > 0.0
    assert log.finish_reason == "end_turn"
    assert log.escalated is False
    assert log.escalation_reason is None
    assert log.classification in {"simple", "complex"}


def test_pre_llm_escalation_logs_zero_tokens(kb_chunks):
    metrics = InMemoryMetricsSink()
    bot = _bot(kb_chunks, MockLLMClient(), metrics)
    bot.turn("best pizza recipe please")  # off-topic, no LLM call
    log = metrics.logs[0]
    assert log.escalated is True
    assert log.escalation_reason == "OFF_TOPIC"
    assert log.input_tokens == 0 and log.output_tokens == 0
    assert log.estimated_cost_usd == 0.0


def test_stats_dashboard_aggregates(kb_chunks):
    metrics = InMemoryMetricsSink()
    bot = _bot(kb_chunks, MockLLMClient(), metrics)
    bot.turn("How do I build my own Redis server?")       # answer
    bot.turn("best pizza recipe please")                  # off-topic escalation
    bot.turn("I want to talk to a human agent.")          # human escalation

    stats = metrics.stats()
    assert stats["turns"] == 3
    assert stats["answers"] == 1
    assert stats["escalations"] == 2
    assert 0.0 < stats["escalation_rate"] < 1.0
    assert stats["by_reason"].get("OFF_TOPIC") == 1
    assert stats["by_reason"].get("HUMAN_REQUEST") == 1
    assert stats["total_input_tokens"] >= 0


def test_cost_estimation_matches_price_table():
    # claude-opus-4-8 is $5 / $25 per 1M tokens.
    cost = estimate_cost_usd("claude-opus-4-8", 1_000_000, 1_000_000)
    assert cost == 30.0
