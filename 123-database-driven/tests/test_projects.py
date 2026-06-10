from __future__ import annotations

from tests.conftest import ADA_DOC, TURING_DOC


def test_project_lifecycle(wiki):
    wiki.create_project("a")
    assert "a" in wiki.list_projects()
    wiki.delete_project("a")
    assert "a" not in wiki.list_projects()


def test_project_isolation_end_to_end(wiki):
    wiki.create_project("a")
    wiki.create_project("b")
    wiki.ingest_text("a", ADA_DOC, title="Ada")
    wiki.ingest_text("b", TURING_DOC, title="Turing")

    a_ids = {p.id for p in wiki.browse("a")}
    b_ids = {p.id for p in wiki.browse("b")}
    assert "entities/ada-lovelace" in a_ids
    assert "entities/ada-lovelace" not in b_ids
    assert "entities/alan-turing" in b_ids
    assert "entities/alan-turing" not in a_ids

    # a query in A can never retrieve B's pages
    hits = wiki.search("a", "Turing Machine", k=10)
    assert all("turing" not in h.page_id for h in hits)

    ans = wiki.ask("a", "Tell me about Alan Turing")
    for c in ans.citations:
        assert c["page_id"] in a_ids


def test_delete_project_removes_files(wiki, tmp_path):
    wiki.create_project("a")
    wiki.ingest_text("a", ADA_DOC, title="Ada")
    assert (tmp_path / "a").exists()
    wiki.delete_project("a")
    assert not (tmp_path / "a").exists()
