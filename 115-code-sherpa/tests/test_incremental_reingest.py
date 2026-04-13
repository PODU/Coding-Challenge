"""Incremental re-ingest: only changed files re-embedded; removals delete vectors."""

from __future__ import annotations

from pathlib import Path

from code_sherpa.embed.mock import MockEmbedder
from code_sherpa.store.memory_store import InMemoryVectorStore

from conftest import SpyEmbedder


def _setup(make_sherpa, fixture_repo):
    spy = SpyEmbedder(MockEmbedder(dim=128))
    store = InMemoryVectorStore()
    sherpa = make_sherpa(embedder=spy, store=store)
    report = sherpa.ingest("demo", str(fixture_repo))
    return sherpa, spy, store, report


def test_first_ingest_embeds_everything(make_sherpa, fixture_repo):
    sherpa, spy, store, report = _setup(make_sherpa, fixture_repo)
    assert report.files_ingested >= 3  # payments, auth, NOTES, weird.xyz
    assert report.chunks_indexed > 0
    assert len(spy.texts) == report.chunks_indexed


def test_unchanged_reingest_embeds_nothing(make_sherpa, fixture_repo):
    sherpa, spy, store, _ = _setup(make_sherpa, fixture_repo)
    before = len(spy.texts)
    report = sherpa.reingest("demo")
    assert report.files_ingested == 0
    assert len(spy.texts) == before  # no re-embedding


def test_changed_file_only_reembeds_that_file(make_sherpa, fixture_repo):
    sherpa, spy, store, _ = _setup(make_sherpa, fixture_repo)
    before = len(spy.texts)

    # Modify exactly one file.
    auth = Path(fixture_repo) / "src" / "auth.py"
    auth.write_text(
        auth.read_text(encoding="utf-8") + "\n\ndef refresh(token):\n    return token\n",
        encoding="utf-8",
    )

    report = sherpa.reingest("demo")
    assert report.files_ingested == 1
    delta_texts = spy.texts[before:]
    # Every newly embedded chunk came from auth.py (re-chunked), and includes the
    # new function.
    assert any("def refresh" in t for t in delta_texts)
    assert all("def login" in t or "def logout" in t or "def refresh" in t or "Authentication" in t
               for t in delta_texts)
    # The new symbol is now retrievable.
    hits = store.text_search("demo", "refresh token", 5)
    assert any(h.chunk.symbol == "refresh" for h in hits)


def test_removed_file_deletes_its_vectors(make_sherpa, fixture_repo):
    sherpa, spy, store, _ = _setup(make_sherpa, fixture_repo)
    assert "src/auth.py" in store.paths("demo")
    count_before = store.count("demo")

    (Path(fixture_repo) / "src" / "auth.py").unlink()

    report = sherpa.reingest("demo")
    assert report.files_removed == 1
    assert "src/auth.py" not in store.paths("demo")
    assert store.count("demo") < count_before
    # auth symbols no longer retrievable.
    hits = store.text_search("demo", "login user password", 5)
    assert all(h.chunk.path != "src/auth.py" for h in hits)


def test_binary_and_gitignored_files_are_skipped(make_sherpa, fixture_repo):
    sherpa, spy, store, _ = _setup(make_sherpa, fixture_repo)
    paths = store.paths("demo")
    assert "blob.bin" not in paths  # binary skipped
    assert "build/artifact.py" not in paths  # default ignore dir
    assert not any("ignored_dir" in p for p in paths)  # .gitignore dir
    assert "app.log" not in paths  # .gitignore *.log
    assert "src/payments.py" in paths
