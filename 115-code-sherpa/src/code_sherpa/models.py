"""Data types shared by ingest, retrieval, prompting and the LLM layer."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

Vector = list[float]


def chunk_id(project: str, path: str, start_line: int, end_line: int) -> str:
    return f"{project}:{path}:{start_line}-{end_line}"


@dataclass(frozen=True)
class FileRecord:
    path: str  # relative to the repo root, with forward slashes
    abs_path: str
    language: str
    size: int
    sha256: str


@dataclass(frozen=True)
class CodeChunk:
    id: str
    project: str
    path: str
    language: str
    kind: str  # "function" | "class" | "method" | "module" | "window"
    symbol: Optional[str]
    start_line: int
    end_line: int
    code: str
    sha256: str  # of the whole source file

    @property
    def line_count(self) -> int:
        return self.end_line - self.start_line + 1

    @staticmethod
    def make(
        project: str,
        path: str,
        language: str,
        kind: str,
        symbol: Optional[str],
        start_line: int,
        end_line: int,
        code: str,
        sha256: str,
    ) -> "CodeChunk":
        return CodeChunk(
            id=chunk_id(project, path, start_line, end_line),
            project=project,
            path=path,
            language=language,
            kind=kind,
            symbol=symbol,
            start_line=start_line,
            end_line=end_line,
            code=code,
            sha256=sha256,
        )


@dataclass(frozen=True)
class ScoredChunk:
    chunk: CodeChunk
    score: float
    vector_rank: Optional[int] = None
    text_rank: Optional[int] = None


@dataclass(frozen=True)
class Citation:
    path: str
    start_line: int
    end_line: int
    symbol: Optional[str] = None

    def label(self) -> str:
        loc = f"{self.path}:{self.start_line}-{self.end_line}"
        return f"{loc} ({self.symbol})" if self.symbol else loc


@dataclass(frozen=True)
class Answer:
    text: str
    citations: list[Citation] = field(default_factory=list)
    used_memory: bool = False
    insufficient_context: bool = False
    classification: Optional[str] = None
    error: Optional[str] = None
