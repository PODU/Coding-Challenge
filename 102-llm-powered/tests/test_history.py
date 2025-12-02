from chatbot.history import ConversationHistory
from chatbot.llm import Message


def test_add_order_and_roles():
    h = ConversationHistory()
    h.add_user("hi")
    h.add_assistant("hello")
    h.add_user("bye")
    msgs = h.messages()
    assert msgs == [
        Message("user", "hi"),
        Message("assistant", "hello"),
        Message("user", "bye"),
    ]


def test_first_message_is_user_and_alternates():
    h = ConversationHistory()
    h.add_user("a")
    h.add_assistant("b")
    h.add_user("c")
    h.add_assistant("d")
    msgs = h.messages()
    assert msgs[0].role == "user"
    roles = [m.role for m in msgs]
    assert roles == ["user", "assistant", "user", "assistant"]


def test_sliding_window_trims_to_two_times_max_turns():
    h = ConversationHistory(max_turns=2)  # keep last 4 messages
    for i in range(5):
        h.add_user(f"u{i}")
        h.add_assistant(f"a{i}")
    msgs = h.messages()
    assert len(msgs) == 4
    assert [m.content for m in msgs] == ["u3", "a3", "u4", "a4"]
    # first message in the window must still be a user turn (valid alternation)
    assert msgs[0].role == "user"


def test_window_on_odd_length_history_starts_on_user():
    # messages() is called mid-send, right after add_user, so the list is often
    # odd-length ending on a user turn. A naive tail slice of 2*max_turns would
    # then start on an assistant turn and the API would 400.
    h = ConversationHistory(max_turns=2)  # window = last 4 messages
    for i in range(3):
        h.add_user(f"u{i}")
        h.add_assistant(f"a{i}")
    h.add_user("u3")  # odd length now: [u0,a0,u1,a1,u2,a2,u3]
    msgs = h.messages()
    assert msgs[0].role == "user"
    # a leading assistant is trimmed, so the window may be shorter than 2*turns
    roles = [m.role for m in msgs]
    assert roles == ["user", "assistant", "user"]
    assert [m.content for m in msgs] == ["u2", "a2", "u3"]


def test_unbounded_window():
    h = ConversationHistory(max_turns=None)
    for i in range(10):
        h.add_user(f"u{i}")
        h.add_assistant(f"a{i}")
    assert len(h.messages()) == 20


def test_messages_returns_a_copy():
    h = ConversationHistory()
    h.add_user("hi")
    out = h.messages()
    out.append(Message("assistant", "tampered"))
    assert len(h) == 1  # internal state untouched


def test_clear_empties():
    h = ConversationHistory()
    h.add_user("hi")
    h.add_assistant("hello")
    h.clear()
    assert h.messages() == []
    assert len(h) == 0
