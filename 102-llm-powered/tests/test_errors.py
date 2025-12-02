import pytest

from chatbot.history import ConversationHistory
from chatbot.llm import (
    LLMAuthError,
    LLMEmptyResponse,
    LLMRateLimitError,
    LLMServiceError,
    Message,
)
from chatbot.mock_client import MockLLMClient
from chatbot.persona import BUILT_IN
from chatbot.session import ChatSession


def _session(client):
    return ChatSession(client, BUILT_IN["assistant"], ConversationHistory())


@pytest.mark.parametrize(
    "exc",
    [LLMAuthError("x"), LLMRateLimitError("x"), LLMServiceError("x")],
)
def test_complete_propagates_raised_errors(exc):
    s = _session(MockLLMClient(raise_on_call=exc))
    with pytest.raises(type(exc)):
        s.send("hi")


@pytest.mark.parametrize(
    "exc",
    [LLMAuthError("x"), LLMRateLimitError("x"), LLMServiceError("x")],
)
def test_stream_propagates_raised_errors(exc):
    s = _session(MockLLMClient(raise_on_call=exc))
    with pytest.raises(type(exc)):
        list(s.send_stream("hi"))


def test_empty_flag_raises_empty_response_on_complete():
    s = _session(MockLLMClient(empty=True))
    with pytest.raises(LLMEmptyResponse):
        s.send("hi")


def test_empty_flag_raises_empty_response_on_stream():
    s = _session(MockLLMClient(empty=True))
    with pytest.raises(LLMEmptyResponse):
        list(s.send_stream("hi"))


def test_failed_turn_rolls_back_user_message_complete():
    s = _session(MockLLMClient(raise_on_call=LLMRateLimitError("429")))
    with pytest.raises(LLMRateLimitError):
        s.send("hi")
    # the unanswered user turn is rolled back so a retry cannot produce two
    # consecutive user turns (which would violate role alternation)
    assert s.history.messages() == []


def test_failed_turn_rolls_back_user_message_stream():
    s = _session(MockLLMClient(raise_on_call=LLMServiceError("boom")))
    with pytest.raises(LLMServiceError):
        list(s.send_stream("hi"))
    assert s.history.messages() == []


def test_retry_after_failure_keeps_roles_valid():
    # first call fails, second succeeds; history must be a single valid turn
    client = MockLLMClient(["recovered"], fail_first=LLMRateLimitError("429"))
    s = _session(client)
    with pytest.raises(LLMRateLimitError):
        s.send("hi")
    assert s.send("hi again") == "recovered"
    msgs = s.history.messages()
    assert msgs == [Message("user", "hi again"), Message("assistant", "recovered")]
    assert [m.role for m in msgs] == ["user", "assistant"]


def test_error_taxonomy_subclasses_llmerror():
    from chatbot.llm import LLMError

    for exc in (LLMAuthError, LLMRateLimitError, LLMServiceError, LLMEmptyResponse):
        assert issubclass(exc, LLMError)
