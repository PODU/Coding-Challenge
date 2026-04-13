"""Picks which retrieved chunks fit in the prompt's character budget.

Chunks contained in a better-ranked one are dropped. The best chunk from each
file goes in first and the rest fill whatever room is left, so one big file
can't crowd everything else out.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from ..models import Citation, CodeChunk, ScoredChunk


@dataclass(frozen=True)
class SelectedContext:
    items: list[ScoredChunk] = field(default_factory=list)
    citations: list[Citation] = field(default_factory=list)
    insufficient_context: bool = False
    used_chars: int = 0


def _contains(outer: CodeChunk, inner: CodeChunk) -> bool:
    return (
        outer.path == inner.path
        and outer.start_line <= inner.start_line
        and outer.end_line >= inner.end_line
    )


class ContextSelector:
    def __init__(self, char_budget: int = 8000) -> None:
        self.char_budget = char_budget

    def select(self, scored: list[ScoredChunk]) -> SelectedContext:
        if not scored:
            return SelectedContext(insufficient_context=True)

        ordered = sorted(
            scored, key=lambda s: (-s.score, s.chunk.path, s.chunk.start_line)
        )
        deduped = self._dedupe(ordered)

        selected: list[ScoredChunk] = []
        used = 0
        seen_files: set[str] = set()

        def try_add(sc: ScoredChunk) -> None:
            nonlocal used
            cost = len(sc.chunk.code)
            if used + cost <= self.char_budget:
                selected.append(sc)
                seen_files.add(sc.chunk.path)
                used += cost

        # First the best chunk from each file, then the rest by score.
        for sc in deduped:
            if sc.chunk.path not in seen_files:
                try_add(sc)
        already = {id(sc) for sc in selected}
        for sc in deduped:
            if id(sc) not in already:
                try_add(sc)

        # If even the top chunk is too big, send a truncated copy rather than
        # nothing.
        if not selected and deduped:
            top = deduped[0]
            truncated = top.chunk.code[: self.char_budget]
            sc = ScoredChunk(
                chunk=_with_code(top.chunk, truncated),
                score=top.score,
                vector_rank=top.vector_rank,
                text_rank=top.text_rank,
            )
            selected = [sc]
            used = len(truncated)

        selected.sort(key=lambda s: (-s.score, s.chunk.path, s.chunk.start_line))
        citations = [
            Citation(
                path=sc.chunk.path,
                start_line=sc.chunk.start_line,
                end_line=sc.chunk.end_line,
                symbol=sc.chunk.symbol,
            )
            for sc in selected
        ]
        return SelectedContext(
            items=selected,
            citations=citations,
            insufficient_context=False,
            used_chars=used,
        )

    def _dedupe(self, ordered: list[ScoredChunk]) -> list[ScoredChunk]:
        kept: list[ScoredChunk] = []
        for sc in ordered:
            if not any(_contains(k.chunk, sc.chunk) for k in kept):
                kept.append(sc)
        return kept


def _with_code(chunk: CodeChunk, code: str) -> CodeChunk:
    return CodeChunk(
        id=chunk.id,
        project=chunk.project,
        path=chunk.path,
        language=chunk.language,
        kind=chunk.kind,
        symbol=chunk.symbol,
        start_line=chunk.start_line,
        end_line=chunk.end_line,
        code=code,
        sha256=chunk.sha256,
    )
