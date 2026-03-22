"""LLM client interface and the Anthropic implementation.

Responses use Anthropic's content blocks as plain dicts, so the real client is
a thin wrapper and tests can script a fake one.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Protocol, runtime_checkable


@dataclass
class LLMResponse:
    """One model turn: the stop reason and the raw content blocks."""

    stop_reason: str
    content: list[dict[str, Any]]


@runtime_checkable
class LLMClient(Protocol):
    """What the agent loop needs from a model."""

    def complete(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        system: str,
    ) -> LLMResponse:
        ...


class AnthropicLLMClient:
    """Client for the Anthropic API. The key comes from ``ANTHROPIC_API_KEY``."""

    def __init__(
        self,
        model: str = "claude-opus-4-8",
        max_tokens: int = 16000,
    ) -> None:
        # Lazy import: the offline tests never need the SDK.
        import anthropic

        self._client = anthropic.Anthropic()
        self._model = model
        self._max_tokens = max_tokens

    def complete(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]],
        system: str,
    ) -> LLMResponse:
        resp = self._client.messages.create(
            model=self._model,
            max_tokens=self._max_tokens,
            # Opus 4.8 uses adaptive thinking. The legacy budget_tokens form is
            # rejected with a 400; temperature/top_p/top_k are not sent either.
            thinking={"type": "adaptive"},
            system=system,
            tools=tools,
            messages=messages,
        )
        # The loop sends these blocks back unchanged on the next turn, which the
        # API requires for thinking and tool_use blocks.
        return LLMResponse(
            stop_reason=resp.stop_reason,
            content=[block.model_dump() for block in resp.content],
        )
