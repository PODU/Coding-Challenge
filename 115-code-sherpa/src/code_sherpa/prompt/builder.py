"""Builds the prompt: a system message with the answering rules, and one user
message with memory, the code context (each chunk tagged ``[path:start-end]``)
and the question. Output depends only on the inputs, so it's easy to
golden-test.
"""

from __future__ import annotations

from ..llm.base import Message
from ..memory.base import Episode, SemanticFact
from .select import SelectedContext
from .templates import (
    CONTEXT_HEADER,
    MEMORY_HEADER,
    NO_CONTEXT_MARKER,
    QUESTION_HEADER,
    SYSTEM_RULES,
)


class PromptBuilder:
    def build_messages(
        self,
        question: str,
        context: SelectedContext,
        facts: list[SemanticFact] | None = None,
        episodes: list[Episode] | None = None,
    ) -> list[Message]:
        user = self.build_user_text(question, context, facts, episodes)
        return [
            Message(role="system", content=SYSTEM_RULES),
            Message(role="user", content=user),
        ]

    def build_user_text(
        self,
        question: str,
        context: SelectedContext,
        facts: list[SemanticFact] | None = None,
        episodes: list[Episode] | None = None,
    ) -> str:
        parts: list[str] = []

        memory_block = self._memory_block(facts or [], episodes or [])
        if memory_block:
            parts.append(memory_block)

        if context.insufficient_context or not context.items:
            parts.append(
                f"{CONTEXT_HEADER}\n{NO_CONTEXT_MARKER}\n"
                "(No relevant code was retrieved. If you cannot answer from memory, "
                'say exactly: "I couldn\'t find that in this codebase.")'
            )
        else:
            parts.append(self._context_block(context))

        parts.append(f"{QUESTION_HEADER} {question.strip()}")
        return "\n\n".join(parts)

    def _context_block(self, context: SelectedContext) -> str:
        blocks = [CONTEXT_HEADER]
        for sc in context.items:
            c = sc.chunk
            marker = f"[{c.path}:{c.start_line}-{c.end_line}]"
            desc = f"{marker} {c.kind}"
            if c.symbol:
                desc += f" {c.symbol}"
            fence = c.language if c.language and c.language != "unknown" else ""
            blocks.append(f"{desc}\n```{fence}\n{c.code}\n```")
        return "\n\n".join(blocks)

    def _memory_block(self, facts: list[SemanticFact], episodes: list[Episode]) -> str:
        if not facts and not episodes:
            return ""
        lines = [MEMORY_HEADER]
        for fact in facts:
            lines.append(f"- fact: {fact.key} = {fact.value}")
        for ep in episodes:
            lines.append(f"- earlier Q: {ep.question}")
        return "\n".join(lines)
