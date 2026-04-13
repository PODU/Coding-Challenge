"""Anthropic client. Retries transient failures with backoff and raises
:class:`LLMError` when it gives up. The SDK is imported lazily since it's an
optional dependency.
"""

from __future__ import annotations

import time

from .base import LLMClient, LLMError, LLMResponse, Message

_DEFAULT_MODEL = "claude-opus-4-8"


class AnthropicClient(LLMClient):
    def __init__(
        self,
        api_key: str | None,
        model: str = _DEFAULT_MODEL,
        timeout: float = 60.0,
        max_retries: int = 2,
    ) -> None:
        if not api_key:
            raise LLMError("ANTHROPIC_API_KEY is required for the Anthropic client.")
        try:
            import anthropic
        except ImportError as exc:  # pragma: no cover - optional dep
            raise LLMError(
                "The `anthropic` package is required. Install it or set SHERPA_LLM=mock."
            ) from exc
        self._anthropic = anthropic
        self._client = anthropic.Anthropic(api_key=api_key, timeout=timeout)
        self._model = model
        self._max_retries = max_retries

    def complete(
        self,
        messages: list[Message],
        *,
        model: str | None = None,
        max_tokens: int = 1024,
    ) -> LLMResponse:
        system_parts = [m.content for m in messages if m.role == "system"]
        convo = [
            {"role": m.role, "content": m.content}
            for m in messages
            if m.role in ("user", "assistant")
        ]
        if not convo:
            raise LLMError("At least one user message is required.")

        # Don't send temperature or other sampling params: claude-opus-4-8
        # rejects them with a 400.
        kwargs = {
            "model": model or self._model,
            "max_tokens": max_tokens,
            "messages": convo,
        }
        if system_parts:
            kwargs["system"] = "\n\n".join(system_parts)

        last_exc: Exception | None = None
        for attempt in range(self._max_retries + 1):
            try:
                resp = self._client.messages.create(**kwargs)
                text = "".join(
                    block.text
                    for block in resp.content
                    if getattr(block, "type", None) == "text"
                )
                usage = getattr(resp, "usage", None)
                return LLMResponse(
                    text=text,
                    model=getattr(resp, "model", kwargs["model"]),
                    stop_reason=getattr(resp, "stop_reason", None),
                    input_tokens=getattr(usage, "input_tokens", None),
                    output_tokens=getattr(usage, "output_tokens", None),
                )
            except Exception as exc:  # noqa: BLE001
                last_exc = exc
                if not self._is_retryable(exc) or attempt == self._max_retries:
                    break
                time.sleep(min(2 ** attempt, 8))
        raise LLMError(f"Anthropic request failed: {last_exc}") from last_exc

    def _is_retryable(self, exc: Exception) -> bool:
        a = self._anthropic
        retryable = tuple(
            t
            for t in (
                getattr(a, "APITimeoutError", None),
                getattr(a, "APIConnectionError", None),
                getattr(a, "RateLimitError", None),
                getattr(a, "InternalServerError", None),
            )
            if t is not None
        )
        return isinstance(exc, retryable) if retryable else False
