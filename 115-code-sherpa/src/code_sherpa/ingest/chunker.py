"""Splits source files into ``CodeChunk`` objects.

Python is chunked by AST, everything else (and Python that fails to parse) by
line windows. Chunks over ``max_chunk_chars`` are split further, and tiny
neighbouring module/window chunks are merged.
"""

from __future__ import annotations

from pathlib import Path

from ..models import CodeChunk, FileRecord
from .strategies import AstChunker, ChunkStrategy, LineWindowChunker, RawChunk

_MERGEABLE = {"module", "window"}


class CodeChunker:
    def __init__(
        self,
        max_chunk_chars: int = 4000,
        min_chunk_chars: int = 40,
        window_size: int = 60,
        window_overlap: int = 10,
    ) -> None:
        self.max_chunk_chars = max_chunk_chars
        self.min_chunk_chars = min_chunk_chars
        self.window_size = window_size
        self.window_overlap = window_overlap
        self._window = LineWindowChunker(window_size, window_overlap)

    def chunk_file(self, project: str, record: FileRecord) -> list[CodeChunk]:
        try:
            source = Path(record.abs_path).read_text(encoding="utf-8", errors="replace")
        except OSError:
            return []
        return self.chunk_source(
            project, record.path, record.language, source, record.sha256
        )

    def chunk_source(
        self, project: str, path: str, language: str, source: str, sha256: str
    ) -> list[CodeChunk]:
        if not source.strip():
            return []
        raw = self._strategy(language).chunk_safe(source, self._window)
        raw = self._subsplit(source, raw)
        raw = self._merge_tiny(raw)
        return [
            CodeChunk.make(
                project=project,
                path=path,
                language=language,
                kind=rc.kind,
                symbol=rc.symbol,
                start_line=rc.start_line,
                end_line=rc.end_line,
                code=rc.code,
                sha256=sha256,
            )
            for rc in raw
        ]

    def _strategy(self, language: str) -> "_SafeStrategy":
        if language == "python":
            return _SafeStrategy(AstChunker())
        return _SafeStrategy(self._window)

    def _subsplit(self, source: str, chunks: list[RawChunk]) -> list[RawChunk]:
        lines = source.splitlines()
        out: list[RawChunk] = []
        for rc in chunks:
            if len(rc.code) <= self.max_chunk_chars:
                out.append(rc)
                continue
            out.extend(self._window_split(lines, rc))
        out.sort(key=lambda c: (c.start_line, c.end_line))
        return out

    def _window_split(self, lines: list[str], rc: RawChunk) -> list[RawChunk]:
        """Split an oversized chunk into parts of at most ``max_chunk_chars``.

        Parts are made of whole lines (so a single huge line can still exceed
        the limit) and consecutive parts share ``window_overlap`` lines.
        """
        abs_lines = list(range(rc.start_line, rc.end_line + 1))
        n = len(abs_lines)
        overlap = self.window_overlap
        parts: list[RawChunk] = []
        i = 0
        part_no = 0
        while i < n:
            cur: list[int] = []
            cur_len = 0
            j = i
            while j < n:
                text = lines[abs_lines[j] - 1]
                add = len(text) + (1 if cur else 0)
                if cur and cur_len + add > self.max_chunk_chars:
                    break
                cur.append(abs_lines[j])
                cur_len += add
                j += 1
            code = "\n".join(lines[s - 1] for s in cur)
            symbol = f"{rc.symbol}#part{part_no}" if rc.symbol else None
            if code.strip():
                parts.append(RawChunk(rc.kind, symbol, cur[0], cur[-1], code))
                part_no += 1
            if j >= n:
                break
            i = max(j - overlap, i + 1)
        return parts

    def _merge_tiny(self, chunks: list[RawChunk]) -> list[RawChunk]:
        merged: list[RawChunk] = []
        for rc in chunks:
            if (
                merged
                and rc.kind in _MERGEABLE
                and merged[-1].kind in _MERGEABLE
                and len(merged[-1].code) < self.min_chunk_chars
                and len(rc.code) < self.min_chunk_chars
                and rc.start_line <= merged[-1].end_line + 1
            ):
                prev = merged.pop()
                start = prev.start_line
                end = max(prev.end_line, rc.end_line)
                code = (prev.code + "\n" + rc.code).strip("\n")
                kind = prev.kind if prev.kind == rc.kind else "module"
                merged.append(RawChunk(kind, None, start, end, code))
            else:
                merged.append(rc)
        return merged


class _SafeStrategy:
    """Wraps a strategy so a parse failure falls back to line windows."""

    def __init__(self, strategy: ChunkStrategy) -> None:
        self._strategy = strategy

    def chunk_safe(self, source: str, fallback: LineWindowChunker) -> list[RawChunk]:
        try:
            chunks = self._strategy.chunk(source)
        except (SyntaxError, ValueError, RecursionError):
            chunks = fallback.chunk(source)
        if not chunks and source.strip():
            chunks = fallback.chunk(source)
        return chunks
