"""Anthropic Messages API, called directly rather than through the SDK."""

from __future__ import annotations

from .. import http
from ..retry import with_retries
from . import Completion

DEFAULT_MODEL = "claude-sonnet-5"


class AnthropicProvider:
    def __init__(
        self,
        api_key: str,
        model: str = "",
        timeout: float = 120.0,
        transport=None,
    ):
        self.api_key = api_key
        self.model = model or DEFAULT_MODEL
        self.timeout = timeout
        self._transport = transport or http.request_json

    def complete(self, system: str, prompt: str) -> Completion:
        def call():
            return self._transport(
                "https://api.anthropic.com/v1/messages",
                method="POST",
                headers={
                    "x-api-key": self.api_key,
                    "anthropic-version": "2023-06-01",
                },
                payload={
                    "model": self.model,
                    "max_tokens": 4096,
                    "system": system,
                    "messages": [{"role": "user", "content": prompt}],
                },
                timeout=self.timeout,
            )

        data = with_retries(call)
        text = "".join(
            block.get("text", "")
            for block in data.get("content", [])
            if block.get("type") == "text"
        )
        usage = data.get("usage", {})
        return Completion(
            text=text,
            input_tokens=usage.get("input_tokens", 0),
            output_tokens=usage.get("output_tokens", 0),
            model=data.get("model", self.model),
        )
