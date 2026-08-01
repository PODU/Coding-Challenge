"""Tracing spans and policy-based challenge filtering."""

from coach.agent import CoachAgent
from coach.mock_client import MockLLMClient
from coach.policy import ChallengeInfo, ChallengePolicy
from coach.telemetry import Tracer

CHALLENGES = [
    ChallengeInfo("wc", "beginner", ("cli",)),
    ChallengeInfo("json-parser", "intermediate", ("parsing",)),
    ChallengeInfo("huffman", "advanced", ("compression",)),
    ChallengeInfo("miner", "advanced", ("crypto-mining",)),
]


def test_policy_difficulty_ceiling():
    policy = ChallengePolicy(max_difficulty="intermediate")
    allowed = policy.filter(list(CHALLENGES))
    assert [c.name for c in allowed] == ["wc", "json-parser"]


def test_policy_blocked_topics():
    policy = ChallengePolicy(blocked_topics=frozenset({"crypto-mining"}))
    allowed = policy.filter(list(CHALLENGES))
    assert "miner" not in [c.name for c in allowed]
    assert len(allowed) == 3


def test_enforcement_recorded_as_span_events():
    tracer = Tracer()
    policy = ChallengePolicy(max_difficulty="beginner")
    with tracer.span("recommend") as span:
        policy.filter(list(CHALLENGES), span=span)
    events = tracer.finished[0].events
    assert len(events) == len(CHALLENGES)
    denied = [e for e in events if not e["attributes"]["allowed"]]
    assert {e["attributes"]["challenge"] for e in denied} == {
        "json-parser",
        "huffman",
        "miner",
    }
    assert all(e["name"] == "policy.enforcement" for e in events)


def test_agent_turn_emits_nested_spans(sessions, registry, tmp_path):
    tracer = Tracer()
    session = sessions.create("dev1")
    agent = CoachAgent(
        MockLLMClient(["hello"]), registry, session=session, tracer=tracer
    )
    agent.turn("hi")
    names = [s.name for s in tracer.finished]
    assert "llm.request" in names and "agent.turn" in names
    llm_span = next(s for s in tracer.finished if s.name == "llm.request")
    assert llm_span.parent == "agent.turn"
    # exportable
    out = tmp_path / "traces.jsonl"
    tracer.export_jsonl(out)
    assert out.read_text().count("\n") == len(names)
