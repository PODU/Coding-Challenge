"""Session (short-term) memory: ordering, isolation, TTL via a fake clock."""

from __future__ import annotations

from rtfm.models import Turn
from rtfm.session_memory import InMemorySessionMemory


class FakeClock:
    def __init__(self) -> None:
        self.now = 1000.0

    def __call__(self) -> float:
        return self.now


def test_append_and_recent_preserve_order():
    mem = InMemorySessionMemory()
    for i in range(5):
        mem.append("s1", Turn(question=f"q{i}", answer=f"a{i}"))
    recent = mem.recent("s1", 3)
    assert [t.question for t in recent] == ["q2", "q3", "q4"]


def test_sessions_do_not_leak_into_each_other():
    mem = InMemorySessionMemory()
    mem.append("s1", Turn("q-a", "a"))
    mem.append("s2", Turn("q-b", "b"))
    assert [t.question for t in mem.recent("s1", 10)] == ["q-a"]
    assert [t.question for t in mem.recent("s2", 10)] == ["q-b"]


def test_ttl_expiry_with_fake_clock():
    clock = FakeClock()
    mem = InMemorySessionMemory(ttl=100, clock=clock)
    mem.append("s1", Turn("q", "a"))
    assert mem.recent("s1", 10)  # fresh

    clock.now += 50
    assert mem.recent("s1", 10)  # still within TTL

    clock.now += 100  # now 150s since write -> expired
    assert mem.recent("s1", 10) == []


def test_clear_one_and_all():
    mem = InMemorySessionMemory()
    mem.append("s1", Turn("q", "a"))
    mem.append("s2", Turn("q", "a"))
    mem.clear("s1")
    assert mem.recent("s1", 10) == []
    assert mem.recent("s2", 10)
    mem.clear()
    assert mem.recent("s2", 10) == []
