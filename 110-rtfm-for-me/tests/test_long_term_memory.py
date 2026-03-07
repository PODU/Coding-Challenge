"""Long-term memory: durable fact extraction, recall, dedupe."""

from __future__ import annotations

from rtfm.long_term_memory import InMemoryLongTermMemory, extract_facts


def test_extract_common_durable_facts():
    facts = extract_facts("I prefer Go. I work on the billing service. I use vim.")
    joined = " | ".join(facts)
    assert "prefers Go" in joined
    assert "works on the billing service" in joined
    assert "uses vim" in joined


def test_no_facts_from_plain_question():
    assert extract_facts("How do I commit in git?") == []


def test_remember_and_recall_dedupes():
    mem = InMemoryLongTermMemory()
    added = mem.remember("u1", "I prefer Go")
    assert added == ["prefers Go"]
    again = mem.remember("u1", "I prefer Go")
    assert again == []  # already known
    assert mem.recall("u1") == ["prefers Go"]


def test_users_are_isolated():
    mem = InMemoryLongTermMemory()
    mem.remember("u1", "I prefer Go")
    mem.remember("u2", "I prefer Rust")
    assert mem.recall("u1") == ["prefers Go"]
    assert mem.recall("u2") == ["prefers Rust"]
