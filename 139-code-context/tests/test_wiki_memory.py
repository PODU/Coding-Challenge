import json

from ccctx import memory, tools, wiki
from ccctx.ingest import ingest
from ccctx.search import search


def test_only_modules_with_ten_symbols_get_pages(indexed):
    res = wiki.build(indexed, "sample")
    assert res["built"] == ["app"]  # app/ has >= 10 symbols; root, web/ and tests/ do not
    page = wiki.find_page(indexed, "app")
    assert page["commit_sha"] == indexed.repo("sample")["commit_sha"]
    body = page["body"]
    assert "## Purpose" in body and "Sample application package." in body
    assert "## Key symbols" in body and "`Router`" in body
    assert "## Dependencies" in body


def test_citations_are_foreign_keys(indexed):
    wiki.build(indexed, "sample")
    page = wiki.find_page(indexed, "app")
    cites = wiki.citations(indexed, page)
    assert cites and all(c["status"] == "fresh" and c["symbol_id"] for c in cites)
    ids = [c["symbol_id"] for c in cites]
    marks = ",".join("?" * len(ids))
    assert indexed.q1(f"SELECT COUNT(*) FROM symbols WHERE id IN ({marks})", ids)[0] == len(ids)
    # sample/ and sample/app are both reachable by "<repo>/<slug>" too
    assert wiki.find_page(indexed, "sample/app")["id"] == page["id"]


def test_staleness_and_selective_rebuild(indexed, sample):
    wiki.build(indexed, "sample")
    page = wiki.find_page(indexed, "app")
    assert not wiki.page_status(indexed, page).stale
    cited = {c["symbol_key"] for c in wiki.citations(indexed, page)}
    assert "app/routing.py::Router" in cited
    text = (sample / "app/routing.py").read_text()
    (sample / "app/routing.py").write_text(text.replace("Routes incoming requests", "Sends requests"))
    ingest(indexed, "sample")
    st = wiki.page_status(indexed, wiki.find_page(indexed, "app"))
    assert st.stale and any("changed: app/routing.py::Router" in r for r in st.reasons)
    statuses = {c["symbol_key"]: c["status"] for c in wiki.citations(indexed, wiki.find_page(indexed, "app"))}
    assert statuses["app/routing.py::Router"] == "changed"
    res = wiki.build(indexed, "sample")
    assert res["built"] == ["app"]
    assert not wiki.page_status(indexed, wiki.find_page(indexed, "app")).stale
    assert wiki.build(indexed, "sample")["fresh"] == ["app"]  # nothing to do the second time


def test_deleted_symbol_sets_citation_null(indexed, sample):
    wiki.build(indexed, "sample")
    (sample / "app/storage.py").unlink()
    ingest(indexed, "sample")
    page = wiki.find_page(indexed, "app")
    statuses = [c["status"] for c in wiki.citations(indexed, page)]
    assert "missing" in statuses or wiki.page_status(indexed, page).stale


def test_wiki_pages_appear_in_hybrid_search(indexed):
    wiki.build(indexed, "sample")
    res = search(indexed, "what does the app module do")
    assert any(r.doc == "wiki" and r.key == "app" for r in res)
    data = tools.read_wiki(indexed, "app")
    assert data["title"] == "app" and data["citations"]
    assert tools.read_wiki(indexed)["pages"][0]["slug"] == "app"
    assert "error" in tools.read_wiki(indexed, "nope")


def test_runs_are_recorded_for_every_tool(indexed):
    tools.search_code(indexed, "router dispatch", repo="sample")
    tools.get_symbol(indexed, "dispatch", repo="sample")
    tools.find_callers(indexed, "make_response", repo="sample")
    tools.impact_of(indexed, "make_response", repo="sample")
    tools.recall(indexed, "router", repo="sample")
    runs = memory.list_runs(indexed, "sample", limit=10)
    assert [r["tool"] for r in runs][::-1] == ["search_code", "get_symbol", "find_callers", "impact_of", "recall"]
    search_run = runs[-1]
    assert search_run["query"] == "router dispatch" and search_run["results"]
    assert search_run["args"]["mode"] == "hybrid"


def test_get_symbol_records_feedback_linked_to_search(indexed):
    out = tools.search_code(indexed, "router dispatch", repo="sample")
    key = "app/routing.py::Router.dispatch"
    assert key in [r["key"] for r in out["results"]]
    tools.get_symbol(indexed, "Router.dispatch", repo="sample")
    fb = indexed.q1("SELECT * FROM feedback WHERE symbol_key = ?", (key,))
    assert fb["run_id"] == out["run_id"]


def test_consolidate_distils_facts(indexed):
    for _ in range(3):
        tools.search_code(indexed, "router dispatch", repo="sample")
        tools.get_symbol(indexed, "Router.dispatch", repo="sample")
    tools.search_code(indexed, "quantum teleportation", repo="sample")
    n = memory.consolidate(indexed, "sample")
    facts = memory.list_facts(indexed, "sample")
    preds = {f["predicate"] for f in facts}
    assert n >= 3 and {"answers", "frequently_used", "gap"} <= preds
    ans = next(f for f in facts if f["predicate"] == "answers")
    assert ans["symbol_key"] == "app/routing.py::Router.dispatch"
    assert 0 < ans["confidence"] <= 0.95 and ans["source_runs"]
    # idempotent: re-consolidating updates rather than duplicates
    memory.consolidate(indexed, "sample")
    assert len(memory.list_facts(indexed, "sample")) == len(facts)
    rec = memory.recall(indexed, "router dispatch", "sample")
    assert rec["facts"] and rec["facts"][0]["predicate"] in ("answers", "frequently_used")
    assert rec["recent_runs"]


def test_facts_boost_later_searches(indexed):
    base = search(indexed, "router dispatch", repo="sample", use_feedback=False)
    for _ in range(3):
        tools.search_code(indexed, "router dispatch", repo="sample")
        tools.get_symbol(indexed, "Router.dispatch", repo="sample")
    memory.consolidate(indexed, "sample")
    after = search(indexed, "dispatch router", repo="sample")
    hit = next(r for r in after if r.key == "app/routing.py::Router.dispatch")
    assert hit.boost > 0
    assert base  # baseline had results too


def test_reindex_consolidates_and_retires_stale_facts(indexed, sample):
    for _ in range(3):
        tools.get_symbol(indexed, "save_user", repo="sample")
    memory.consolidate(indexed, "sample")
    assert any(f["symbol_key"] == "app/storage.py::save_user" for f in memory.list_facts(indexed, "sample"))
    (sample / "app/storage.py").unlink()
    ingest(indexed, "sample")  # consolidation runs automatically after a re-index
    assert not any(f["symbol_key"] == "app/storage.py::save_user" for f in memory.list_facts(indexed, "sample"))


def test_run_results_are_json(indexed):
    tools.search_code(indexed, "router", repo="sample")
    raw = indexed.q1("SELECT results, args FROM runs")
    assert isinstance(json.loads(raw[0]), list) and isinstance(json.loads(raw[1]), dict)
