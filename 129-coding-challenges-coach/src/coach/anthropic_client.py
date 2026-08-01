"""LLM client for the Anthropic API (needs the ``live`` extra).

The CLI only imports this when an API key is set, so the rest of the package
has no third-party dependencies.
"""

from __future__ import annotations

import json
import os
from typing import Sequence

from .costs import CAPABLE_MODEL, LIGHT_MODEL
from .llm import LLMError, LLMResult, Message, ToolCall, ToolSpec, Usage

MODEL_MAP = {
    LIGHT_MODEL: "claude-haiku-4-5-20251001",
    CAPABLE_MODEL: "claude-sonnet-5",
}


class AnthropicClient:
    def __init__(self, api_key: str | None = None) -> None:
        try:
            import anthropic
        except ImportError as exc:  # pragma: no cover
            raise LLMError(
                "install the 'live' extra to use the Anthropic client: "
                "pip install .[live]"
            ) from exc
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY")
        )

    @staticmethod
    def _to_api_messages(messages: Sequence[Message]) -> list[dict]:
        api: list[dict] = []
        for m in messages:
            if m.role == "tool":
                api.append(
                    {
                        "role": "user",
                        "content": [
                            {
                                "type": "tool_result",
                                "tool_use_id": m.tool_call_id,
                                "content": m.content,
                            }
                        ],
                    }
                )
            elif m.role == "assistant" and m.tool_call_id is not None:
                call = json.loads(m.content)
                api.append(
                    {
                        "role": "assistant",
                        "content": [
                            {
                                "type": "tool_use",
                                "id": m.tool_call_id,
                                "name": call["tool_call"],
                                "input": call["arguments"],
                            }
                        ],
                    }
                )
            else:
                api.append({"role": m.role, "content": m.content})
        return api

    def complete(
        self,
        *,
        system: str,
        messages: Sequence[Message],
        tools: Sequence[ToolSpec] = (),
        model: str = "",
        max_tokens: int = 1024,
    ) -> LLMResult:
        response = self._client.messages.create(
            model=MODEL_MAP.get(model, MODEL_MAP[CAPABLE_MODEL]),
            system=system,
            messages=self._to_api_messages(messages),
            tools=[
                {
                    "name": t.name,
                    "description": t.description,
                    "input_schema": t.parameters,
                }
                for t in tools
            ],
            max_tokens=max_tokens,
        )
        text_parts: list[str] = []
        tool_calls: list[ToolCall] = []
        for block in response.content:
            if block.type == "text":
                text_parts.append(block.text)
            elif block.type == "tool_use":
                tool_calls.append(
                    ToolCall(id=block.id, name=block.name, arguments=dict(block.input))
                )
        return LLMResult(
            text="".join(text_parts),
            tool_calls=tuple(tool_calls),
            stop_reason=response.stop_reason or "end_turn",
            usage=Usage(
                input_tokens=response.usage.input_tokens,
                output_tokens=response.usage.output_tokens,
            ),
        )
