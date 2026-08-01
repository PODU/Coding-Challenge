"""Persistent sessions and the memory bank."""

from coach.agent import CoachAgent
from coach.memory import MemoryNote
from coach.mock_client import MockLLMClient
from coach.sessions import SessionStore


def test_session_persists_and_resumes(tmp_path, registry):
    store = SessionStore(tmp_path / "s")
    session = store.create("dev1", "abc123")
    agent = CoachAgent(
        MockLLMClient(["Try the wc challenge first."]),
        registry,
        session=session,
        session_store=store,
    )
    agent.turn("where should I start?")

    # New process: reload from disk.
    store2 = SessionStore(tmp_path / "s")
    resumed = store2.load("abc123")
    assert [m.role for m in resumed.history] == ["user", "assistant"]
    assert resumed.history[1].content == "Try the wc challenge first."

    # Continue the conversation without losing context.
    client = MockLLMClient(["Next: add the -l flag."])
    agent2 = CoachAgent(client, registry, session=resumed, session_store=store2)
    agent2.turn("done with -c, what next?")
    sent = client.calls[0]["messages"]
    assert sent[0].content == "where should I start?"  # old turn still there
    assert len(sent) == 3


def test_memory_dedupes_and_recalls(memory):
    note = MemoryNote(kind="advice", text="Start with the -c flag", session_id="s1")
    assert memory.remember("dev1", note) is True
    assert memory.remember("dev1", note) is False  # duplicate skipped
    assert memory.already_advised("dev1", "start with the  -c FLAG")
    assert [n.text for n in memory.recall("dev1")] == ["Start with the -c flag"]
    assert memory.recall("other-user") == []


def test_prior_advice_injected_prevents_repetition(sessions, memory, registry):
    # Session 1: coach gives advice; it lands in the memory bank.
    s1 = sessions.create("dev1")
    agent1 = CoachAgent(
        MockLLMClient(["Read the file in binary mode for -c."]),
        registry,
        session=s1,
        session_store=sessions,
        memory=memory,
    )
    agent1.turn("hints for wc -c?")
    assert memory.already_advised("dev1", "Read the file in binary mode for -c.")

    # Session 2 (fresh session, same user): prior advice is in the system
    # prompt under an explicit do-not-repeat instruction.
    s2 = sessions.create("dev1")
    client2 = MockLLMClient(["Since you already know -c, let's tackle -l."])
    agent2 = CoachAgent(
        client2,
        registry,
        session=s2,
        session_store=sessions,
        memory=memory,
    )
    agent2.turn("remind me about wc")
    system = client2.calls[0]["system"]
    assert "Read the file in binary mode for -c." in system
    assert "do not repeat" in system
