from __future__ import annotations

from llmwiki import Frontmatter, Operation, WikiPage


def _upsert(wiki, project, page):
    page.recompute_sha()
    vec = wiki.embedder.embed([page.embedding_text()])[0]
    wiki.store.upsert_page(project, page, vec)


def _page(pid, type, project="p", links=None, status="active", body="body"):
    fm = Frontmatter(id=pid, type=type, title=pid.split("/")[-1], project=project,
                     links=links or [], status=status)
    return WikiPage(frontmatter=fm, body=body)


def test_orphan_detection(wiki):
    wiki.create_project("p")
    # hub links to a, but b is an orphan entity
    _upsert(wiki, "p", _page("summaries/hub", "summary", links=["entities/a"]))
    _upsert(wiki, "p", _page("entities/a", "entity"))
    _upsert(wiki, "p", _page("entities/b", "entity"))
    findings = wiki.lint("p")
    orphans = {f.page_id for f in findings if f.kind == "orphan"}
    assert "entities/b" in orphans
    assert "entities/a" not in orphans


def test_missing_page_detection_and_fix(wiki):
    wiki.create_project("p")
    _upsert(wiki, "p", _page("summaries/s", "summary", links=["entities/ghost"]))
    findings = wiki.lint("p")
    missing = [f for f in findings if f.kind == "missing_page"]
    assert any(f.target == "entities/ghost" for f in missing)

    actions = wiki.lint_fix("p")
    assert any("entities/ghost" in a for a in actions)
    # stub now exists; missing finding gone; fix is idempotent
    assert wiki.get_page("p", "entities/ghost") is not None
    findings2 = wiki.lint("p")
    assert not any(f.kind == "missing_page" and f.target == "entities/ghost" for f in findings2)
    assert wiki.lint_fix("p") == [] or all("entities/ghost" not in a for a in wiki.lint_fix("p"))


def test_broken_link_after_delete(wiki):
    wiki.create_project("p")
    _upsert(wiki, "p", _page("summaries/s", "summary", links=["entities/gone"]))
    _upsert(wiki, "p", _page("entities/gone", "entity"))
    # delete the target and log the deletion
    wiki.store.delete_page("p", "entities/gone")
    wiki.store.append_log("p", Operation(op="delete_page", page_id="entities/gone"))
    findings = wiki.lint("p")
    broken = [f for f in findings if f.kind == "broken_link"]
    assert any(f.target == "entities/gone" for f in broken)


def test_stale_detection(wiki):
    wiki.create_project("p")
    _upsert(wiki, "p", _page("summaries/old", "summary",
                             links=["x"], status="stale"))
    findings = wiki.lint("p")
    assert any(f.kind == "stale" and f.page_id == "summaries/old" for f in findings)


def test_data_gap(wiki):
    wiki.create_project("p")
    # summary references an entity page that doesn't exist -> gap + missing
    _upsert(wiki, "p", _page("summaries/s", "summary", links=["entities/nobody"]))
    findings = wiki.lint("p")
    kinds = {(f.kind, f.target) for f in findings}
    assert ("data_gap", "entities/nobody") in kinds
