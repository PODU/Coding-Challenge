"""Turn the LLM's reply into an ``Answer`` with citations.

Only sources the reply actually cites (``[1]``, ``[2, 3]``) are attached. The
"not in the docs" sentinel and model refusals get no citations.
"""

from __future__ import annotations

import re

from .models import Answer, Citation, LLMResponse, RetrievedChunk
from .prompt_builder import NOT_FOUND_SENTINEL

REFUSAL_MESSAGE = "The model declined to answer this question."

_CITE = re.compile(r"\[(\d+(?:\s*,\s*\d+)*)\]")


def _normalize(text: str) -> str:
    return " ".join(text.split()).lower()


def format_answer(response: LLMResponse, retrieved: list[RetrievedChunk]) -> Answer:
    if response.stop_reason == "refusal":
        return Answer(text=REFUSAL_MESSAGE, citations=[], found=False, refused=True)

    text = response.text.strip()

    if not text or NOT_FOUND_SENTINEL.lower() in _normalize(text):
        return Answer(text=text or NOT_FOUND_SENTINEL, citations=[], found=False)

    cited_indices: list[int] = []
    for match in _CITE.finditer(text):
        for num in match.group(1).split(","):
            idx = int(num.strip())
            if idx not in cited_indices:
                cited_indices.append(idx)

    citations: list[Citation] = []
    seen: set[tuple[str, str]] = set()
    for idx in cited_indices:
        if 1 <= idx <= len(retrieved):
            chunk = retrieved[idx - 1].chunk
            key = (chunk.filename, chunk.section)
            if key in seen:
                continue
            seen.add(key)
            citations.append(
                Citation(filename=chunk.filename, section=chunk.section, position=chunk.position)
            )

    return Answer(text=text, citations=citations, found=True)
