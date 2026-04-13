"""Chunking strategies: ``AstChunker`` for Python, ``LineWindowChunker`` for
everything else.

They return bare :class:`RawChunk` values; ``CodeChunker`` adds the project and
hash and does the splitting and merging.
"""

from __future__ import annotations

import ast
from abc import ABC, abstractmethod
from dataclasses import dataclass


@dataclass(frozen=True)
class RawChunk:
    kind: str  # function | class | method | module | window
    symbol: str | None
    start_line: int  # 1-based, inclusive
    end_line: int  # 1-based, inclusive
    code: str


class ChunkStrategy(ABC):
    @abstractmethod
    def chunk(self, source: str) -> list[RawChunk]:
        ...


def _slice_lines(lines: list[str], start: int, end: int) -> str:
    """Lines start..end of the source, 1-based and inclusive."""
    return "\n".join(lines[start - 1 : end])


class AstChunker(ChunkStrategy):
    """Chunks Python by AST: one chunk per top-level function and class, one per
    method, and ``module`` chunks for the top-level code in between (imports,
    constants, ``__main__`` blocks).
    """

    def chunk(self, source: str) -> list[RawChunk]:
        tree = ast.parse(source)
        lines = source.splitlines()
        chunks: list[RawChunk] = []
        pending: list[ast.stmt] = []

        def flush_pending() -> None:
            if not pending:
                return
            start = self._start_line(pending[0])
            end = self._end_line(pending[-1])
            code = _slice_lines(lines, start, end)
            if code.strip():
                chunks.append(RawChunk("module", None, start, end, code))
            pending.clear()

        for node in tree.body:
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                flush_pending()
                chunks.append(self._fn_chunk(node, lines, "function", None))
            elif isinstance(node, ast.ClassDef):
                flush_pending()
                chunks.extend(self._class_chunks(node, lines))
            else:
                pending.append(node)
        flush_pending()
        chunks.sort(key=lambda c: (c.start_line, c.end_line))
        return chunks

    def _class_chunks(self, node: ast.ClassDef, lines: list[str]) -> list[RawChunk]:
        start = self._start_line(node)
        end = self._end_line(node)
        out: list[RawChunk] = [
            RawChunk("class", node.name, start, end, _slice_lines(lines, start, end))
        ]
        for child in node.body:
            if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                out.append(
                    self._fn_chunk(child, lines, "method", parent=node.name)
                )
        return out

    def _fn_chunk(
        self,
        node: ast.FunctionDef | ast.AsyncFunctionDef,
        lines: list[str],
        kind: str,
        parent: str | None,
    ) -> RawChunk:
        start = self._start_line(node)
        end = self._end_line(node)
        symbol = f"{parent}.{node.name}" if parent else node.name
        return RawChunk(kind, symbol, start, end, _slice_lines(lines, start, end))

    @staticmethod
    def _start_line(node: ast.stmt) -> int:
        # Include decorators in the chunk span.
        decorators = getattr(node, "decorator_list", []) or []
        if decorators:
            return min(d.lineno for d in decorators)
        return node.lineno

    @staticmethod
    def _end_line(node: ast.stmt) -> int:
        return getattr(node, "end_lineno", node.lineno) or node.lineno


class LineWindowChunker(ChunkStrategy):
    """Fixed-size overlapping windows of lines."""

    def __init__(self, window: int = 60, overlap: int = 10) -> None:
        if overlap >= window:
            raise ValueError("overlap must be smaller than window")
        self.window = window
        self.overlap = overlap

    def chunk(self, source: str) -> list[RawChunk]:
        lines = source.splitlines()
        n = len(lines)
        if n == 0:
            return []
        step = self.window - self.overlap
        chunks: list[RawChunk] = []
        start = 0
        while start < n:
            end = min(start + self.window, n)
            code = "\n".join(lines[start:end])
            if code.strip():
                chunks.append(
                    RawChunk("window", None, start + 1, end, code)
                )
            if end >= n:
                break
            start += step
        return chunks
