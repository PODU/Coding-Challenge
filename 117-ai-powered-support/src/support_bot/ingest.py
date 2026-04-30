"""Load the knowledge base and split it into chunks.

``chunk_id`` is a content hash, so ingesting the same document twice replaces
its chunks in the store instead of duplicating them.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from pathlib import Path

from .config import Config
from .models import Chunk
from .text import count_tokens


@dataclass
class Document:
    doc_id: str
    title: str
    source: str
    text: str


def _content_hash(doc_id: str, text: str) -> str:
    digest = hashlib.sha256(f"{doc_id}\n{text}".encode("utf-8")).hexdigest()
    return f"{doc_id}:{digest[:16]}"


def _title_from_markdown(text: str, fallback: str) -> str:
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("#"):
            return line.lstrip("#").strip()
    return fallback


def load_kb(kb_path: str | Path) -> list[Document]:
    """Load every non-empty ``*.md`` file in ``kb_path``."""
    docs: list[Document] = []
    for md in sorted(Path(kb_path).glob("*.md")):
        text = md.read_text(encoding="utf-8").strip()
        if not text:
            continue
        doc_id = md.stem
        docs.append(
            Document(
                doc_id=doc_id,
                title=_title_from_markdown(text, doc_id.replace("_", " ").title()),
                source=md.name,
                text=text,
            )
        )
    return docs


def chunk_document(doc: Document, *, max_tokens: int, overlap_tokens: int) -> list[Chunk]:
    """Split on paragraph boundaries into chunks of about ``max_tokens``.

    Each new chunk starts with the last paragraphs of the previous one, up to
    ``overlap_tokens``.
    """
    paragraphs = [p.strip() for p in doc.text.split("\n\n") if p.strip()]
    chunks: list[Chunk] = []
    current: list[str] = []
    current_tokens = 0

    def flush() -> None:
        nonlocal current, current_tokens
        if not current:
            return
        text = "\n\n".join(current)
        chunks.append(
            Chunk(
                chunk_id=_content_hash(doc.doc_id, text),
                doc_id=doc.doc_id,
                title=doc.title,
                source=doc.source,
                text=text,
            )
        )
        tail: list[str] = []
        tail_tokens = 0
        for para in reversed(current):
            n = count_tokens(para)
            if tail_tokens + n > overlap_tokens:
                break
            tail.insert(0, para)
            tail_tokens += n
        current = tail
        current_tokens = tail_tokens

    for para in paragraphs:
        n = count_tokens(para)
        if current and current_tokens + n > max_tokens:
            flush()
        current.append(para)
        current_tokens += n
    flush()
    return chunks


def build_chunks(docs: list[Document], config: Config) -> list[Chunk]:
    return [
        chunk
        for doc in docs
        for chunk in chunk_document(
            doc,
            max_tokens=config.chunk_max_tokens,
            overlap_tokens=config.chunk_overlap_tokens,
        )
    ]
