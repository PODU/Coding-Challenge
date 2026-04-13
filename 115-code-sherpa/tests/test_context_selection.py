"""Context selection: budget, overlap de-dup, file diversity, no-match."""

from __future__ import annotations

from code_sherpa.models import CodeChunk, ScoredChunk
from code_sherpa.prompt.select import ContextSelector


def mk(path, start, end, code, score, symbol=None, kind="function"):
    chunk = CodeChunk.make("p", path, "python", kind, symbol, start, end, code, "sha")
    return ScoredChunk(chunk=chunk, score=score)


def test_no_match_flags_insufficient():
    out = ContextSelector().select([])
    assert out.insufficient_context is True
    assert out.items == []
    assert out.citations == []


def test_respects_char_budget():
    chunks = [mk(f"f{i}.py", 1, 10, "x" * 100, score=1.0 - i * 0.01) for i in range(20)]
    selector = ContextSelector(char_budget=250)
    out = selector.select(chunks)
    assert out.used_chars <= 250
    assert sum(len(s.chunk.code) for s in out.items) <= 250


def test_dedupes_contained_overlapping_ranges():
    big = mk("a.py", 1, 30, "class C: ...", score=0.9, symbol="C", kind="class")
    inner = mk("a.py", 5, 8, "def m(self): ...", score=0.8, symbol="C.m", kind="method")
    other = mk("b.py", 1, 4, "def f(): ...", score=0.7, symbol="f")
    out = ContextSelector(char_budget=10_000).select([big, inner, other])
    paths_lines = {(s.chunk.path, s.chunk.start_line) for s in out.items}
    assert ("a.py", 1) in paths_lines
    assert ("a.py", 5) not in paths_lines  # contained chunk dropped
    assert ("b.py", 1) in paths_lines


def test_prefers_file_diversity():
    # Two high-scoring chunks from the same file, one lower from another file.
    a1 = mk("a.py", 1, 5, "aaa", score=0.99, symbol="a1")
    a2 = mk("a.py", 10, 15, "bbb", score=0.98, symbol="a2")
    b1 = mk("b.py", 1, 5, "ccc", score=0.50, symbol="b1")
    # Budget fits exactly two chunks of len 3.
    out = ContextSelector(char_budget=6).select([a1, a2, b1])
    files = {s.chunk.path for s in out.items}
    assert files == {"a.py", "b.py"}  # diversity beat the 2nd a.py chunk


def test_citations_match_selected_chunks():
    c = mk("a.py", 3, 9, "def f(): ...", score=1.0, symbol="f")
    out = ContextSelector().select([c])
    assert len(out.citations) == 1
    cit = out.citations[0]
    assert (cit.path, cit.start_line, cit.end_line, cit.symbol) == ("a.py", 3, 9, "f")
