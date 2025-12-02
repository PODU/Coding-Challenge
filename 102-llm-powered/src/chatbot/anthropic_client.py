"""The Anthropic-backed :class:`LLMClient`.

This is the only module that imports the ``anthropic`` SDK. SDK exceptions are
translated into the ``LLMError`` types.
"""

from __future__ import annotations

from typing import Iterator, Sequence

import anthropic

from .config import Config
from .llm import (
    LLMAuthError,
    LLMEmptyResponse,
    LLMRateLimitError,
    LLMResult,
    LLMServiceError,
    Message,
)


class AnthropicLLMClient:
    def __init__(self, cfg: Config, client: "anthropic.Anthropic | None" = None) -> None:
        self._cfg = cfg
        # Tests pass a fake SDK client here.
        self._client = client or anthropic.Anthropic(api_key=cfg.api_key)

    @staticmethod
    def _wire(messages: Sequence[Message]) -> list[dict]:
        return [{"role": m.role, "content": m.content} for m in messages]

    def complete(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> LLMResult:
        try:
            resp = self._client.messages.create(
                model=self._cfg.model,
                max_tokens=max_tokens,
                system=system,
                messages=self._wire(messages),
            )
        except anthropic.AuthenticationError as e:
            raise LLMAuthError(str(e)) from e
        except anthropic.RateLimitError as e:
            raise LLMRateLimitError(str(e)) from e
        except (anthropic.APIConnectionError, anthropic.APIStatusError) as e:
            raise LLMServiceError(str(e)) from e

        text = "".join(b.text for b in resp.content if b.type == "text")
        if not text.strip():
            raise LLMEmptyResponse("model returned no text")
        return LLMResult(text=text, stop_reason=resp.stop_reason)

    def stream(
        self, *, system: str, messages: Sequence[Message], max_tokens: int
    ) -> Iterator[str]:
        try:
            with self._client.messages.stream(
                model=self._cfg.model,
                max_tokens=max_tokens,
                system=system,
                messages=self._wire(messages),
            ) as stream:
                produced = False
                for delta in stream.text_stream:
                    produced = True
                    yield delta
                if not produced:
                    raise LLMEmptyResponse("empty stream")
        except anthropic.AuthenticationError as e:
            raise LLMAuthError(str(e)) from e
        except anthropic.RateLimitError as e:
            raise LLMRateLimitError(str(e)) from e
        except (anthropic.APIConnectionError, anthropic.APIStatusError) as e:
            raise LLMServiceError(str(e)) from e
