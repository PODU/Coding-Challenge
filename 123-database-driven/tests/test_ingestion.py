from __future__ import annotations

import pytest

from llmwiki import EmptyDocumentError
from tests.conftest import ADA_DOC


def _ids(pages):
    return {p.id for p in pages}


def test_ingest_creates_pages_with_frontmatter_and_links(wiki):
    wiki.create_project("p")
    res = wiki.ingest_text("p", ADA_DOC, title="Ada")
    ids = _ids(wiki.browse("p"))
    assert "summaries/ada" in ids
    assert "entities/ada-lovelace" in ids
    assert "entities/charles-babbage" in ids
    # a topic page was created from tags
    assert any(i.startswith("topics/") for i in ids)

    summary = wiki.get_page("p", "summaries/ada")
    assert summary.frontmatter.type == "summary"
    assert summary.frontmatter.project == "p"
    assert summary.frontmatter.content_sha256
    assert summary.frontmatter.sources  # provenance recorded
    # summary links to its entities
    assert "entities/ada-lovelace" in summary.frontmatter.links

    # entity back-links to the summary
    ada = wiki.get_page("p", "entities/ada-lovelace")
    assert "summaries/ada" in ada.frontmatter.links
    assert res.pages_created


def test_ingest_writes_markdown_files_and_log(wiki, tmp_path):
    wiki.create_project("p")
    wiki.ingest_text("p", ADA_DOC, title="Ada")
    base = tmp_path / "p"
    assert (base / "wiki" / "summaries" / "ada.md").exists()
    assert (base / "wiki" / "entities" / "ada-lovelace.md").exists()
    assert (base / "index.md").exists()
    log = (base / "log.jsonl").read_text(encoding="utf-8").strip().splitlines()
    assert len(log) >= 2  # ingest_source + upsert_page ops


def test_reingest_identical_is_noop(wiki):
    wiki.create_project("p")
    wiki.ingest_text("p", ADA_DOC, title="Ada")
    before = {p.id: p.frontmatter.content_sha256 for p in wiki.browse("p")}
    log_before = len(wiki.read_log("p"))

    res2 = wiki.ingest("p", ADA_DOC)  # same content via path-or-text
    after = {p.id: p.frontmatter.content_sha256 for p in wiki.browse("p")}
    assert before == after
    assert res2.pages_created == []
    assert res2.pages_updated == []
    assert res2.skipped  # recognised as already-ingested
    assert len(wiki.read_log("p")) == log_before  # no new ops


def test_reingest_updated_reembeds(wiki):
    wiki.create_project("p")
    wiki.ingest_text("p", ADA_DOC, title="Ada")
    sha_before = wiki.get_page("p", "summaries/ada").frontmatter.content_sha256
    updated = ADA_DOC.replace("a pioneer", "a celebrated pioneer")
    res = wiki.ingest_text("p", updated, title="Ada")
    sha_after = wiki.get_page("p", "summaries/ada").frontmatter.content_sha256
    assert sha_before != sha_after
    assert "summaries/ada" in res.pages_updated


def test_empty_document_rejected(wiki):
    wiki.create_project("p")
    with pytest.raises(EmptyDocumentError):
        wiki.ingest_text("p", "   \n  ")


def test_ingest_directory_corpus(wiki, corpus_dir):
    wiki.create_project("p")
    res = wiki.ingest("p", corpus_dir)
    assert len(res.source_ids) >= 50  # the sample corpus has 50-100 docs
    pages = wiki.browse("p")
    assert any(p.type == "entity" for p in pages)
    assert any(p.type == "summary" for p in pages)
    assert any(p.type == "topic" for p in pages)
