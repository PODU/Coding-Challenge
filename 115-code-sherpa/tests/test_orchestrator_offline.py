"""End-to-end offline smoke test + multi-project isolation."""

from __future__ import annotations


def test_ingest_then_ask_end_to_end(make_sherpa, fixture_repo):
    sherpa = make_sherpa()  # default MockLLMClient (synthesises from prompt)
    report = sherpa.ingest("demo", str(fixture_repo))
    assert report.total_chunks > 0

    answer = sherpa.ask("demo", "where do we handle payment failures?")
    assert not answer.insufficient_context
    assert answer.citations
    # default mock echoes the first cited [path:line] marker back.
    assert any("payments.py" in c.path for c in answer.citations)
    assert "payments.py" in answer.text


def test_multi_project_isolation_through_ask(make_sherpa, fixture_repo):
    sherpa = make_sherpa()
    sherpa.ingest("alpha", str(fixture_repo))

    # Second project: only an auth file in a fresh dir.
    other = fixture_repo.parent / "other"
    (other / "src").mkdir(parents=True)
    (other / "src" / "only_auth.py").write_text(
        "def login(u, p):\n    return True\n", encoding="utf-8"
    )
    sherpa.ingest("beta", str(other))

    a = sherpa.ask("alpha", "refund payment failure")
    assert all("only_auth.py" not in c.path for c in a.citations)

    b = sherpa.ask("beta", "login user")
    assert b.citations
    assert all("payments.py" not in c.path for c in b.citations)


def test_projects_registered(make_sherpa, fixture_repo):
    sherpa = make_sherpa()
    sherpa.ingest("demo", str(fixture_repo))
    names = [p.name for p in sherpa.registry.list()]
    assert "demo" in names
    info = sherpa.registry.get("demo")
    assert info.file_count >= 3
    assert info.chunk_count == sherpa.store.count("demo")


def test_empty_question_handled(make_sherpa, fixture_repo):
    sherpa = make_sherpa()
    sherpa.ingest("demo", str(fixture_repo))
    answer = sherpa.ask("demo", "   ")
    assert answer.insufficient_context is True
