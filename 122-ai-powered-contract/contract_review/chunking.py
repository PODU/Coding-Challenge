"""Splitting a contract into overlapping chunks that fit a token budget.

The text is cut at headings and blank lines, and the pieces are packed into
chunks. Consecutive chunks share a few trailing pieces so a clause that
straddles a cut still appears whole in one of them. Each chunk keeps its
character offsets: ``doc.text[start_char:end_char] == chunk.text``.
"""

from __future__ import annotations

import re

from .ingest import estimate_tokens
from .models import Chunk, ContractDocument

# Headings such as "Section 3", "ARTICLE II", "Clause 4", "3." or "3.2)".
_HEADING_KW = re.compile(r"^[ \t]*(?:section|article|clause)\b", re.IGNORECASE)
_HEADING_NUM = re.compile(r"^[ \t]*\d+(?:\.\d+)*[.)]\s")
_PARA_BREAK = re.compile(r"\n[ \t]*\n+")


def _split_points(text: str) -> list[int]:
    """Return the sorted offsets where the text may be cut."""

    points: set[int] = {0, len(text)}
    for match in _PARA_BREAK.finditer(text):
        points.add(match.end())

    offset = 0
    for line in text.splitlines(keepends=True):
        if _HEADING_KW.match(line) or _HEADING_NUM.match(line):
            points.add(offset)
        offset += len(line)

    return sorted(p for p in points if 0 <= p <= len(text))


def _segments(text: str, max_tokens: int) -> list[tuple[int, int]]:
    """Return (start, end) spans covering the text, none over ``max_tokens``."""

    points = _split_points(text)
    spans: list[tuple[int, int]] = []
    budget_chars = max(1, max_tokens * 4)

    for start, end in zip(points, points[1:]):
        if end <= start:
            continue
        if estimate_tokens(text[start:end]) <= max_tokens:
            spans.append((start, end))
            continue
        # Too big on its own (a long clause with no breaks): cut it blindly.
        cursor = start
        while cursor < end:
            stop = min(cursor + budget_chars, end)
            spans.append((cursor, stop))
            cursor = stop
    return spans


def chunk_document(
    doc: ContractDocument,
    max_tokens: int = 6000,
    overlap: int = 400,
) -> list[Chunk]:
    text = doc.text
    if not text:
        return []
    if max_tokens <= 0:
        raise ValueError("max_tokens must be positive")
    overlap = max(0, min(overlap, max_tokens - 1))

    spans = _segments(text, max_tokens)
    if not spans:
        return []

    seg_tokens = [estimate_tokens(text[s:e]) for s, e in spans]
    n = len(spans)

    chunks: list[Chunk] = []
    i = 0
    index = 0
    while i < n:
        # Take spans[i:j], as many as fit.
        j = i
        used = 0
        while j < n:
            if j > i and used + seg_tokens[j] > max_tokens:
                break
            used += seg_tokens[j]
            j += 1

        start_char = spans[i][0]
        end_char = spans[j - 1][1]
        chunk_text = text[start_char:end_char]
        chunks.append(
            Chunk(
                index=index,
                text=chunk_text,
                start_char=start_char,
                end_char=end_char,
                est_tokens=estimate_tokens(chunk_text),
            )
        )
        index += 1

        if j >= n:
            break

        # Start the next chunk early enough to repeat up to `overlap` tokens,
        # but always move forward by at least one span.
        k = j - 1
        carried = 0
        while k > i:
            if carried + seg_tokens[k] > overlap:
                break
            carried += seg_tokens[k]
            k -= 1
        i = max(k + 1, i + 1)

    return chunks
