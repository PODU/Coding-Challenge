"""Load text files, chunk them, embed the chunks and store them.

A file that fails is recorded in the report and the rest carry on.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

from .chunker import chunk_text
from .config import Config
from .embedder import Embedder
from .models import Chunk
from .vector_store import VectorStore

_TEXT_SUFFIXES = {".md", ".txt", ".rst", ".man", ".1", ".2", ".3", ""}


@dataclass
class IngestReport:
    files_ok: list[str] = field(default_factory=list)
    files_failed: list[tuple[str, str]] = field(default_factory=list)  # (path, error)
    chunks_added: int = 0

    @property
    def ok(self) -> bool:
        return not self.files_failed


def _iter_files(path: str) -> list[str]:
    p = Path(path)
    if p.is_file():
        return [str(p)]
    if p.is_dir():
        out: list[str] = []
        for root, _dirs, files in os.walk(p):
            for name in files:
                if Path(name).suffix.lower() in _TEXT_SUFFIXES:
                    out.append(str(Path(root) / name))
        return sorted(out)
    return []


def ingest_paths(
    paths: list[str],
    *,
    embedder: Embedder,
    store: VectorStore,
    cfg: Config,
) -> IngestReport:
    report = IngestReport()
    targets: list[str] = []
    for path in paths:
        found = _iter_files(path)
        if not found:
            report.files_failed.append((path, "no such file or directory"))
        targets.extend(found)

    for file_path in targets:
        try:
            raw = Path(file_path).read_text(encoding="utf-8", errors="replace")
        except Exception as exc:  # pragma: no cover - unusual IO failure
            report.files_failed.append((file_path, str(exc)))
            continue
        report.chunks_added += _ingest_text(
            os.path.basename(file_path), raw, embedder=embedder, store=store, cfg=cfg
        )
        report.files_ok.append(file_path)

    return report


def ingest_documents(
    docs: dict[str, str],
    *,
    embedder: Embedder,
    store: VectorStore,
    cfg: Config,
) -> IngestReport:
    """Ingest documents given as ``filename -> text``."""
    report = IngestReport()
    for filename, text in docs.items():
        report.chunks_added += _ingest_text(
            filename, text, embedder=embedder, store=store, cfg=cfg
        )
        report.files_ok.append(filename)
    return report


def _ingest_text(
    filename: str, text: str, *, embedder: Embedder, store: VectorStore, cfg: Config
) -> int:
    chunks = chunk_text(
        text,
        filename=filename,
        chunk_tokens=cfg.chunk_tokens,
        overlap=cfg.chunk_overlap,
    )
    if not chunks:
        return 0
    vectors = embedder.embed_texts([c.text for c in chunks])
    embedded: list[Chunk] = [c.with_embedding(v) for c, v in zip(chunks, vectors)]
    store.upsert(embedded)
    return len(embedded)
