from __future__ import annotations

import pytest

from contract_review import (
    JSONFileStore,
    apply_decisions,
    build_report,
    new_session,
    resume,
)
from contract_review.errors import ReviewDecisionError, SessionNotFoundError
from contract_review.models import Clause, ClauseAnalysis, ReviewDecision


def _report():
    clauses = [Clause(number=i, title=f"C{i}", text=f"t{i}") for i in (1, 2, 3)]
    analyses = [
        ClauseAnalysis(clause_number=i, risk="low", category="x", explanation="e", recommendation="r")
        for i in (1, 2, 3)
    ]
    return build_report(clauses, analyses)


def test_store_roundtrip_persists_pending_session(tmp_path):
    store = JSONFileStore(tmp_path)
    session = new_session(_report(), contract_text="body", session_id="sess1", store=store)
    assert session.status == "pending_review"
    assert store.exists("sess1")

    loaded = store.load("sess1")
    assert loaded.id == "sess1"
    assert loaded.status == "pending_review"
    assert loaded.contract_text == "body"
    assert len(loaded.report.clauses) == 3
    assert "sess1" in store.list_ids()


def test_apply_decisions_records_and_persists(tmp_path):
    store = JSONFileStore(tmp_path)
    session = new_session(_report(), session_id="s", store=store)
    apply_decisions(
        session,
        [
            ReviewDecision(clause_number=1, decision="approve"),
            ReviewDecision(clause_number=3, decision="reject"),
            ReviewDecision(clause_number=2, decision="annotate", note="add cap"),
        ],
        store=store,
    )
    reloaded = store.load("s")
    kinds = {d.clause_number: d.decision for d in reloaded.decisions}
    assert kinds == {1: "approve", 2: "annotate", 3: "reject"}
    note = next(d.note for d in reloaded.decisions if d.clause_number == 2)
    assert note == "add cap"


def test_unknown_clause_rejected(tmp_path):
    store = JSONFileStore(tmp_path)
    session = new_session(_report(), session_id="s", store=store)
    with pytest.raises(ReviewDecisionError):
        apply_decisions(session, [ReviewDecision(clause_number=99, decision="approve")])


def test_last_decision_per_clause_wins():
    session = new_session(_report(), session_id="s")
    apply_decisions(session, [ReviewDecision(clause_number=1, decision="approve")])
    apply_decisions(session, [ReviewDecision(clause_number=1, decision="reject")])
    assert len(session.decisions) == 1
    assert session.decisions[0].decision == "reject"


def test_resume_flips_status(tmp_path):
    store = JSONFileStore(tmp_path)
    session = new_session(_report(), session_id="s", store=store)
    apply_decisions(session, [ReviewDecision(clause_number=1, decision="approve")], store=store)
    resume(session, store=store)
    assert session.status == "resumed"
    assert store.load("s").status == "resumed"


def test_resume_before_review_allowed():
    session = new_session(_report(), session_id="s")
    resume(session)  # no decisions recorded — implicitly approved
    assert session.status == "resumed"


def test_resume_when_complete_is_noop():
    session = new_session(_report(), session_id="s")
    session.status = "complete"
    resume(session)
    assert session.status == "complete"


def test_load_missing_session_raises(tmp_path):
    store = JSONFileStore(tmp_path)
    with pytest.raises(SessionNotFoundError):
        store.load("nope")
