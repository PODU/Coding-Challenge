"""Split a document into overlapping chunks of roughly 500 tokens.

"Tokens" are just whitespace-separated words. That keeps chunk boundaries
predictable without pulling in a tokenizer model.

Man-page formatting (backspace overstrikes, CRs, form feeds) is stripped first.
Each chunk records the section it came from, taken from the nearest markdown
heading or ALL-CAPS man-page header.
"""

from __future__ import annotations

import re

from .models import Chunk

_MD_HEADING = re.compile(r"^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$")
# Man-page section headers: "NAME", "SYNOPSIS", "SEE ALSO", ...
_MAN_HEADING = re.compile(r"^([A-Z][A-Z0-9 /,'\-]{1,40})$")


def count_tokens(text: str) -> int:
    """Approximate token count (whitespace-separated words)."""
    return len(text.split())


def normalize(text: str) -> str:
    """Clean up text before chunking.

    Backspace overstrikes (``x\\x08x`` bold, ``_\\x08x`` underline) become plain
    ``x``, CRs and form feeds are dropped, and trailing spaces are trimmed.
    """
    if not text:
        return ""
    # Loop because bold text can be overstruck more than once.
    prev = None
    while prev != text:
        prev = text
        text = re.sub(r".\x08", "", text)
    text = text.replace("\r", "").replace("\x0c", "")
    return "\n".join(line.rstrip() for line in text.split("\n"))


def _heading_for(line: str) -> str | None:
    md = _MD_HEADING.match(line)
    if md:
        return md.group(2).strip() or md.group(1)
    stripped = line.strip()
    # Short and without sentence punctuation, so a shouted sentence doesn't
    # count as a header.
    if (
        stripped
        and _MAN_HEADING.match(stripped)
        and not stripped.endswith((".", ":", "!", "?"))
        and len(stripped.split()) <= 5
    ):
        return stripped
    return None


def _tokens_with_sections(text: str) -> list[tuple[str, str]]:
    """Split text into (token, section) pairs.

    Heading lines keep their own tokens too, so no text is lost.
    """
    out: list[tuple[str, str]] = []
    section = ""
    for line in text.split("\n"):
        heading = _heading_for(line)
        if heading is not None:
            section = heading
        for token in line.split():
            out.append((token, section))
    return out


def chunk_text(
    text: str,
    *,
    filename: str,
    chunk_tokens: int = 500,
    overlap: int = 50,
) -> list[Chunk]:
    """Split ``text`` into chunks of at most ``chunk_tokens`` tokens.

    Consecutive chunks share ``overlap`` tokens. Blank input gives no chunks.
    A chunk's section is the one in effect at its first token.
    """
    if chunk_tokens <= 0:
        raise ValueError("chunk_tokens must be positive")
    if overlap < 0 or overlap >= chunk_tokens:
        raise ValueError("overlap must satisfy 0 <= overlap < chunk_tokens")

    tokens = _tokens_with_sections(normalize(text))
    step = chunk_tokens - overlap
    chunks: list[Chunk] = []
    start = 0
    while start < len(tokens):
        window = tokens[start : start + chunk_tokens]
        position = len(chunks)
        chunks.append(
            Chunk(
                filename=filename,
                section=window[0][1],
                position=position,
                text=" ".join(tok for tok, _ in window),
                chunk_id=f"{filename}#{position}",
            )
        )
        if start + chunk_tokens >= len(tokens):
            break
        start += step
    return chunks
