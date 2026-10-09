"""Retrieval evaluation against a gold question set.

gold.json:
    {"repo": "flask", "k": 10,
     "questions": [{"q": "how is a url built for an endpoint", "expected": ["url_for"]},
                   {"q": "kubernetes pod autoscaling", "expected": []}]}

`expected` entries match a result's name, qualified name, or key suffix. A
question with no expected answers is a negative: it should return little,
and nothing that is only there because of feedback.

--with-feedback runs on an in-memory copy of the database (your real
feedback table is never touched). It simulates an agent using the tool:
for each question it searches, pages through up to 3*k results, and
"opens" the gold symbols it finds, recording feedback the same way
get_symbol would. It then re-runs every question with feedback boosts on
and reports the before/after metrics, plus how many negative-query results
carry a boost (should be ~0: boosts only reorder documents that were
already retrieved).
"""

from __future__ import annotations

import json
import sqlite3
from dataclasses import dataclass, field
from pathlib import Path

from .memory import record_feedback
from .search import search
from .store import Store


@dataclass
class Metrics:
    questions: int = 0
    answered: int = 0          # positives with >= 1 expected symbol in top k
    recall_sum: float = 0.0    # mean fraction of expected symbols found in top k
    rr_sum: float = 0.0        # reciprocal rank of the first expected symbol
    negatives: int = 0
    negative_results: int = 0  # results returned for negative questions
    negative_boosted: int = 0  # ... of which carried a feedback boost
    per_question: list[dict] = field(default_factory=list)

    @property
    def positives(self):
        return self.questions - self.negatives

    def summary(self) -> dict:
        p = max(1, self.positives)
        return {"questions": self.questions, "hit_rate@k": round(self.answered / p, 3),
                "recall@k": round(self.recall_sum / p, 3), "mrr": round(self.rr_sum / p, 3),
                "negatives": self.negatives, "negative_results": self.negative_results,
                "negative_boosted": self.negative_boosted}


def _matches(result, expected: str) -> bool:
    return (result.name == expected or result.qualname == expected or result.key == expected
            or result.key.endswith("::" + expected) or result.key.endswith("/" + expected))


def run(store: Store, gold: dict, mode: str = "hybrid", use_feedback: bool = False,
        k: int | None = None) -> Metrics:
    repo = gold.get("repo")
    k = k or gold.get("k", 10)
    m = Metrics()
    for item in gold["questions"]:
        expected = item.get("expected", [])
        results = search(store, item["q"], mode=mode, repo=item.get("repo", repo), k=k,
                         use_feedback=use_feedback)
        m.questions += 1
        row = {"q": item["q"], "top": [r.qualname for r in results[:3]]}
        if not expected:
            m.negatives += 1
            m.negative_results += len(results)
            m.negative_boosted += sum(1 for r in results if r.boost > 0)
            row["results"] = len(results)
        else:
            found = [e for e in expected if any(_matches(r, e) for r in results)]
            first = next((i for i, r in enumerate(results, 1) if any(_matches(r, e) for e in expected)), None)
            m.answered += 1 if found else 0
            m.recall_sum += len(found) / len(expected)
            m.rr_sum += 1.0 / first if first else 0.0
            row["rank"] = first
        m.per_question.append(row)
    return m


def simulate_feedback(store: Store, gold: dict, k: int, rounds: int = 2) -> int:
    repo = gold.get("repo")
    events = 0
    for _ in range(rounds):
        for item in gold["questions"]:
            expected = item.get("expected", [])
            if not expected:
                continue
            for r in search(store, item["q"], repo=item.get("repo", repo), k=3 * k, use_feedback=True):
                if r.doc == "symbol" and any(_matches(r, e) for e in expected):
                    record_feedback(store, r.repo, r.key, "eval-open")
                    events += 1
    return events


def evaluate(store: Store, gold_path: str | Path, with_feedback: bool = False, modes=("vector", "text", "hybrid"),
             k: int | None = None) -> dict:
    gold = json.loads(Path(gold_path).read_text(encoding="utf-8"))
    k = k or gold.get("k", 10)
    report = {"k": k, "modes": {}}
    for mode in modes:
        report["modes"][mode] = run(store, gold, mode, use_feedback=False, k=k)
    if with_feedback:
        mem = sqlite3.connect(":memory:")
        store.db.backup(mem)
        shadow = Store.__new__(Store)
        shadow.path = Path(":memory:")
        shadow.db = mem
        mem.row_factory = sqlite3.Row
        shadow.db.execute("DELETE FROM feedback")
        report["feedback_events"] = simulate_feedback(shadow, gold, k)
        report["modes"]["hybrid+feedback"] = run(shadow, gold, "hybrid", use_feedback=True, k=k)
        mem.close()
    return report


def render(report: dict) -> str:
    lines = [f"{'mode':<17} {'hit@k':>6} {'recall@k':>9} {'MRR':>6} {'neg results':>12} {'neg boosted':>12}"]
    for mode, m in report["modes"].items():
        s = m.summary()
        lines.append(f"{mode:<17} {s['hit_rate@k']:>6.3f} {s['recall@k']:>9.3f} {s['mrr']:>6.3f} "
                     f"{s['negative_results']:>12} {s['negative_boosted']:>12}")
    if "feedback_events" in report:
        lines.append(f"(simulated {report['feedback_events']} feedback events on an in-memory copy; k={report['k']})")
    return "\n".join(lines)
