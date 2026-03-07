"""Doc/man-page parsing and chunking (pure)."""

from __future__ import annotations

from rtfm.chunker import chunk_text, count_tokens, normalize


def _long_doc(n_words: int = 600) -> str:
    body = " ".join(f"word{i}" for i in range(n_words))
    return f"# title\n\n## SECTION ONE\n{body}\n"


def test_chunks_bounded_by_token_limit():
    chunks = chunk_text(_long_doc(600), filename="d.txt", chunk_tokens=100, overlap=20)
    assert len(chunks) > 1
    for c in chunks:
        assert count_tokens(c.text) <= 100


def test_overlap_reconstructs_source():
    text = _long_doc(300)
    chunk_tokens, overlap = 100, 20
    chunks = chunk_text(text, filename="d.txt", chunk_tokens=chunk_tokens, overlap=overlap)

    rebuilt = list(chunks[0].text.split())
    for c in chunks[1:]:
        rebuilt.extend(c.text.split()[overlap:])

    expected = " ".join(normalize(text).split())
    assert " ".join(rebuilt) == expected


def test_metadata_filename_section_and_monotonic_position():
    text = "# title\n\n## NAME\nalpha beta gamma\n\n## DESCRIPTION\ndelta epsilon zeta eta theta\n"
    chunks = chunk_text(text, filename="man.txt", chunk_tokens=5, overlap=0)
    assert all(c.filename == "man.txt" for c in chunks)
    assert [c.position for c in chunks] == list(range(len(chunks)))
    sections = {c.section for c in chunks}
    assert "NAME" in sections
    assert "DESCRIPTION" in sections


def test_empty_file_yields_zero_chunks():
    assert chunk_text("", filename="e.txt") == []
    assert chunk_text("   \n\n  ", filename="e.txt") == []


def test_short_file_yields_one_chunk():
    chunks = chunk_text("just a few words here", filename="s.txt", chunk_tokens=500)
    assert len(chunks) == 1
    assert chunks[0].position == 0


def test_manpage_backspace_bold_is_normalized():
    # "NAME" rendered with backspace-bold overstrike (each char doubled).
    raw = "N\x08NA\x08AM\x08ME\x08E\nthe _\x08tool description here\n"
    out = normalize(raw)
    assert "NAME" in out
    assert "\x08" not in out
    # The underline overstrike "_\x08t" collapses to "t".
    assert "tool description" in out


def test_no_chunk_exceeds_limit_on_real_markdown():
    md = "## OPTIONS\n" + " ".join([f"flag{i}" for i in range(250)])
    chunks = chunk_text(md, filename="grep.md", chunk_tokens=50, overlap=10)
    assert chunks
    assert all(count_tokens(c.text) <= 50 for c in chunks)
