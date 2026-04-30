"""The Anthropic-backed client. Raises at construction if there's no API key."""

from __future__ import annotations

import os
import time

from ..models import LLMResponse, Prompt
from .base import TransientLLMError


class ClaudeClient:
    name = "claude"

    def __init__(
        self,
        *,
        model: str = "claude-opus-4-8",
        api_key: str | None = None,
        max_tokens: int = 1024,
        client: object | None = None,
    ) -> None:
        import anthropic  # lazy, so the package works without the SDK installed

        self._anthropic = anthropic
        self.model = model
        self.max_tokens = max_tokens
        if client is not None:
            self.client = client
        else:
            key = api_key or os.environ.get("ANTHROPIC_API_KEY")
            if not key:
                raise ValueError(
                    "ANTHROPIC_API_KEY is not set; cannot construct the real ClaudeClient."
                )
            self.client = anthropic.Anthropic(api_key=key)

    def complete(self, prompt: Prompt, *, model: str | None = None) -> LLMResponse:
        use_model = model or self.model
        start = time.perf_counter()
        try:
            resp = self.client.messages.create(
                model=use_model,
                max_tokens=self.max_tokens,
                system=prompt.system,
                thinking={"type": "adaptive"},
                messages=[{"role": m.role, "content": m.content} for m in prompt.messages],
            )
        except self._anthropic.APIError as exc:
            raise TransientLLMError(str(exc)) from exc
        latency_ms = (time.perf_counter() - start) * 1000.0

        text = "".join(
            getattr(block, "text", "")
            for block in resp.content
            if getattr(block, "type", None) == "text"
        )
        return LLMResponse(
            text=text,
            model=getattr(resp, "model", use_model),
            input_tokens=resp.usage.input_tokens,
            output_tokens=resp.usage.output_tokens,
            finish_reason=getattr(resp, "stop_reason", None) or "end_turn",
            latency_ms=latency_ms,
            provider="claude",
        )
