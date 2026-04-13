"""Offline LLM client for tests and dry runs.

Replies come from ``handler`` if given, else the ``responses`` queue, else a
canned answer that cites the first location in the prompt (or the "couldn't
find it" sentence when there was no context). ``error=True`` makes every call
fail. Calls are recorded in ``calls``.
"""

from __future__ import annotations

import re
from collections.abc import Callable

from .base import LLMClient, LLMError, LLMResponse, Message

NOT_FOUND_SENTENCE = "I couldn't find that in this codebase."

_CITE_RE = re.compile(r"\[(?P<path>[^\]\s]+):(?P<start>\d+)-(?P<end>\d+)\]")


class MockLLMClient(LLMClient):
    def __init__(
        self,
        responses: list[str] | None = None,
        handler: Callable[[list[Message]], str] | None = None,
        error: bool = False,
        model: str = "mock-llm",
    ) -> None:
        self._responses = list(responses or [])
        self._handler = handler
        self._error = error
        self._model = model
        self.calls: list[list[Message]] = []

    @property
    def call_count(self) -> int:
        return len(self.calls)

    def complete(
        self,
        messages: list[Message],
        *,
        model: str | None = None,
        max_tokens: int = 1024,
    ) -> LLMResponse:
        self.calls.append(list(messages))
        if self._error:
            raise LLMError("mock LLM forced failure")

        if self._handler is not None:
            text = self._handler(messages)
        elif self._responses:
            text = self._responses.pop(0)
        else:
            text = self._default_reply(messages)

        return LLMResponse(
            text=text,
            model=model or self._model,
            stop_reason="end_turn",
            input_tokens=sum(len(m.content) for m in messages) // 4,
            output_tokens=len(text) // 4,
        )

    def _default_reply(self, messages: list[Message]) -> str:
        prompt = "\n".join(m.content for m in messages)
        # Match the whole marker line PromptBuilder writes. A bare "NO_CONTEXT"
        # check would fire on indexed code that happens to contain the word.
        if "context:\nNO_CONTEXT" in prompt:
            return NOT_FOUND_SENTENCE
        question = next(
            (m.content for m in reversed(messages) if m.role == "user"), ""
        )
        # PromptBuilder puts the real question last, after any quoted ones.
        q_matches = re.findall(r"Question:\s*(.+)", question)
        q = q_matches[-1].strip() if q_matches else question.strip()
        cite = _CITE_RE.search(prompt)
        if cite:
            loc = f"{cite.group('path')}:{cite.group('start')}-{cite.group('end')}"
            return (
                f"Based on the retrieved code, here is what handles this. "
                f"See {loc}. (re: {q})"
            )
        return f"Based on the provided context: {q}"
