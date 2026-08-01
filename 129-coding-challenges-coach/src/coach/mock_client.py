"""Scripted LLM client for tests and the offline demo.

Each script entry answers one call: a string is final text, a list of
:class:`ToolCall` asks for tools. Every call is recorded in ``calls``.
"""

from __future__ import annotations

from typing import Sequence

from .llm import LLMEmptyResponse, LLMResult, Message, ToolCall, ToolSpec, Usage

TurnScript = str | list[ToolCall]


class MockLLMClient:
    def __init__(
        self,
        script: Sequence[TurnScript] | None = None,
        *,
        raise_on_call: BaseException | type[BaseException] | None = None,
        empty: bool = False,
        tokens_per_call: tuple[int, int] = (100, 50),
    ) -> None:
        self._script = list(script or ["Hello from the mock coach."])
        self._raise = raise_on_call
        self._empty = empty
        self._tokens = tokens_per_call
        self.calls: list[dict] = []

    def complete(
        self,
        *,
        system: str,
        messages: Sequence[Message],
        tools: Sequence[ToolSpec] = (),
        model: str = "",
        max_tokens: int = 1024,
    ) -> LLMResult:
        self.calls.append(
            {
                "system": system,
                "messages": list(messages),
                "tools": list(tools),
                "model": model,
                "max_tokens": max_tokens,
            }
        )
        if self._raise:
            raise self._raise
        if self._empty:
            raise LLMEmptyResponse("empty")
        step = self._script.pop(0) if self._script else "…and that is my advice."
        usage = Usage(*self._tokens)
        if isinstance(step, str):
            return LLMResult(text=step, usage=usage)
        return LLMResult(tool_calls=tuple(step), stop_reason="tool_use", usage=usage)
