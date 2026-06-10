from __future__ import annotations

from tests.conftest import ADA_DOC, TURING_DOC


def _seed(wiki):
    wiki.create_project("p")
    wiki.ingest_text("p", ADA_DOC, title="Ada")
    wiki.ingest_text("p", TURING_DOC, title="Turing")


def test_hybrid_search_retrieves_relevant_page(wiki):
    _seed(wiki)
    hits = wiki.search("p", "Turing Machine model of computation", k=5)
    assert hits
    # the top hit is a Turing-related page, never an Ada page
    assert "turing" in hits[0].page_id
    assert all("ada" not in h.page_id for h in hits[:2])


def test_ask_returns_grounded_answer_with_citations(wiki):
    _seed(wiki)
    ans = wiki.ask("p", "What did Alan Turing create?")
    assert ans.answer
    assert ans.citations
    # citations reference real page ids and source ids
    pages = {p.id for p in wiki.browse("p")}
    for c in ans.citations:
        assert c["page_id"] in pages
    # grounded in Turing pages, not Ada
    assert any("turing" in c["page_id"] for c in ans.citations)
    assert ans.retrieved
    assert all("page_id" in r and "score" in r and "via" in r for r in ans.retrieved)


def test_ask_save_creates_retrievable_answer_page(wiki):
    _seed(wiki)
    ans = wiki.ask("p", "What did Alan Turing create?", save=True)
    assert ans.saved_as == "answers/what-did-alan-turing-create"
    saved = wiki.get_page("p", ans.saved_as)
    assert saved is not None and saved.frontmatter.type == "answer"
    # asking again updates the saved answer instead of duplicating it
    before = len([p for p in wiki.browse("p") if p.type == "answer"])
    wiki.ask("p", "What did Alan Turing create?", save=True)
    after = len([p for p in wiki.browse("p") if p.type == "answer"])
    assert before == after == 1
    # the saved answer is itself retrievable
    hits = wiki.search("p", "What did Alan Turing create?", k=10)
    assert any(h.page_id == ans.saved_as for h in hits)


def test_no_result_query_is_explicit(wiki):
    wiki.create_project("empty")
    ans = wiki.ask("empty", "anything at all")
    assert ans.citations == []
    assert "no grounded information" in ans.answer.lower()


def test_determinism_repeated_ask(wiki):
    _seed(wiki)
    a1 = wiki.ask("p", "What did Alan Turing create?")
    a2 = wiki.ask("p", "What did Alan Turing create?")
    assert a1.answer == a2.answer
    assert a1.citations == a2.citations
