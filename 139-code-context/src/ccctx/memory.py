"""Agent memory: episodic runs, distilled facts, and usage feedback.

* Episodic memory: every tool invocation is a row in `runs`: tool, repo,
  query, arguments, the ranked symbol keys it returned, and timing.
* Usage feedback: when a result is used (opened with get_symbol, cited)
  a `feedback` row is written, linked to the search run that surfaced it.
* Semantic memory: `consolidate` distils runs + feedback into durable
  `facts` (subject, predicate, assertion, confidence, source run ids):

    answers          "<query terms>" is answered by <symbol>   (search -> used)
    frequently_used  <symbol> is consulted often               (opens, graph lookups)
    gap              nothing indexed matches "<query>"         (empty searches)

  Facts citing symbols that no longer exist are dropped on consolidation,
  which runs automatically after every re-index. `answers` facts feed back
  into search as a small bounded boost (see search.fact_boosts).
"""

from __future__ import annotations

import json
import math
import time
from collections import defaultdict

from .embed import terms
from .store import Store, dumps

OPEN_WINDOW_SECONDS = 30 * 60


def subject_of(query: str) -> str:
    return " ".join(sorted(set(t for t in terms(query) if not t.startswith("@"))))


def record_run(store: Store, tool: str, repo: str | None, query: str | None, args: dict,
               results: list[str], duration_ms: float | None = None, ts: float | None = None) -> int:
    cur = store.db.execute(
        "INSERT INTO runs (ts, tool, repo, query, args, results, result_count, duration_ms)"
        " VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
        (ts or time.time(), tool, repo, query, dumps(args), dumps(results[:20]), len(results), duration_ms))
    store.commit()
    return cur.lastrowid


def record_feedback(store: Store, repo: str, symbol_key: str, kind: str = "open",
                    run_id: int | None = None, ts: float | None = None) -> int:
    ts = ts or time.time()
    if run_id is None:
        # Attribute the use to the most recent search that surfaced this symbol.
        row = store.q1("SELECT id FROM runs WHERE tool = 'search_code' AND ts >= ? AND results LIKE ? "
                       "ORDER BY ts DESC LIMIT 1",
                       (ts - OPEN_WINDOW_SECONDS, "%" + json.dumps(symbol_key) + "%"))
        run_id = row[0] if row else None
    cur = store.db.execute("INSERT INTO feedback (ts, repo, symbol_key, kind, run_id) VALUES (?, ?, ?, ?, ?)",
                           (ts, repo, symbol_key, kind, run_id))
    store.commit()
    return cur.lastrowid


def list_runs(store: Store, repo: str | None = None, limit: int = 10) -> list[dict]:
    sql = "SELECT * FROM runs"
    params: list = []
    if repo:
        sql += " WHERE repo = ? OR repo IS NULL"
        params.append(repo)
    sql += " ORDER BY ts DESC, id DESC LIMIT ?"
    params.append(limit)
    out = []
    for r in store.q(sql, params):
        d = dict(r)
        d["args"] = json.loads(d["args"])
        d["results"] = json.loads(d["results"])
        out.append(d)
    return out


def list_facts(store: Store, repo: str | None = None, limit: int = 50) -> list[dict]:
    sql = "SELECT * FROM facts"
    params: list = []
    if repo:
        sql += " WHERE repo = ?"
        params.append(repo)
    sql += " ORDER BY confidence DESC, updated_at DESC LIMIT ?"
    params.append(limit)
    out = []
    for r in store.q(sql, params):
        d = dict(r)
        d["source_runs"] = json.loads(d["source_runs"])
        out.append(d)
    return out


def _upsert_fact(store: Store, repo: str, subject: str, predicate: str, assertion: str,
                 confidence: float, runs: list[int], symbol_key: str | None, now: float) -> None:
    row = store.q1("SELECT id, source_runs FROM facts WHERE repo = ? AND subject = ? AND predicate = ? "
                   "AND symbol_key IS ?", (repo, subject, predicate, symbol_key))
    if row:
        merged = sorted(set(json.loads(row["source_runs"])) | set(runs))
        store.db.execute("UPDATE facts SET assertion = ?, confidence = ?, source_runs = ?, updated_at = ? "
                         "WHERE id = ?", (assertion, round(confidence, 3), dumps(merged), now, row["id"]))
    else:
        store.db.execute(
            "INSERT INTO facts (repo, subject, predicate, assertion, confidence, source_runs, symbol_key,"
            " created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
            (repo, subject, predicate, assertion, round(confidence, 3), dumps(sorted(set(runs))),
             symbol_key, now, now))


