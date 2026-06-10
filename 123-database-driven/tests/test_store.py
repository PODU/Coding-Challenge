from __future__ import annotations

import pytest

from llmwiki import (
    Contradiction,
    EmbeddingDimMismatch,
    Frontmatter,
    HashingEmbedder,
    Operation,
    SqliteStore,
    WikiPage,
)
from llmwiki.fusion import reciprocal_rank_fusion
from llmwiki.domain import Hit


def _page(pid, title, body, project="p", type="entity"):
    fm = Frontmatter(id=pid, type=type, title=title, project=project)
    page = WikiPage(frontmatter=fm, body=body)
    page.recompute_sha()
    return page


@pytest.fixture
def seeded(store):
    e = HashingEmbedder(dim=64)
    store.init_project("p")
    pages = {
        "entities/ada-lovelace": "Ada Lovelace pioneered computing and wrote the first algorithm.",
        "entities/alan-turing": "Alan Turing created the Turing Machine model of computation.",
        "entities/grace-hopper": "Grace Hopper built the first compiler for programming languages.",
    }
    for pid, body in pages.items():
        page = _page(pid, pid.split("/")[-1], body)
        store.upsert_page("p", page, e.embed([page.embedding_text()])[0])
    return store, e


def test_upsert_get_all_delete(seeded):
    store, _ = seeded
    assert store.get_page("p", "entities/ada-lovelace") is not None
    assert len(store.all_pages("p")) == 3
    store.delete_page("p", "entities/ada-lovelace")
    assert store.get_page("p", "entities/ada-lovelace") is None
    assert len(store.all_pages("p")) == 2


def test_vector_search_ranks_nearest_first(seeded):
    store, e = seeded
    qvec = e.embed(["Turing Machine model of computation"])[0]
    hits = store.vector_search("p", qvec, k=3)
    assert hits[0].page_id == "entities/alan-turing"
    assert all(h.via == "vector" for h in hits)


def test_fulltext_search_matches_terms(seeded):
    store, _ = seeded
    hits = store.fulltext_search("p", "compiler programming", k=3)
    assert hits
    assert hits[0].page_id == "entities/grace-hopper"


def test_hybrid_search_finds_relevant(seeded):
    store, e = seeded
    qvec = e.embed(["who wrote the first algorithm"])[0]
    hits = store.hybrid_search("p", "first algorithm", qvec, k=3)
    assert hits[0].page_id == "entities/ada-lovelace"


def test_dim_mismatch_raises(seeded):
    store, _ = seeded
    with pytest.raises(EmbeddingDimMismatch):
        store.vector_search("p", [0.1] * 999, k=3)


def test_project_isolation(store):
    e = HashingEmbedder(dim=64)
    store.init_project("a")
    store.init_project("b")
    page = _page("entities/x", "X", "secret content only in a", project="a")
    store.upsert_page("a", page, e.embed([page.embedding_text()])[0])
    assert store.get_page("a", "entities/x") is not None
    assert store.get_page("b", "entities/x") is None
    assert store.all_pages("b") == []
    qvec = e.embed(["secret content"])[0]
    assert store.vector_search("b", qvec, k=5) == []
    assert store.fulltext_search("b", "secret", k=5) == []


def test_oplog_ordering(store):
    store.init_project("p")
    for i in range(3):
        store.append_log("p", Operation(op="upsert_page", page_id=f"x{i}"))
    ops = store.read_log("p")
    assert [o.seq for o in ops] == [1, 2, 3]
    assert [o.page_id for o in ops] == ["x0", "x1", "x2"]


def test_contradiction_store(store):
    store.init_project("p")
    c = Contradiction(id="c1", entity="Ada", page_a="a", claim_a="x",
                      page_b="b", claim_b="y")
    store.add_contradiction("p", c)
    out = store.list_contradictions("p")
    assert len(out) == 1 and out[0].entity == "Ada"


def test_reopen_from_file(tmp_path):
    dbfile = str(tmp_path / "wiki.db")
    e = HashingEmbedder(dim=64)
    s1 = SqliteStore(dbfile)
    s1.init_project("p")
    page = _page("entities/x", "X", "persisted body")
    s1.upsert_page("p", page, e.embed([page.embedding_text()])[0])
    s1.close()
    s2 = SqliteStore(dbfile)
    assert s2.get_page("p", "entities/x").body == "persisted body"


def test_rrf_known_order():
    vec = [Hit("a", 0.9, "vector"), Hit("b", 0.8, "vector")]
    ft = [Hit("b", 5.0, "fulltext"), Hit("c", 4.0, "fulltext")]
    fused = reciprocal_rank_fusion([vec, ft])
    ids = [h.page_id for h in fused]
    assert ids[0] == "b"  # appears in both lists -> highest fused score
    assert set(ids) == {"a", "b", "c"}
    assert next(h for h in fused if h.page_id == "b").via == "both"
