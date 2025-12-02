"""Conversation history.

The Messages API is stateless, so the history is resent on every turn. Only
the last ``max_turns`` exchanges are kept to bound cost.
"""

from __future__ import annotations

from .llm import Message


class ConversationHistory:
    def __init__(self, max_turns: int | None = 20) -> None:
        self._messages: list[Message] = []
        self._max_turns = max_turns  # None = unbounded

    def add_user(self, text: str) -> None:
        self._messages.append(Message("user", text))

    def add_assistant(self, text: str) -> None:
        self._messages.append(Message("assistant", text))

    def messages(self) -> list[Message]:
        """Return the windowed messages, oldest first.

        This is called right after ``add_user``, so the list usually has odd
        length and a plain tail slice can start on an ``assistant`` turn. The
        API rejects that with a 400, so a leading assistant turn is dropped.
        """
        msgs = self._messages
        if self._max_turns is not None:
            msgs = msgs[-2 * self._max_turns :]
            if msgs and msgs[0].role != "user":
                msgs = msgs[1:]
        return list(msgs)

    def pop_last(self) -> Message | None:
        """Remove and return the newest message, or ``None`` if empty.

        Used to undo the user turn when a reply fails, so the next request
        doesn't send two user turns in a row.
        """
        if not self._messages:
            return None
        return self._messages.pop()

    def clear(self) -> None:
        self._messages.clear()

    def __len__(self) -> int:
        return len(self._messages)
