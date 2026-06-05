"""Extracting clauses chunk by chunk."""

from __future__ import annotations

import re
from difflib import SequenceMatcher
from typing import Optional

from .chunking import chunk_document
from .config import Config
from .llm import LLMClient
from .models import Clause, ContractDocument
from .obs import RunRecorder
from .parsing import parse_clauses
from .prompts import EXTRACT_SCHEMA, build_extract_prompt

_WS = re.compile(r"\s+")
_DEDUP_THRESHOLD = 0.92


def _normalise(text: str) -> str:
    return _WS.sub(" ", text).strip().lower()


def _is_duplicate(candidate: str, seen: list[str]) -> bool:
    if not candidate:
        return True
    for existing in seen:
        if candidate == existing:
            return True
        if candidate in existing or existing in candidate:
            return True
        if SequenceMatcher(None, candidate, existing).ratio() >= _DEDUP_THRESHOLD:
            return True
    return False


def extract_clauses(
    doc: ContractDocument,
    llm: LLMClient,
    config: Optional[Config] = None,
    recorder: Optional[RunRecorder] = None,
) -> list[Clause]:
    """Extract the document's clauses, numbered from 1.

    Chunks overlap, so the same clause can come back twice; near-identical
    clause texts are treated as duplicates and only the first is kept.
    """

    config = config or Config.from_env()
    chunks = chunk_document(doc, config.max_chunk_tokens, config.chunk_overlap)

    merged: list[Clause] = []
    seen: list[str] = []
    for chunk in chunks:
        system, messages = build_extract_prompt(chunk)
        raw = llm.complete(
            system, messages, output_schema=EXTRACT_SCHEMA, intent="extract"
        )
        for clause in parse_clauses(raw):
            norm = _normalise(clause.text)
            if _is_duplicate(norm, seen):
                continue
            seen.append(norm)
            merged.append(clause.model_copy(update={"source_chunk": chunk.index}))

    # Each chunk numbers its clauses from 1, so renumber across the document.
    renumbered = [c.model_copy(update={"number": i}) for i, c in enumerate(merged, start=1)]

    if recorder is not None:
        recorder.emit("extract_clauses", chunks=len(chunks), clauses=len(renumbered))
    return renumbered
