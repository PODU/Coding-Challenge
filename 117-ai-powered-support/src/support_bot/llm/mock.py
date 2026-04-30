"""Offline LLM client for tests.

It records every prompt and requested model, and can be told to abstain or to
raise.
"""

from __future__ import annotations

from typing import Callable

from ..models import LLMResponse, Prompt
from ..text import count_tokens
from .base import LLMClient


class MockLLMClient:
    name = "mock"

    def __init__(
        self,
        *,
        responder: Callable[[Prompt, str | None], str] | None = None,
        abstain: bool = False,
        error: Exception | None = None,
        default_model: str = "mock-model",
        sentinel: str = "NO_ANSWER",
    ) -> None:
        self.responder = responder
        self.abstain = abstain
        self.error = error
        self.default_model = default_model
        self.sentinel = sentinel
        self.calls: list[Prompt] = []
        self.requested_models: list[str | None] = []

    def complete(self, prompt: Prompt, *, model: str | None = None) -> LLMResponse:
        self.calls.append(prompt)
        self.requested_models.append(model)
        if self.error is not None:
            raise self.error

        if self.responder is not None:
            text = self.responder(prompt, model)
        elif self.abstain:
            text = self.sentinel
        else:
            text = self._default_text(prompt)

        input_tokens = count_tokens(prompt.system) + sum(
            count_tokens(m.content) for m in prompt.messages
        )
        return LLMResponse(
            text=text,
            model=model or self.default_model,
            input_tokens=input_tokens,
            output_tokens=count_tokens(text),
            finish_reason="end_turn",
            latency_ms=0.0,
            provider="mock",
        )

    def _default_text(self, prompt: Prompt) -> str:
        # Quote the first context line if there is one, otherwise abstain.
        if "[Source:" in prompt.system:
            snippet = self._first_context_line(prompt.system)
            return f"Based on the knowledge base: {snippet}"
        return self.sentinel

    @staticmethod
    def _first_context_line(system: str) -> str:
        capture = False
        for line in system.splitlines():
            if line.startswith("[Source:"):
                capture = True
                continue
            if capture and line.strip():
                return line.strip()
        return "see the cited sources."


def scripted(text: str, *, default_model: str = "mock-model") -> MockLLMClient:
    return MockLLMClient(responder=lambda _p, _m: text, default_model=default_model)


__all__ = ["MockLLMClient", "scripted", "LLMClient"]
