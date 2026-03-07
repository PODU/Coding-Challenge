"""Build the prompt from the question, retrieved context and memory.

There's deliberately no assistant prefill: ``claude-opus-4-8`` rejects it with
a 400, so all formatting instructions live in the system prompt.
"""

from __future__ import annotations

from dataclasses import dataclass

from .models import RetrievedChunk, Turn

# The model is told to reply with exactly this when the context has no answer;
# answer_formatter matches on it.
NOT_FOUND_SENTINEL = "I couldn't find this in the indexed documentation."

SYSTEM_PROMPT = (
    "You are RTFM, a precise documentation assistant. Answer the user's question "
    "using ONLY the numbered context sources provided in the user message. "
    "Do not use outside knowledge and do not invent facts.\n"
    "Cite every claim by appending the source number(s) in square brackets, e.g. [1] or [2]. "
    "Cite only sources you actually used.\n"
    f'If the context does not contain the answer, reply with exactly: "{NOT_FOUND_SENTINEL}" '
    "and nothing else.\n"
    "If the question is ambiguous, answer the most likely reading and briefly note the ambiguity. "
    "Be concise and respond directly without preamble."
)


@dataclass(frozen=True)
class BuiltPrompt:
    system: str
    user: str


def build_prompt(
    question: str,
    retrieved: list[RetrievedChunk],
    *,
    recent_turns: list[Turn] | None = None,
    long_term_memories: list[str] | None = None,
) -> BuiltPrompt:
    """Build the prompt. Sources are numbered ``[1]``, ``[2]``, ... in rank
    order so the model can cite them, and the question goes last.
    """
    parts: list[str] = []

    memories = [m.strip() for m in (long_term_memories or []) if m and m.strip()]
    if memories:
        parts.append("What you know about the user (long-term memory):")
        parts.extend(f"- {mem}" for mem in memories)
        parts.append("")

    if recent_turns:
        parts.append("Recent conversation (most recent last):")
        for turn in recent_turns:
            parts.append(f"User: {turn.question}")
            parts.append(f"Assistant: {turn.answer}")
        parts.append("")

    parts.append("Context sources:")
    if retrieved:
        for i, rc in enumerate(retrieved, start=1):
            label = rc.chunk.filename
            if rc.chunk.section:
                label += f" § {rc.chunk.section}"
            parts.append(f"[{i}] ({label})")
            parts.append(rc.chunk.text)
            parts.append("")
    else:
        parts.append("(no sources retrieved)")
        parts.append("")

    parts.append(f"Question: {question}")

    return BuiltPrompt(system=SYSTEM_PROMPT, user="\n".join(parts))
