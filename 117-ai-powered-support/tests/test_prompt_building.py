"""Grounded-prompt building: KB context injected, history trimmed, no-context case."""

from __future__ import annotations

from support_bot.config import Config
from support_bot.models import Hit, Message
from support_bot.prompt_builder import PromptBuilder


def _hits(kb_chunks):
    return [Hit(chunk=c, score=0.9 - i * 0.1) for i, c in enumerate(kb_chunks)]


def test_system_prompt_has_grounding_and_offtopic_rules(config, kb_chunks):
    prompt = PromptBuilder(config).build([], _hits(kb_chunks))
    system = prompt.system
    assert "Coding Challenges support assistant" in system
    assert "ONLY" in system  # answer only from context
    assert config.abstain_sentinel in system  # abstention sentinel instruction
    assert "decline" in system.lower()  # off-topic rule
    assert "inject" in system.lower()  # prompt-injection resistance


def test_retrieved_chunks_appear_in_context_block_with_source_tags(config, kb_chunks):
    prompt = PromptBuilder(config).build([], _hits(kb_chunks))
    system = prompt.system
    assert "=== KNOWLEDGE BASE CONTEXT ===" in system
    for chunk in kb_chunks:
        assert chunk.text in system
        assert f"[Source: {chunk.title} | {chunk.source}]" in system


def test_history_included_and_truncated_oldest_first(kb_chunks):
    config = Config(max_history_tokens=12)  # tiny budget forces trimming
    history = [
        Message("user", "first question about the very first topic here please"),
        Message("assistant", "first answer body text here"),
        Message("user", "second and final current question"),
    ]
    prompt = PromptBuilder(config).build(history, _hits(kb_chunks))
    contents = [m.content for m in prompt.messages]
    # Current (last) question is always retained.
    assert "second and final current question" in contents
    # Oldest turn dropped first.
    assert "first question about the very first topic here please" not in contents


def test_history_under_budget_is_kept_intact(config, kb_chunks):
    history = [
        Message("user", "what is the wc challenge"),
        Message("assistant", "it rebuilds wc"),
        Message("user", "and the redis one"),
    ]
    prompt = PromptBuilder(config).build(history, _hits(kb_chunks))
    assert len(prompt.messages) == 3


def test_zero_hits_reflects_no_context_condition(config):
    prompt = PromptBuilder(config).build([], [])
    assert "no relevant knowledge base context was found" in prompt.system
    assert "[Source:" not in prompt.system
