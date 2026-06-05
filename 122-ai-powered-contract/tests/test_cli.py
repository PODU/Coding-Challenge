from __future__ import annotations

import io
import re

from contract_review import JSONFileStore, MockLLM
from contract_review.cli import main
from tests.conftest import analysis_json, extract_json, qa_json, summary_json

_NUM = re.compile(r"clause number (\d+)")

CLAUSES = [
    (1, "Term", "This Agreement runs for one year from the Effective Date."),
    (2, "Termination", "Either party may terminate with thirty (30) days written notice."),
    (3, "Liability", "No party is liable for indirect or consequential damages."),
]


def _router(system, messages, intent):
    content = messages[0]["content"]
    if intent == "extract":
        return extract_json(CLAUSES)
    if intent == "analyse":
        m = _NUM.search(content)
        n = int(m.group(1)) if m else 1
        risk = "high" if n == 3 else "low"
        return analysis_json(n, risk=risk)
    if intent == "summary":
        return summary_json("Contract reviewed; one high-risk clause.", ["termination"], ["liability"])
    if intent == "qa":
        return qa_json("Thirty days written notice.", [2])
    return "{}"


def _llm():
    return MockLLM(router=_router)


def _write_contract(tmp_path):
    p = tmp_path / "contract.txt"
    body = "SERVICES AGREEMENT\n\n" + "\n\n".join(
        f"{n}. {title}\n{text}" for n, title, text in CLAUSES
    )
    p.write_text(body, encoding="utf-8")
    return p


def test_end_to_end_analyse_review_resume_summary_ask(tmp_path):
    store = JSONFileStore(tmp_path / "sessions")
    contract = _write_contract(tmp_path)
    llm = _llm()

    # 1. analyse (pauses for review)
    out = io.StringIO()
    rc = main(["analyse", str(contract), "--session-id", "cli1"], llm=llm, store=store, out=out)
    assert rc == 0
    session = store.load("cli1")
    assert session.status == "pending_review"
    assert len(session.report.clauses) == 3
    assert session.report.overall_risk == "high"
    assert "Paused for human review" in out.getvalue()

    # 2. review (approve/annotate)
    out = io.StringIO()
    rc = main(
        ["review", "--session", "cli1", "--approve", "1,2", "--annotate", "3=add a cap"],
        llm=llm,
        store=store,
        out=out,
    )
    assert rc == 0
    session = store.load("cli1")
    assert {d.clause_number for d in session.decisions} == {1, 2, 3}

    # 3. resume (triggers summary)
    out = io.StringIO()
    rc = main(["resume", "--session", "cli1"], llm=llm, store=store, out=out)
    assert rc == 0
    session = store.load("cli1")
    assert session.status == "complete"
    assert session.summary is not None
    assert "Review Summary" in out.getvalue()

    # 4. summary (re-display)
    out = io.StringIO()
    rc = main(["summary", "--session", "cli1"], llm=llm, store=store, out=out)
    assert rc == 0
    assert "liability" in out.getvalue()

    # 5. ask (grounded Q&A)
    out = io.StringIO()
    rc = main(
        ["ask", "--session", "cli1", "What is the termination notice period?"],
        llm=llm,
        store=store,
        out=out,
    )
    assert rc == 0
    assert "Thirty days" in out.getvalue()
    assert "Grounded: True" in out.getvalue()

    # Every model interaction went through the injected mock — no network.
    assert llm.calls


def test_analyse_no_pause_completes(tmp_path):
    store = JSONFileStore(tmp_path / "sessions")
    contract = _write_contract(tmp_path)
    out = io.StringIO()
    rc = main(["analyse", str(contract), "--no-pause", "--session-id", "np"], llm=_llm(), store=store, out=out)
    assert rc == 0
    assert store.load("np").status == "complete"
    assert "Review Summary" in out.getvalue()


def test_analyse_malformed_document_returns_error(tmp_path):
    store = JSONFileStore(tmp_path / "sessions")
    rc = main(["analyse", str(tmp_path / "missing.txt")], llm=_llm(), store=store, out=io.StringIO())
    assert rc == 2
