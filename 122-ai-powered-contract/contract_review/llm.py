"""The LLM interface, the Anthropic implementation, and a scripted mock.

Every stage talks to the model through ``LLMClient``. ``make_client`` picks
the implementation from the config.
"""

from __future__ import annotations

import os
from collections import defaultdict
from typing import Any, Callable, Iterator, Optional, Protocol, runtime_checkable

from .config import Config

# An intent names the pipeline step making a call ("extract", "analyse", ...).
# MockLLM uses it to choose a scripted reply.
Intent = str


@runtime_checkable
class LLMClient(Protocol):
    def complete(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        output_schema: Optional[dict[str, Any]] = None,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> str:
        ...

    def stream(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> Iterator[str]:
        ...


class ClaudeClient:
    """Calls Claude through the ``anthropic`` SDK. The SDK client is created on
    first use, so the API key is only needed once a request is made."""

    def __init__(self, model: str = "claude-opus-4-8") -> None:
        self.model = model
        self._client: Any = None

    def _ensure_client(self) -> Any:
        if self._client is None:
            if not os.environ.get("ANTHROPIC_API_KEY"):
                raise RuntimeError(
                    "ANTHROPIC_API_KEY is not set. Set it in the environment or use "
                    "the 'mock' provider for offline runs."
                )
            import anthropic

            self._client = anthropic.Anthropic()
        return self._client

    def complete(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        output_schema: Optional[dict[str, Any]] = None,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> str:
        client = self._ensure_client()
        kwargs: dict[str, Any] = {
            "model": self.model,
            "max_tokens": max_tokens,
            "system": system,
            "messages": messages,
            "thinking": {"type": "adaptive"},
        }
        if output_schema is not None:
            kwargs["output_config"] = {
                "format": {"type": "json_schema", "schema": output_schema}
            }
        response = client.messages.create(**kwargs)
        return _text_from_blocks(response.content)

    def stream(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> Iterator[str]:
        client = self._ensure_client()
        with client.messages.stream(
            model=self.model,
            max_tokens=max_tokens,
            system=system,
            messages=messages,
        ) as stream:
            for text in stream.text_stream:
                yield text


def _text_from_blocks(content: Any) -> str:
    parts = []
    for block in content:
        if getattr(block, "type", None) == "text":
            parts.append(block.text)
    return "".join(parts)


ResponseValue = Any  # str | list[str] | Callable[[str, list, Intent], str]


class MockLLM:
    """Offline stand-in for tests.

    ``responses`` maps an intent to a reply: a string, a list of strings
    returned in turn (the last one repeats), or a callable taking
    ``(system, messages, intent)``. A ``router`` callable, if given, answers
    every call instead. Calls are recorded in ``self.calls``.
    """

    def __init__(
        self,
        responses: Optional[dict[Intent, ResponseValue]] = None,
        *,
        router: Optional[Callable[[str, list[dict[str, Any]], Optional[Intent]], str]] = None,
        default: Optional[str] = None,
    ) -> None:
        self.responses: dict[Intent, ResponseValue] = dict(responses or {})
        self.router = router
        self.default = default
        self.calls: list[dict[str, Any]] = []
        self._counters: dict[Intent, int] = defaultdict(int)

    def _resolve(
        self, system: str, messages: list[dict[str, Any]], intent: Optional[Intent]
    ) -> str:
        if self.router is not None:
            return self.router(system, messages, intent)
        if intent in self.responses:
            value = self.responses[intent]
        elif self.default is not None:
            value = self.default
        else:
            raise KeyError(f"MockLLM has no scripted response for intent {intent!r}")
        if callable(value):
            return value(system, messages, intent)
        if isinstance(value, (list, tuple)):
            i = self._counters[intent or ""]
            self._counters[intent or ""] += 1
            if not value:
                raise KeyError(f"MockLLM response list for {intent!r} is empty")
            return value[i] if i < len(value) else value[-1]
        return value

    def complete(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        output_schema: Optional[dict[str, Any]] = None,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> str:
        self.calls.append(
            {
                "method": "complete",
                "intent": intent,
                "system": system,
                "messages": messages,
                "output_schema": output_schema,
            }
        )
        return self._resolve(system, messages, intent)

    def stream(
        self,
        system: str,
        messages: list[dict[str, Any]],
        *,
        max_tokens: int = 4096,
        intent: Optional[Intent] = None,
    ) -> Iterator[str]:
        self.calls.append(
            {
                "method": "stream",
                "intent": intent,
                "system": system,
                "messages": messages,
            }
        )
        # Split into words so callers see more than one chunk.
        yield from _tokenise(self._resolve(system, messages, intent))

    def calls_for(self, intent: Intent) -> list[dict[str, Any]]:
        return [c for c in self.calls if c.get("intent") == intent]


def _tokenise(text: str) -> Iterator[str]:
    if not text:
        return
    chunk: list[str] = []
    for ch in text:
        chunk.append(ch)
        if ch.isspace():
            yield "".join(chunk)
            chunk = []
    if chunk:
        yield "".join(chunk)


def make_client(config: Optional[Config] = None) -> LLMClient:
    """Return the client for ``config.provider`` ("claude" or "mock")."""

    config = config or Config.from_env()
    provider = (config.provider or "claude").lower()
    if provider == "mock":
        return MockLLM(default="{}")
    if provider == "claude":
        return ClaudeClient(model=config.model)
    raise ValueError(f"Unknown LLM provider: {config.provider!r}")
