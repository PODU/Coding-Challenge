"""A chat session: persona + history + client.

A reply is only added to history once it has fully arrived. If the call fails,
the user turn is removed again so the user can simply retry.
"""

from __future__ import annotations

from typing import Iterator

from .history import ConversationHistory
from .llm import LLMClient
from .persona import Persona


class ChatSession:
    def __init__(
        self,
        client: LLMClient,
        persona: Persona,
        history: ConversationHistory,
        *,
        max_tokens: int = 1024,
    ) -> None:
        self._client = client
        self._persona = persona
        self._history = history
        self._max_tokens = max_tokens

    @property
    def persona(self) -> Persona:
        return self._persona

    @property
    def history(self) -> ConversationHistory:
        return self._history

    def send_stream(self, user_text: str) -> Iterator[str]:
        """Yield the reply as it streams in."""
        self._history.add_user(user_text)
        chunks: list[str] = []
        try:
            for delta in self._client.stream(
                system=self._persona.system_prompt,
                messages=self._history.messages(),
                max_tokens=self._max_tokens,
            ):
                chunks.append(delta)
                yield delta
        except BaseException:
            self._history.pop_last()
            raise
        self._history.add_assistant("".join(chunks))

    def send(self, user_text: str) -> str:
        """Non-streaming version of :meth:`send_stream`."""
        self._history.add_user(user_text)
        try:
            result = self._client.complete(
                system=self._persona.system_prompt,
                messages=self._history.messages(),
                max_tokens=self._max_tokens,
            )
        except BaseException:
            self._history.pop_last()
            raise
        self._history.add_assistant(result.text)
        return result.text
