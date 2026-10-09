import pytest
from conftest import HAS_GIT, commit_all, git

from ccctx.embed import HashEmbedder
from ccctx.ingest import ingest

needs_git = pytest.mark.skipif(not HAS_GIT, reason="git not installed")


def counts(store, repo="sample"):
    return {r["name"]: (r["file_count"], r["symbol_count"]) for r in store.repos()}[repo]


def sym_id(store, key):
    row = store.q1("SELECT id FROM symbols WHERE key = ?", (key,))
    return row[0] if row else None


def test_register_starts_empty(store, sample):
    store.add_repo("sample", str(sample))
    assert counts(store) == (0, 0)
    with pytest.raises(Exception):
        store.add_repo("sample", str(sample))


def test_first_ingest_report(store, sample):
    store.add_repo("sample", str(sample))
    rep = ingest(store, "sample")
    assert rep.binary == 1 and rep.ignored == 2  # logo.png; build/ and debug.log
    assert rep.symbols_indexed == rep.symbols_total > 20
    files, symbols = counts(store)
    assert files == rep.scanned - rep.binary - rep.too_large
    text = rep.render()
    assert "Scanned" in text and "Skipped" in text and "Indexed" in text
    langs = {r[0] for r in store.q("SELECT DISTINCT language FROM files")}
    assert {"python", "go", "typescript", "markdown"} <= langs
    kinds = {r[0] for r in store.q("SELECT DISTINCT kind FROM symbols")}
    assert {"class", "method", "function", "struct", "interface", "chunk"} <= kinds


def test_symbols_store_lines_source_and_embeddings(indexed):
    row = indexed.q1("SELECT * FROM symbols WHERE qualname = 'Router.dispatch'")
    assert row["language"] == "python" and row["kind"] == "method"
    assert row["source"].lstrip().startswith("def dispatch")
    assert len(row["embedding"]) == 384 * 4
    assert indexed.q1("SELECT COUNT(*) FROM symbols_fts WHERE rowid = ?", (row["id"],))[0] == 1
    callees = {r[0] for r in indexed.q("SELECT dst_name FROM calls WHERE src_id = ?", (row["id"],))}
    assert "not_found" in callees


@needs_git
def test_commit_sha_tracked_and_noop_reingest(indexed, sample):
    head = git(sample, "rev-parse", "HEAD")
    assert indexed.repo("sample")["commit_sha"] == head
    assert indexed.q1("SELECT DISTINCT commit_sha FROM symbols")[0] == head
    rep = ingest(indexed, "sample")
    assert rep.up_to_date and "nothing to do" in rep.render()


@needs_git
def test_incremental_modify_keeps_unchanged_symbol_ids(indexed, sample):
    keep = sym_id(indexed, "app/routing.py::make_response")
    changed = sym_id(indexed, "app/routing.py::not_found")
    text = (sample / "app/routing.py").read_text()
    (sample / "app/routing.py").write_text(text.replace('"not found"', '"missing"'))
    commit_all(sample, "edit")
    rep = ingest(indexed, "sample")
    assert rep.modified == ["app/routing.py"] and not rep.added and not rep.deleted
    assert rep.symbols_indexed == 1  # only not_found's code changed
    assert sym_id(indexed, "app/routing.py::make_response") == keep
    assert sym_id(indexed, "app/routing.py::not_found") == changed
    assert "missing" in indexed.q1("SELECT source FROM symbols WHERE id = ?", (changed,))[0]
    assert "Changed" in rep.render()


def test_incremental_without_git(store, tmp_path):
    from conftest import write_sample
    root = write_sample(tmp_path / "plain")
    store.add_repo("plain", str(root))
    ingest(store, "plain")
    rep = ingest(store, "plain")
    assert not rep.up_to_date and rep.changed == 0 and rep.symbols_indexed == 0
    (root / "app/extra.py").write_text("def extra():\n    return 1\n")
    rep = ingest(store, "plain")
    assert rep.added == ["app/extra.py"] and rep.symbols_indexed == 1


def test_delete_removes_orphans(indexed, sample):
    ids = [r[0] for r in indexed.q("SELECT s.id FROM symbols s JOIN files f ON f.id = s.file_id "
                                   "WHERE f.path = 'app/storage.py'")]
    assert ids
    (sample / "app/storage.py").unlink()
    rep = ingest(indexed, "sample")
    assert rep.deleted == ["app/storage.py"]
    marks = ",".join("?" * len(ids))
    assert indexed.q1(f"SELECT COUNT(*) FROM symbols WHERE id IN ({marks})", ids)[0] == 0
    assert indexed.q1(f"SELECT COUNT(*) FROM symbols_fts WHERE rowid IN ({marks})", ids)[0] == 0
    assert indexed.q1(f"SELECT COUNT(*) FROM calls WHERE src_id IN ({marks})", ids)[0] == 0
    assert indexed.q1("SELECT COUNT(*) FROM files WHERE path = 'app/storage.py'")[0] == 0


def test_rename_is_detected_and_keeps_ids(indexed, sample):
    before = sym_id(indexed, "app/storage.py::Database")
    (sample / "app/storage.py").rename(sample / "app/persistence.py")
    rep = ingest(indexed, "sample")
    assert rep.renamed == [("app/storage.py", "app/persistence.py")]
    assert rep.symbols_indexed == 0
    assert sym_id(indexed, "app/persistence.py::Database") == before


def test_removed_symbol_within_file(indexed, sample):
    text = (sample / "app/views.py").read_text()
    (sample / "app/views.py").write_text(text.split("def setup")[0])
    rep = ingest(indexed, "sample")
    assert rep.symbols_removed == 1
    assert sym_id(indexed, "app/views.py::setup") is None


def test_full_rebuild(indexed):
    total = indexed.q1("SELECT COUNT(*) FROM symbols")[0]
    rep = ingest(indexed, "sample", full=True)
    assert rep.full and rep.symbols_indexed == total
    assert indexed.q1("SELECT COUNT(*) FROM symbols")[0] == total
    assert indexed.q1("SELECT COUNT(*) FROM symbols_fts")[0] == total


def test_embedder_change_forces_full(indexed):
    class Other(HashEmbedder):
        name = "other"
    rep = ingest(indexed, "sample", embedder=Other())
    assert rep.full and any("embedder changed" in n for n in rep.notes)


def test_unknown_repo(store):
    with pytest.raises(KeyError):
        ingest(store, "nope")


def test_remove_repo_cleans_everything(indexed):
    assert indexed.remove_repo("sample")
    for t in ("files", "symbols", "symbols_fts", "calls", "imports"):
        assert indexed.q1(f"SELECT COUNT(*) FROM {t}")[0] == 0, t
