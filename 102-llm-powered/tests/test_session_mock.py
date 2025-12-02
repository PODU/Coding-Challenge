from chatbot.history import ConversationHistory
from chatbot.llm import Message
from chatbot.mock_client import MockLLMClient
from chatbot.persona import BUILT_IN
from chatbot.session import ChatSession


def _session(client):
    return ChatSession(client, BUILT_IN["assistant"], ConversationHistory())


def test_send_returns_mock_text():
    client = MockLLMClient(["Hello from the mock model."])
    s = _session(client)
    assert s.send("hi") == "Hello from the mock model."


def test_send_commits_one_user_and_one_assistant():
    client = MockLLMClient(["reply"])
    s = _session(client)
    s.send("hi")
    msgs = s.history.messages()
    assert msgs == [Message("user", "hi"), Message("assistant", "reply")]


def test_multi_turn_carries_prior_turns():
    client = MockLLMClient(["r1", "r2"])
    s = _session(client)
    s.send("first")
    s.send("second")
    # final history holds both full turns
    assert s.history.messages() == [
        Message("user", "first"),
        Message("assistant", "r1"),
        Message("user", "second"),
        Message("assistant", "r2"),
    ]
    # and the model remembered turn 1 when answering turn 2
    assert Message("user", "first") in client.calls[1]["messages"]


def test_replies_cycle_across_turns():
    client = MockLLMClient(["a", "b"])
    s = _session(client)
    assert s.send("1") == "a"
    assert s.send("2") == "b"
    assert s.send("3") == "a"  # cycles back
