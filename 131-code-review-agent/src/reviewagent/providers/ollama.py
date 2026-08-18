"""A local model through Ollama, for when nothing should leave the machine."""

from __future__ import annotations

from .. import http
from ..retry import with_retries
from . import Completion

DEFAULT_MODEL = "llama3.1"
DEFAULT_BASE_URL = "http://localhost:11434"


class OllamaProvider:
    def __init__(
        self,
        model: str = "",
        base_url: str = DEFAULT_BASE_URL,
        timeout: float = 300.0,
        transport=None,
    ):
        self.model = model or DEFAULT_MODEL
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self._transport = transport or http.request_json

    def complete(self, system: str, prompt: str) -> Completion:
        def call():
            return self._transport(
                f"{self.base_url}/api/chat",
                method="POST",
                payload={
                    "model": self.model,
                    "stream": False,
                    "messages": [
                        {"role": "system", "content": system},
                        {"role": "user", "content": prompt},
                    ],
                },
                timeout=self.timeout,
            )

        data = with_retries(call)
        return Completion(
            text=data.get("message", {}).get("content", ""),
            input_tokens=data.get("prompt_eval_count", 0),
            output_tokens=data.get("eval_count", 0),
            model=self.model,
        )
