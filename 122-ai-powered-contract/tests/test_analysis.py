from __future__ import annotations

import re

from contract_review import Config, MockLLM, analyse_all, analyse_clause
from contract_review.models import Clause
from tests.conftest import analysis_json

_NUM = re.compile(r"clause number (\d+)")


def _router_by_number(risk_map):
    def router(system, messages, intent):
        content = messages[0]["content"]
        m = _NUM.search(content)
        n = int(m.group(1)) if m else 0
        value = risk_map.get(n, "low")
        if value == "MALFORMED":
            return "this is not json at all"
        return analysis_json(n, risk=value, category="liability" if value == "high" else "general")

    return router


def test_analyse_clause_single():
    llm = MockLLM(router=_router_by_number({1: "high"}))
    clause = Clause(number=1, title="Indemnity", text="indemnify fully")
    analysis = analyse_clause(clause, llm)
    assert analysis.clause_number == 1
    assert analysis.risk == "high"


def test_analyse_all_risks_and_order(sample_clauses):
    llm = MockLLM(router=_router_by_number({1: "low", 2: "high", 3: "medium"}))
    analyses = analyse_all(sample_clauses, llm)
    assert [a.clause_number for a in analyses] == [1, 2, 3]
    assert [a.risk for a in analyses] == ["low", "high", "medium"]


def test_malformed_clause_degrades_to_placeholder(sample_clauses):
    llm = MockLLM(router=_router_by_number({1: "low", 2: "MALFORMED", 3: "low"}))
    analyses = analyse_all(sample_clauses, llm)
    assert analyses[1].risk == "high"
    assert analyses[1].category == "unanalysed"
    # The run completes for every clause despite one bad response.
    assert len(analyses) == 3


def test_ordering_preserved_under_concurrency():
    clauses = [Clause(number=i, title=f"C{i}", text=f"text {i}") for i in range(1, 13)]
    config = Config(provider="mock", analysis_workers=8)
    llm = MockLLM(router=_router_by_number({i: "low" for i in range(1, 13)}))
    analyses = analyse_all(clauses, llm, config)
    assert [a.clause_number for a in analyses] == list(range(1, 13))
