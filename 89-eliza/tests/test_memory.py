"""Memory queue + deferred recall tests."""

from __future__ import annotations

from eliza.engine import ElizaEngine


def test_memory_push_on_my_keyword(engine: ElizaEngine) -> None:
    reply = engine.respond("My boss is always shouting at me")
    # A normal 'my' reply is returned (round-robin starts at template 0).
    assert reply == "Your boss is always shouting at you?"
    # ... and the reflected memory sentence is queued.
    assert len(engine.memory) == 1
    assert engine.memory[0] == "Earlier you said your boss is always shouting at you."


def test_no_keyword_pops_memory(engine: ElizaEngine) -> None:
    engine.respond("My boss is always shouting at me")
    recalled = engine.respond("qwerty")
    assert recalled == "Earlier you said your boss is always shouting at you."
    assert len(engine.memory) == 0


def test_empty_memory_uses_xnone(engine: ElizaEngine) -> None:
    assert engine.respond("blorptang") == "I see."
    assert engine.respond("flibberty") == "Please go on."


def test_memory_queue_is_bounded(engine: ElizaEngine) -> None:
    for _ in range(10):
        engine.respond("my head hurts")
    assert len(engine.memory) <= engine.script.memory_size
