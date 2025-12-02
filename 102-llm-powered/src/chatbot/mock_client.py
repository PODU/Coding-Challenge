"""Scripted LLM client for tests. Records every call it receives."""

from __future__ import annotations

from itertools import cycle
from typing import Iterable, Iterator, Sequence

from .llm import LLMEmptyResponse, LLMResult, Message


class MockLLMClient:
    """Offline ``LLMClient``.

    ``replies`` are cycled across calls. ``raise_on_call`` is raised on every
    call, ``fail_first`` only on the first one, and ``empty`` makes every call
    raise :class:`LLMEmptyResponse`.
    """

    def __init__(
        self,
        replies: Iterable[str] | None = None,
        *,
        raise_on_call: BaseException | type[BaseException] | None = None,
        empty: bool = False,
        fail_first: BaseException | type[BaseException] | None = None,
    ) -> None:
        self._replies = cycle(list(replies) if replies else ["Hello from the mock model."])
        self._raise = raise_on_call
        self._empty = empty
        self._fail_first = fail_first
        self.calls: list[dict] = []

    def _maybe_raise(self) -> None:
        if self._fail_first is not None:
            exc, self._fail_first = self._fail_first, None
            raise exc
        if self._raise:
            raise self._raise
        if self._empty:
            raise LLMEmptyResponse("empty")

    def _record(
        self, system: str, messages: Sequence[Message], max_tokens: int
    ) -> None:
        self.calls.append(
            {
                "system": system,
                "messages": list(messages),
                "max_tokens": max_tokens,
            }
        )

    def complete(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> LLMResult:
        self._record(system, messages, max_tokens)
        self._maybe_raise()
        return LLMResult(text=next(self._replies), stop_reason="end_turn")

    def stream(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> Iterator[str]:
        self._record(system, messages, max_tokens)
        self._maybe_raise()
        reply = next(self._replies)
        words = reply.split(" ")
        for i, word in enumerate(words):
            yield word if i == len(words) - 1 else word + " "
