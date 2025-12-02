import io

from chatbot.cli import repl
from chatbot.history import ConversationHistory
from chatbot.llm import LLMRateLimitError
from chatbot.mock_client import MockLLMClient
from chatbot.persona import BUILT_IN
from chatbot.session import ChatSession


def _session(client, max_turns=20):
    return ChatSession(
        client, BUILT_IN["assistant"], ConversationHistory(max_turns=max_turns)
    )


def _run(lines, client, *, stream=True):
    """Run the REPL against scripted input lines, return captured output."""
    in_ = io.StringIO("".join(line + "\n" for line in lines))
    out = io.StringIO()
    code = repl(_session(client), stream=stream, out=out, in_=in_)
    return code, out.getvalue()


def test_basic_turn_prints_bot_output_streaming():
    client = MockLLMClient(["hello there"])
    code, text = _run(["hi", "/exit"], client)
    assert code == 0
    assert "hello there" in text
    assert "bot>" in text
    assert "bye" in text


def test_non_streaming_turn():
    client = MockLLMClient(["plain reply"])
    code, text = _run(["hi", "/exit"], client, stream=False)
    assert code == 0
    assert "plain reply" in text


def test_reset_clears_history():
    client = MockLLMClient(["r1", "r2"])
    session = _session(client)
    in_ = io.StringIO("hi\n/reset\nbye\n/exit\n")
    out = io.StringIO()
    code = repl(session, out=out, in_=in_)
    assert code == 0
    assert "history cleared" in out.getvalue()
    # after /reset only the second exchange remains
    contents = [m.content for m in session.history.messages()]
    assert "hi" not in contents
    assert "bye" in contents


def test_help_command():
    client = MockLLMClient(["x"])
    _, text = _run(["/help", "/exit"], client)
    assert "/reset" in text and "/exit" in text


def test_empty_input_is_skipped():
    client = MockLLMClient(["should-not-be-called"])
    code, text = _run(["", "   ", "/exit"], client)
    assert code == 0
    # no API call was made for blank lines
    assert client.calls == []


def test_exit_returns_zero():
    client = MockLLMClient(["x"])
    code, _ = _run(["/exit"], client)
    assert code == 0


def test_quit_alias_returns_zero():
    client = MockLLMClient(["x"])
    code, _ = _run(["/quit"], client)
    assert code == 0


def test_eof_exits_cleanly():
    # empty input stream → readline returns "" immediately (Ctrl-D)
    client = MockLLMClient(["x"])
    out = io.StringIO()
    code = repl(_session(client), out=out, in_=io.StringIO(""))
    assert code == 0
    assert "bye" in out.getvalue()


class _KeyboardInterruptStream:
    """A stream whose readline raises KeyboardInterrupt (simulates Ctrl-C)."""

    def readline(self):
        raise KeyboardInterrupt


def test_keyboard_interrupt_exits_cleanly():
    client = MockLLMClient(["x"])
    out = io.StringIO()
    code = repl(_session(client), out=out, in_=_KeyboardInterruptStream())
    assert code == 0
    assert "bye" in out.getvalue()


def test_rate_limit_prints_friendly_message_and_loop_survives():
    client = MockLLMClient(raise_on_call=LLMRateLimitError("429"))
    code, text = _run(["hi", "/exit"], client)
    assert code == 0
    assert "rate limited" in text.lower()
    # loop survived the error and processed the later /exit cleanly
    assert "bye" in text


def test_auth_error_message():
    from chatbot.llm import LLMAuthError

    client = MockLLMClient(raise_on_call=LLMAuthError("401"))
    _, text = _run(["hi", "/exit"], client)
    assert "auth error" in text.lower()


def test_empty_response_message():
    client = MockLLMClient(empty=True)
    _, text = _run(["hi", "/exit"], client)
    assert "no response" in text.lower()


def test_service_error_message():
    from chatbot.llm import LLMServiceError

    client = MockLLMClient(raise_on_call=LLMServiceError("boom"))
    _, text = _run(["hi", "/exit"], client)
    assert "service error" in text.lower()
