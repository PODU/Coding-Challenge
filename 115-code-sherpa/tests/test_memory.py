"""Episodic + semantic memory store behaviour."""

from __future__ import annotations

from code_sherpa.memory.base import Episode, SemanticFact
from code_sherpa.memory.in_memory import InMemoryMemoryStore, make_episode


def test_episodes_recent_order_and_isolation():
    mem = InMemoryMemoryStore()
    mem.add_episode(make_episode("p1", "q1", "a1", []))
    mem.add_episode(make_episode("p1", "q2", "a2", []))
    mem.add_episode(make_episode("p2", "other", "a", []))

    recent = mem.recent_episodes("p1", limit=5)
    assert [e.question for e in recent] == ["q2", "q1"]  # most recent first
    assert mem.recent_episodes("p2")[0].question == "other"
    assert len(mem.recent_episodes("p2")) == 1  # isolation


def test_episode_lexical_recall():
    mem = InMemoryMemoryStore()
    mem.add_episode(make_episode("p", "how do refunds work", "...", []))
    mem.add_episode(make_episode("p", "how does login work", "...", []))
    hits = mem.search_episodes("p", "refunds failing", limit=3)
    assert hits
    assert hits[0].question == "how do refunds work"


def test_semantic_facts_and_relevance():
    mem = InMemoryMemoryStore()
    mem.add_fact(SemanticFact("p", "gateway", "we use Stripe for payments"))
    mem.add_fact(SemanticFact("p", "auth", "JWT tokens expire in 1h"))

    assert {f.key for f in mem.facts("p")} == {"gateway", "auth"}
    rel = mem.relevant_facts("p", "how are payments charged", limit=5)
    assert rel and rel[0].key == "gateway"


def test_fact_upsert_by_key():
    mem = InMemoryMemoryStore()
    mem.add_fact(SemanticFact("p", "k", "v1"))
    mem.add_fact(SemanticFact("p", "k", "v2"))
    facts = mem.facts("p")
    assert len(facts) == 1 and facts[0].value == "v2"


def test_empty_query_recall_is_empty():
    mem = InMemoryMemoryStore()
    mem.add_episode(make_episode("p", "q", "a", []))
    assert mem.search_episodes("p", "") == []
    assert mem.relevant_facts("p", "") == []
