"""Code chunking: functions/classes/methods, fallback, sub-split, merge, skip."""

from __future__ import annotations

from code_sherpa.ingest.chunker import CodeChunker
from code_sherpa.ingest.strategies import AstChunker, LineWindowChunker, RawChunk

from conftest import AUTH_PY, PAYMENTS_PY, UNKNOWN_SRC


def _by_symbol(chunks):
    return {c.symbol: c for c in chunks if c.symbol}


def test_python_functions_classes_methods():
    chunker = CodeChunker()
    chunks = chunker.chunk_source("proj", "src/payments.py", "python", PAYMENTS_PY, "sha1")
    by_sym = _by_symbol(chunks)

    assert "charge" in by_sym
    assert by_sym["charge"].kind == "function"
    assert by_sym["charge"].start_line == 9
    assert by_sym["charge"].end_line == 11

    assert by_sym["PaymentService"].kind == "class"
    assert by_sym["PaymentService"].start_line == 14
    assert by_sym["PaymentService"].end_line == 30

    assert by_sym["PaymentService.refund"].kind == "method"
    assert by_sym["PaymentService.refund"].start_line == 20
    assert by_sym["PaymentService.refund"].end_line == 22

    assert by_sym["PaymentService.handle_payment_failure"].kind == "method"

    # A module-level chunk covers the imports/constants at the top.
    module_chunks = [c for c in chunks if c.kind == "module"]
    assert module_chunks
    assert module_chunks[0].start_line == 1


def test_chunk_ids_are_stable_and_unique():
    chunker = CodeChunker()
    chunks = chunker.chunk_source("proj", "src/auth.py", "python", AUTH_PY, "shaA")
    ids = [c.id for c in chunks]
    assert len(ids) == len(set(ids))
    login = next(c for c in chunks if c.symbol == "login")
    assert login.id == f"proj:src/auth.py:{login.start_line}-{login.end_line}"
    assert all(c.sha256 == "shaA" for c in chunks)


def test_decorators_included_in_span():
    src = "@decorator\n@another\ndef wrapped():\n    return 1\n"
    raw = AstChunker().chunk(src)
    fn = next(c for c in raw if c.symbol == "wrapped")
    assert fn.start_line == 1  # first decorator line
    assert fn.end_line == 4


def test_unknown_language_falls_back_to_line_window():
    chunker = CodeChunker(window_size=2, window_overlap=1)
    chunks = chunker.chunk_source("proj", "weird.xyz", "unknown", UNKNOWN_SRC, "shaU")
    assert chunks
    assert all(c.kind == "window" for c in chunks)
    # Overlap: consecutive windows share a line (next.start <= prev.end).
    for a, b in zip(chunks, chunks[1:]):
        assert b.start_line <= a.end_line


def test_line_window_overlap_exact():
    src = "\n".join(f"line{i}" for i in range(1, 11))  # 10 lines
    raw = LineWindowChunker(window=4, overlap=2).chunk(src)
    assert raw[0].start_line == 1 and raw[0].end_line == 4
    assert raw[1].start_line == 3  # step = window - overlap = 2


def test_syntax_error_falls_back_without_crashing():
    chunker = CodeChunker()
    bad = "def broken(:\n    this is not python\n"
    chunks = chunker.chunk_source("proj", "bad.py", "python", bad, "shaB")
    assert chunks  # produced via window fallback
    assert all(c.kind == "window" for c in chunks)


def test_overlong_function_is_subsplit():
    body = "\n".join(f"    x{i} = {i}" for i in range(60))
    src = "def big():\n" + body + "\n"
    chunker = CodeChunker(max_chunk_chars=120, window_size=10, window_overlap=2)
    chunks = chunker.chunk_source("proj", "big.py", "python", src, "shaG")
    parts = [c for c in chunks if c.symbol and c.symbol.startswith("big")]
    assert len(parts) >= 2  # was sub-split
    assert all(len(c.code) <= 120 for c in parts)
    # Coverage: parts span from line 1 to the final line of the function.
    assert min(c.start_line for c in parts) == 1
    assert max(c.end_line for c in parts) == src.count("\n")


def test_empty_source_skipped():
    chunker = CodeChunker()
    assert chunker.chunk_source("proj", "empty.py", "python", "   \n\n", "shaE") == []


def test_merge_tiny_adjacent_module_chunks():
    chunker = CodeChunker(min_chunk_chars=40)
    tiny = [
        RawChunk("module", None, 1, 1, "a = 1"),
        RawChunk("module", None, 2, 2, "b = 2"),
    ]
    merged = chunker._merge_tiny(tiny)
    assert len(merged) == 1
    assert merged[0].start_line == 1 and merged[0].end_line == 2
    assert "a = 1" in merged[0].code and "b = 2" in merged[0].code
