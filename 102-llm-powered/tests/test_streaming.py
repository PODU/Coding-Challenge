from chatbot.history import ConversationHistory
from chatbot.llm import Message
from chatbot.mock_client import MockLLMClient
from chatbot.persona import BUILT_IN
from chatbot.session import ChatSession


def _session(client):
    return ChatSession(client, BUILT_IN["assistant"], ConversationHistory())


def test_stream_yields_deltas_in_order_and_concatenates_to_reply():
    reply = "the quick brown fox"
    client = MockLLMClient([reply])
    s = _session(client)
    deltas = list(s.send_stream("hi"))
    assert len(deltas) > 1  # actually streamed in pieces
    assert "".join(deltas) == reply


def test_assistant_message_committed_once_after_stream_completes():
    client = MockLLMClient(["one two three"])
    s = _session(client)
    # while iterating, the assistant turn is not yet committed
    gen = s.send_stream("hi")
    next(gen)
    assert s.history.messages() == [Message("user", "hi")]
    # drain the rest
    list(gen)
    assert s.history.messages() == [
        Message("user", "hi"),
        Message("assistant", "one two three"),
    ]


def test_stream_records_call_shape():
    client = MockLLMClient(["hello world"])
    s = _session(client)
    list(s.send_stream("ping"))
    call = client.calls[0]
    assert call["system"] == BUILT_IN["assistant"].system_prompt
    assert call["messages"] == [Message("user", "ping")]


def test_multi_turn_streaming_history():
    client = MockLLMClient(["first reply", "second reply"])
    s = _session(client)
    assert "".join(s.send_stream("a")) == "first reply"
    assert "".join(s.send_stream("b")) == "second reply"
    assert s.history.messages() == [
        Message("user", "a"),
        Message("assistant", "first reply"),
        Message("user", "b"),
        Message("assistant", "second reply"),
    ]
