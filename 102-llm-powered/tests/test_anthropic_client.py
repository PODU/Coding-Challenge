"""Offline tests for the real AnthropicLLMClient.

These never hit the network and need no API key: a *fake* SDK object is
injected into ``AnthropicLLMClient`` to verify request shape and the mapping
from SDK exceptions onto our typed ``LLMError`` hierarchy.
"""

from __future__ import annotations

from types import SimpleNamespace

import pytest

anthropic = pytest.importorskip("anthropic")
httpx = pytest.importorskip("httpx")

from chatbot.anthropic_client import AnthropicLLMClient  # noqa: E402
from chatbot.config import Config  # noqa: E402
from chatbot.llm import (  # noqa: E402
    LLMAuthError,
    LLMEmptyResponse,
    LLMRateLimitError,
    LLMServiceError,
    Message,
)


def _cfg():
    return Config(api_key="test-key", model="claude-opus-4-8", max_tokens=128)



def _text_block(text):
    return SimpleNamespace(type="text", text=text)


class _FakeMessages:
    def __init__(self, *, response=None, raises=None, stream_cm=None):
        self._response = response
        self._raises = raises
        self._stream_cm = stream_cm
        self.create_kwargs = None
        self.stream_kwargs = None

    def create(self, **kwargs):
        self.create_kwargs = kwargs
        if self._raises:
            raise self._raises
        return self._response

    def stream(self, **kwargs):
        self.stream_kwargs = kwargs
        if self._raises:
            raise self._raises
        return self._stream_cm


class _FakeSDK:
    def __init__(self, messages):
        self.messages = messages


class _FakeStreamCM:
    def __init__(self, deltas):
        self._deltas = deltas

    def __enter__(self):
        return SimpleNamespace(text_stream=iter(self._deltas))

    def __exit__(self, *exc):
        return False


def _request():
    return httpx.Request("POST", "https://api.anthropic.com/v1/messages")


def _response(status):
    return httpx.Response(status, request=_request())



def test_complete_sends_expected_request_shape():
    resp = SimpleNamespace(
        content=[_text_block("hi "), _text_block("there")], stop_reason="end_turn"
    )
    msgs = _FakeMessages(response=resp)
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(msgs))

    result = client.complete(
        system="SYS",
        messages=[Message("user", "hello")],
        max_tokens=128,
    )

    assert result.text == "hi there"
    assert result.stop_reason == "end_turn"
    kw = msgs.create_kwargs
    assert kw["model"] == "claude-opus-4-8"
    assert kw["system"] == "SYS"
    assert kw["max_tokens"] == 128
    assert kw["messages"] == [{"role": "user", "content": "hello"}]


def test_complete_empty_text_raises_empty_response():
    resp = SimpleNamespace(content=[_text_block("   ")], stop_reason="end_turn")
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(response=resp)))
    with pytest.raises(LLMEmptyResponse):
        client.complete(system="s", messages=[Message("user", "x")], max_tokens=8)


def test_complete_ignores_non_text_blocks():
    resp = SimpleNamespace(
        content=[
            SimpleNamespace(type="thinking", text="ignored"),
            _text_block("kept"),
        ],
        stop_reason="end_turn",
    )
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(response=resp)))
    result = client.complete(system="s", messages=[Message("user", "x")], max_tokens=8)
    assert result.text == "kept"


@pytest.mark.parametrize(
    "sdk_exc, mapped",
    [
        (anthropic.AuthenticationError("bad", response=_response(401), body=None), LLMAuthError),
        (anthropic.RateLimitError("slow", response=_response(429), body=None), LLMRateLimitError),
        (anthropic.APIStatusError("oops", response=_response(500), body=None), LLMServiceError),
    ],
)
def test_complete_maps_sdk_exceptions(sdk_exc, mapped):
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(raises=sdk_exc)))
    with pytest.raises(mapped):
        client.complete(system="s", messages=[Message("user", "x")], max_tokens=8)


def test_complete_maps_connection_error():
    exc = anthropic.APIConnectionError(request=_request())
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(raises=exc)))
    with pytest.raises(LLMServiceError):
        client.complete(system="s", messages=[Message("user", "x")], max_tokens=8)



def test_stream_yields_deltas_and_sends_expected_shape():
    cm = _FakeStreamCM(["a", "b", "c"])
    msgs = _FakeMessages(stream_cm=cm)
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(msgs))

    out = list(
        client.stream(system="SYS", messages=[Message("user", "hi")], max_tokens=64)
    )
    assert out == ["a", "b", "c"]
    kw = msgs.stream_kwargs
    assert kw["model"] == "claude-opus-4-8"
    assert kw["system"] == "SYS"
    assert kw["messages"] == [{"role": "user", "content": "hi"}]


def test_stream_empty_raises_empty_response():
    cm = _FakeStreamCM([])
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(stream_cm=cm)))
    with pytest.raises(LLMEmptyResponse):
        list(client.stream(system="s", messages=[Message("user", "x")], max_tokens=8))


def test_stream_maps_rate_limit():
    exc = anthropic.RateLimitError("slow", response=_response(429), body=None)
    client = AnthropicLLMClient(_cfg(), client=_FakeSDK(_FakeMessages(raises=exc)))
    with pytest.raises(LLMRateLimitError):
        list(client.stream(system="s", messages=[Message("user", "x")], max_tokens=8))
