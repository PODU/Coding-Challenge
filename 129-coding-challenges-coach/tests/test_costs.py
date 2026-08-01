"""Model routing, cost tracking, per-session caps."""

import pytest

from coach.agent import CoachAgent
from coach.costs import (
    CAPABLE_MODEL,
    LIGHT_MODEL,
    CostCapExceeded,
    CostTracker,
    pick_model,
)
from coach.llm import Usage
from coach.mock_client import MockLLMClient


def test_routing_light_for_chat_capable_for_code():
    assert pick_model("what should I learn next?") == LIGHT_MODEL
    assert pick_model("my code throws a traceback") == CAPABLE_MODEL
    assert pick_model("anything", wants_tools_likely=True) == CAPABLE_MODEL


def test_per_request_and_session_tracking():
    tracker = CostTracker()
    r1 = tracker.record(LIGHT_MODEL, Usage(1000, 500))
    r2 = tracker.record(CAPABLE_MODEL, Usage(1000, 500))
    assert r2.usd > r1.usd  # capable model costs more for same tokens
    assert tracker.session_tokens == (2000, 1000)
    assert tracker.summary()["requests"] == 2
    assert tracker.session_usd == pytest.approx(r1.usd + r2.usd)


def test_cap_raises():
    tracker = CostTracker(cap_usd=0.0001)
    tracker.record(CAPABLE_MODEL, Usage(1_000_000, 0))  # $3
    with pytest.raises(CostCapExceeded):
        tracker.check_cap()


def test_agent_stops_before_next_call_when_capped(sessions, registry):
    session = sessions.create("dev1")
    tracker = CostTracker(cap_usd=0.000001)
    client = MockLLMClient(["first reply", "second reply"])
    agent = CoachAgent(
        client, registry, session=session, session_store=sessions, costs=tracker
    )
    agent.turn("hello")  # succeeds, spends tokens
    with pytest.raises(CostCapExceeded):
        agent.turn("hello again")
    assert len(client.calls) == 1  # second model call never happened


def test_agent_records_usage(sessions, registry):
    session = sessions.create("dev1")
    tracker = CostTracker()
    agent = CoachAgent(
        MockLLMClient(["ok"]),
        registry,
        session=session,
        costs=tracker,
    )
    agent.turn("hi")
    assert tracker.summary()["requests"] == 1
    assert tracker.session_tokens == (100, 50)
