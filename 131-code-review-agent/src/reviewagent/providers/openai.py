"""OpenAI-compatible chat completions provider.

Set REVIEW_BASE_URL to use Gemini's OpenAI-compatible endpoint or any other
service with /v1/chat/completions.
"""

from __future__ import annotations

from .. import http
from ..retry import with_retries
from . import Completion

DEFAULT_MODEL = "gpt-4o-mini"
DEFAULT_BASE_URL = "https://api.openai.com/v1"


class OpenAIProvider:
    def __init__(
        self,
        api_key: str,
        model: str = "",
        base_url: str = DEFAULT_BASE_URL,
        timeout: float = 120.0,
        transport=None,
    ):
        self.api_key = api_key
        self.model = model or DEFAULT_MODEL
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self._transport = transport or http.request_json

    def complete(self, system: str, prompt: str) -> Completion:
        def call():
            return self._transport(
                f"{self.base_url}/chat/completions",
                method="POST",
                headers={"Authorization": f"Bearer {self.api_key}"},
                payload={
                    "model": self.model,
                    "messages": [
                        {"role": "system", "content": system},
                        {"role": "user", "content": prompt},
                    ],
                },
                timeout=self.timeout,
            )

        data = with_retries(call)
        text = data["choices"][0]["message"]["content"] or ""
        usage = data.get("usage", {})
        return Completion(
            text=text,
            input_tokens=usage.get("prompt_tokens", 0),
            output_tokens=usage.get("completion_tokens", 0),
            model=data.get("model", self.model),
        )
