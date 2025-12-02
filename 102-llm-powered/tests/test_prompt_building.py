from chatbot.history import ConversationHistory
from chatbot.llm import Message
from chatbot.mock_client import MockLLMClient
from chatbot.persona import BUILT_IN, Persona
from chatbot.session import ChatSession


def _session(client, persona, *, max_tokens=1024, max_turns=20):
    return ChatSession(
        client, persona, ConversationHistory(max_turns=max_turns), max_tokens=max_tokens
    )


def test_sends_personas_exact_system_prompt():
    client = MockLLMClient(["ok"])
    persona = BUILT_IN["pirate"]
    s = _session(client, persona)
    s.send("ahoy")
    assert client.calls[0]["system"] == persona.system_prompt


def test_custom_system_prompt_overrides_persona():
    client = MockLLMClient(["ok"])
    persona = Persona("custom", "You are a custom test persona.")
    s = _session(client, persona)
    s.send("hi")
    assert client.calls[0]["system"] == "You are a custom test persona."


def test_sends_full_windowed_message_list_in_order():
    client = MockLLMClient(["r1", "r2"])
    s = _session(client, BUILT_IN["assistant"])
    s.send("first")
    s.send("second")
    # second call must carry the whole prior conversation, in order
    sent = client.calls[1]["messages"]
    assert sent == [
        Message("user", "first"),
        Message("assistant", "r1"),
        Message("user", "second"),
    ]


def test_max_tokens_is_forwarded():
    client = MockLLMClient(["ok"])
    s = _session(client, BUILT_IN["assistant"], max_tokens=256)
    s.send("hi")
    assert client.calls[0]["max_tokens"] == 256


def test_window_limits_messages_sent():
    client = MockLLMClient(["a", "b", "c"])
    s = _session(client, BUILT_IN["assistant"], max_turns=1)  # keep last 2 messages
    s.send("one")    # history: [u:one, a:a]
    s.send("two")    # history: [u:one, a:a, u:two, a:b]
    s.send("three")  # window before this reply: last 2 = [a:b, u:three]
    sent = client.calls[-1]["messages"]
    # the raw tail slice would be [a:b, u:three], which starts on an assistant
    # turn and the API rejects with a 400 — the leading assistant is trimmed so
    # the sent list is a valid user-first sequence
    assert sent == [Message("user", "three")]
    assert sent[0].role == "user"