def consolidate(store: Store, repo: str, now: float | None = None) -> int:
    """Distil episodic runs and feedback for `repo` into facts. Returns facts written."""
    now = now or time.time()
    repo_row = store.repo(repo)
    if repo_row is None:
        raise KeyError(f"unknown repository '{repo}'")
    live = {r[0]: r for r in store.q(
        "SELECT s.key, s.qualname, f.path FROM symbols s JOIN files f ON f.id = s.file_id "
        "WHERE s.repo_id = ?", (repo_row["id"],))}

    # Staleness: facts about symbols that no longer exist are retired.
    for fid, key in store.q("SELECT id, symbol_key FROM facts WHERE repo = ? AND symbol_key IS NOT NULL",
                            (repo,)):
        if key not in live:
            store.db.execute("DELETE FROM facts WHERE id = ?", (fid,))

    runs = store.q("SELECT id, tool, query, results, result_count, args FROM runs WHERE repo = ? "
                   "ORDER BY ts", (repo,))
    feedback = store.q("SELECT symbol_key, run_id, ts FROM feedback WHERE repo = ?", (repo,))
    used_in_run: dict[int, set[str]] = defaultdict(set)
    use_count: dict[str, list[int]] = defaultdict(list)
    for key, run_id, _ts in feedback:
        if run_id is not None:
            used_in_run[run_id].add(key)
        use_count[key].append(run_id or 0)

    written = 0
    # 1. answers: queries -> the symbols that were used (or kept coming top).
    by_subject: dict[str, list] = defaultdict(list)
    for r in runs:
        if r["tool"] == "search_code" and r["query"]:
            subj = subject_of(r["query"])
            if subj:
                by_subject[subj].append(r)
    for subj, rs in by_subject.items():
        hits: dict[str, int] = defaultdict(int)
        used: dict[str, int] = defaultdict(int)
        run_ids: dict[str, list[int]] = defaultdict(list)
        for r in rs:
            top = json.loads(r["results"])[:3]
            for key in top:
                hits[key] += 1
                run_ids[key].append(r["id"])
            for key in used_in_run.get(r["id"], ()):
                used[key] += 1
                run_ids[key].append(r["id"])
        ranked = sorted(set(hits) | set(used), key=lambda k: (-(used[k] * 2 + hits[k]), k))
        for key in ranked[:2]:
            if key not in live or (used[key] == 0 and hits[key] < 3):
                continue
            conf = min(0.95, 0.3 + 0.1 * hits[key] + 0.25 * used[key])
            qualname, path = live[key][1], live[key][2]
            sample = rs[-1]["query"]
            _upsert_fact(store, repo, subj, "answers",
                         f'"{sample}" is answered by {qualname} ({path})', conf, run_ids[key], key, now)
            written += 1

    # 2. frequently_used symbols: opened/cited, or looked up by graph tools.
    lookups: dict[str, list[int]] = defaultdict(list)
    for r in runs:
        if r["tool"] in ("get_symbol", "find_callers", "impact_of"):
            for key in json.loads(r["results"])[:1]:
                lookups[key].append(r["id"])
    for key in set(use_count) | set(lookups):
        n = len(use_count.get(key, [])) + len(lookups.get(key, []))
        if n < 3 or key not in live:
            continue
        qualname, path = live[key][1], live[key][2]
        _upsert_fact(store, repo, qualname, "frequently_used",
                     f"{qualname} ({path}) is consulted often ({n} uses)",
                     1.0 - math.exp(-n / 5.0), [i for i in use_count.get(key, []) + lookups.get(key, []) if i],
                     key, now)
        written += 1

    # 3. gaps: searches that found nothing.
    for subj, rs in by_subject.items():
        empty = [r for r in rs if r["result_count"] == 0]
        if empty and len(empty) == len(rs):
            _upsert_fact(store, repo, subj, "gap", f'Nothing indexed matches "{empty[-1]["query"]}"',
                         min(0.9, 0.4 + 0.15 * len(empty)), [r["id"] for r in empty], None, now)
            written += 1
        elif not empty:
            store.db.execute("DELETE FROM facts WHERE repo = ? AND subject = ? AND predicate = 'gap'",
                             (repo, subj))
    store.commit()
    return written


def recall(store: Store, query: str, repo: str | None = None, limit: int = 5) -> dict:
    """Facts relevant to `query` plus recent runs, for an agent to start from."""
    qterms = set(subject_of(query).split())
    facts = []
    for f in list_facts(store, repo, limit=500):
        fterms = set(f["subject"].split()) | set(t for t in terms(f["assertion"]) if not t.startswith("@"))
        overlap = len(qterms & fterms) / max(1, len(qterms)) if qterms else 0.0
        if overlap > 0:
            f["relevance"] = round(overlap * f["confidence"], 3)
            facts.append(f)
    facts.sort(key=lambda f: -f["relevance"])
    runs = list_runs(store, repo, limit=limit)
    return {"facts": facts[:limit], "recent_runs": runs}
