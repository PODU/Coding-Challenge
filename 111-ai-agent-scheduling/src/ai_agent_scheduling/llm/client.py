"""``LLMClient`` protocol with Anthropic and mock implementations."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, List, Optional, Protocol, Sequence, Union, runtime_checkable


@dataclass(frozen=True)
class LLMResult:

    text: str
    model: str
    input_tokens: int = 0
    output_tokens: int = 0


class LLMTimeoutError(Exception):
    """The LLM call exceeded its timeout."""


class LLMRefusalError(Exception):
    """The model declined to answer (``stop_reason == 'refusal'``)."""


@runtime_checkable
class LLMClient(Protocol):

    def complete(self, *, system: Optional[str], task: str, timeout_s: float) -> LLMResult:
        ...


class AnthropicLLMClient:
    """Anthropic SDK client. The SDK is imported lazily so tests don't need it,
    and it reads ``ANTHROPIC_API_KEY`` itself."""

    def __init__(self, client: object = None, model: str = "claude-opus-4-8", max_tokens: int = 4096):
        if client is None:
            import anthropic

            client = anthropic.Anthropic()
        self._client = client
        self._model = model
        self._max_tokens = max_tokens

    def complete(self, *, system: Optional[str], task: str, timeout_s: float) -> LLMResult:
        import anthropic

        try:
            # Stream so a large max_tokens doesn't trip the HTTP timeout. Opus 4.8
            # only supports adaptive thinking, not budget_tokens.
            with self._client.with_options(timeout=timeout_s).messages.stream(
                model=self._model,
                max_tokens=self._max_tokens,
                system=system if system else anthropic.NOT_GIVEN,
                thinking={"type": "adaptive"},
                messages=[{"role": "user", "content": task}],
            ) as stream:
                msg = stream.get_final_message()
        except anthropic.APITimeoutError as exc:  # pragma: no cover - network path
            raise LLMTimeoutError(str(exc)) from exc

        if getattr(msg, "stop_reason", None) == "refusal":  # pragma: no cover
            details = getattr(msg, "stop_details", None)
            category = getattr(details, "category", None)
            raise LLMRefusalError(f"model refused (category={category})")

        text = "".join(b.text for b in msg.content if getattr(b, "type", None) == "text")
        return LLMResult(
            text=text,
            model=getattr(msg, "model", self._model),
            input_tokens=getattr(msg.usage, "input_tokens", 0),
            output_tokens=getattr(msg.usage, "output_tokens", 0),
        )


# A scripted reply may be an LLMResult, a string (wrapped into an LLMResult),
# or an Exception/Exception-class to be raised (to simulate timeouts/failures).
ScriptedReply = Union[LLMResult, str, BaseException, Callable[..., "LLMResult"]]


class MockLLMClient:
    """Scripted ``LLMClient`` for tests.

    Replies are used in order, then ``default`` forever. An exception (instance
    or class) is raised instead of returned.
    """

    def __init__(
        self,
        replies: Optional[Sequence[ScriptedReply]] = None,
        *,
        default: ScriptedReply = "ok",
        model: str = "mock-model",
    ):
        self._replies: List[ScriptedReply] = list(replies or [])
        self._default = default
        self._model = model
        self.calls: List[dict] = []

    @property
    def call_count(self) -> int:
        return len(self.calls)

    def complete(self, *, system: Optional[str], task: str, timeout_s: float) -> LLMResult:
        self.calls.append({"system": system, "task": task, "timeout_s": timeout_s})
        reply = self._replies.pop(0) if self._replies else self._default
        return self._render(reply, system=system, task=task, timeout_s=timeout_s)

    def _render(self, reply: ScriptedReply, **ctx) -> LLMResult:
        if isinstance(reply, type) and issubclass(reply, BaseException):
            raise reply("scripted failure")
        if isinstance(reply, BaseException):
            raise reply
        if callable(reply) and not isinstance(reply, LLMResult):
            return reply(**ctx)
        if isinstance(reply, LLMResult):
            return reply
        return LLMResult(text=str(reply), model=self._model, input_tokens=5, output_tokens=7)
