"""Search: vector (cosine), text (FTS5/BM25) and hybrid (Reciprocal Rank Fusion).

Hybrid mirrors DBMS_HYBRID_VECTOR.SEARCH with RRF scoring and UNION fusion:
each retriever produces a ranked list, a document's fused score is
sum(weight / (60 + rank)) over the lists it appears in, and documents found
by either retriever are kept. A third list, exact identifier definitions,
is fused in so that `url_for` the function outranks the dozens of places
that call it. Feedback and memory boosts are applied last, multiplicatively
and capped at +20%, and only to documents already retrieved: a popular
symbol can climb within a relevant result list but can never be injected
into an unrelated one.
"""

from __future__ import annotations

import math
import re
import time
from dataclasses import asdict, dataclass

import numpy as np

from .embed import STOPWORDS, from_blob, get_embedder, split_identifier, stem, terms
from .store import Store

RRF_K = 60
MIN_SIMILARITY = 0.3
TEXT_ONLY_FLOOR = 0.15  # hybrid: a keyword-only hit must still be at least this similar
CANDIDATES = 50
MAX_BOOST = 0.20
HALF_LIFE_DAYS = 30.0
TEST_PENALTY = 0.8  # tests mention everything; prefer implementation unless asked

_TEST_PATH = re.compile(r"(^|/)(tests?|testing|__tests__|spec)/|(^|/)test_[^/]*$|_test\.\w+$|\.(test|spec)\.\w+$")
_TEST_QUERY = re.compile(r"\btest|\bspec\b|\bfixture", re.I)


def is_test_path(path: str) -> bool:
    return bool(_TEST_PATH.search(path))


@dataclass
class Result:
    doc: str  # "symbol" | "wiki"
    id: int
    key: str
    repo: str
    path: str
    name: str
    qualname: str
    kind: str
    language: str
    start_line: int
    end_line: int
    score: float
    similarity: float | None = None
    text_rank: int | None = None
    vector_rank: int | None = None
    definition: bool = False
    boost: float = 0.0
    signature: str = ""
    snippet: str = ""

    def to_dict(self) -> dict:
        return asdict(self)

    @property
    def location(self) -> str:
        if self.doc == "wiki":
            return f"wiki:{self.key}"
        return f"{self.repo}/{self.path}:{self.start_line}-{self.end_line}"


def _repo_filter(store: Store, repo: str | None) -> list[int]:
    if repo:
        r = store.repo(repo)
        if r is None:
            raise KeyError(f"unknown repository '{repo}'")
        return [r["id"]]
    return [r["id"] for r in store.q("SELECT id FROM repos")]


_MATRIX_CACHE: dict = {}


def _matrix(store: Store, table: str, repo_ids: list[int], language: str | None):
    gen = store.q1("SELECT COALESCE(SUM(indexed_at), 0), COUNT(*) FROM repos")
    wiki_gen = store.q1("SELECT COALESCE(MAX(built_at), 0), COUNT(*) FROM wiki_pages")
    cache_key = (str(store.path), table, tuple(repo_ids), language, tuple(gen), tuple(wiki_gen))
    hit = _MATRIX_CACHE.get(cache_key)
    if hit is not None:
        return hit
    marks = ",".join("?" * len(repo_ids)) or "NULL"
    sql = f"SELECT id, embedding FROM {table} WHERE embedding IS NOT NULL AND repo_id IN ({marks})"
    params: list = list(repo_ids)
    if language and table == "symbols":
        sql += " AND language = ?"
        params.append(language)
    rows = store.q(sql, params)
    ids = np.array([r[0] for r in rows], dtype=np.int64)
    mat = np.stack([from_blob(r[1]) for r in rows]) if rows else np.zeros((0, 1), dtype=np.float32)
    if len(_MATRIX_CACHE) > 16:
        _MATRIX_CACHE.clear()
    _MATRIX_CACHE[cache_key] = (ids, mat)
    return ids, mat


def similarities(store: Store, qvec: np.ndarray, table: str, repo_ids, language, ids) -> dict[int, float]:
    all_ids, mat = _matrix(store, table, repo_ids, language)
    if not len(all_ids) or not ids:
        return {}
    pos = {int(i): n for n, i in enumerate(all_ids)}
    rows = [pos[i] for i in ids if i in pos]
    sims = mat[rows] @ qvec
    return {int(all_ids[r]): float(v) for r, v in zip(rows, sims)}


def vector_candidates(store: Store, qvec: np.ndarray, table: str, repo_ids, language,
                      limit: int = CANDIDATES, min_sim: float = MIN_SIMILARITY) -> list[tuple[int, float]]:
    ids, mat = _matrix(store, table, repo_ids, language)
    if not len(ids):
        return []
    sims = mat @ qvec
    order = np.argsort(-sims)[:limit]
    return [(int(ids[i]), float(sims[i])) for i in order if sims[i] >= min_sim]


_TOKEN = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def fts_query(query: str) -> str:
    """Build an FTS5 OR-query: exact identifiers, their parts, and stem prefixes."""
    parts: dict[str, None] = {}
    for tok in _TOKEN.findall(query):
        low = tok.lower()
        if low not in STOPWORDS and len(low) > 1:
            parts.setdefault(f'"{tok}"')
        for p in split_identifier(tok):
            if p in STOPWORDS or len(p) < 2:
                continue
            parts.setdefault(f'"{p}"')
            st = stem(p)
            if len(st) >= 4 and st != p:
                parts.setdefault(f'"{st}"*')
    return " OR ".join(parts)


def text_candidates(store: Store, query: str, repo_ids, language, limit: int = CANDIDATES,
                    wiki: bool = False) -> list[int]:
    match = fts_query(query)
    if not match:
        return []
    marks = ",".join("?" * len(repo_ids)) or "NULL"
    if wiki:
        sql = (f"SELECT p.id FROM wiki_fts f JOIN wiki_pages p ON p.id = f.rowid "
               f"WHERE wiki_fts MATCH ? AND p.repo_id IN ({marks}) "
               f"ORDER BY bm25(wiki_fts, 5.0, 1.0) LIMIT ?")
        return [r[0] for r in store.q(sql, [match, *repo_ids, limit])]
    sql = (f"SELECT s.id FROM symbols_fts f JOIN symbols s ON s.id = f.rowid "
           f"WHERE symbols_fts MATCH ? AND s.repo_id IN ({marks})")
    params: list = [match, *repo_ids]
    if language:
        sql += " AND s.language = ?"
        params.append(language)
    sql += " ORDER BY bm25(symbols_fts, 10.0, 6.0, 4.0, 2.0, 1.0) LIMIT ?"
    params.append(limit)
    return [r[0] for r in store.q(sql, params)]


_IDENTISH = re.compile(r"_|\.|[a-z][A-Z]|^[A-Z][a-z]+[A-Z]|\(\)$")


def identifier_tokens(query: str) -> list[str]:
    """Tokens that look like code identifiers (snake_case, camelCase, dotted, Name()).

    A bare English word only counts when the whole query is one or two words,
    so "url_for" or "Engine" find definitions but "build a url for an endpoint"
    does not treat "endpoint" as an identifier lookup.
    """
    found = re.findall(r"[A-Za-z_][\w.]*(?:\(\))?", query)
    clean = [(t, t.rstrip("()").strip(".")) for t in found]
    clean = [(t, c) for t, c in clean if len(c) > 1]
    if len(clean) <= 2:
        return [c for _, c in clean]
    return [c for t, c in clean if _IDENTISH.search(t)]


def definition_candidates(store: Store, query: str, repo_ids, language) -> list[int]:
    """Symbols whose name or qualified name is spelled exactly in the query."""
    tokens = identifier_tokens(query)
    if not tokens:
        return []
    marks = ",".join("?" * len(repo_ids)) or "NULL"
    tmarks = ",".join("?" * len(tokens))
    sql = (f"SELECT id FROM symbols WHERE repo_id IN ({marks}) "
           f"AND (name IN ({tmarks}) OR qualname IN ({tmarks})) AND kind != 'chunk'")
    params: list = [*repo_ids, *tokens, *tokens]
    if language:
        sql += " AND language = ?"
        params.append(language)
    sql += (" ORDER BY CASE kind WHEN 'function' THEN 0 WHEN 'class' THEN 0 WHEN 'struct' THEN 0"
            " WHEN 'method' THEN 1 ELSE 2 END, length(qualname)")
    return [r[0] for r in store.q(sql, params)]


def wiki_name_candidates(store: Store, query: str, repo_ids) -> list[int]:
    """Wiki pages whose module name is spelled in the query ("the json module")."""
    words = {w.lower() for w in re.findall(r"[A-Za-z_][\w]*", query)}
    if not words:
        return []
    marks = ",".join("?" * len(repo_ids)) or "NULL"
    out = []
    for pid, slug in store.q(f"SELECT id, slug FROM wiki_pages WHERE repo_id IN ({marks}) "
                             f"ORDER BY length(slug)", repo_ids):
        if slug.rsplit("/", 1)[-1].lower() in words:
            out.append(pid)
    return out


def feedback_weights(store: Store, keys: list[str], now: float | None = None) -> dict[str, float]:
    """Decayed usage weight per symbol key: sum of 0.5 ** (age_days / 30)."""
    if not keys:
        return {}
    now = now or time.time()
    out: dict[str, float] = {}
    for i in range(0, len(keys), 500):
        chunk = keys[i:i + 500]
        marks = ",".join("?" * len(chunk))
        for key, ts in store.q(f"SELECT symbol_key, ts FROM feedback WHERE symbol_key IN ({marks})", chunk):
            age_days = max(0.0, (now - ts) / 86400.0)
            out[key] = out.get(key, 0.0) + 0.5 ** (age_days / HALF_LIFE_DAYS)
    return out


def boost_from_weight(weight: float) -> float:
    """Saturating map from usage weight to a boost in [0, MAX_BOOST)."""
    return MAX_BOOST * (1.0 - math.exp(-weight / 3.0))


def fact_boosts(store: Store, query: str, repos: list[str]) -> dict[str, float]:
    """Facts of the form 'query terms -> symbol answers it' nudge that symbol."""
    qterms = set(t for t in terms(query) if not t.startswith("@"))
    if not qterms or not repos:
        return {}
    marks = ",".join("?" * len(repos))
    out: dict[str, float] = {}
    for subject, key, conf in store.q(
            f"SELECT subject, symbol_key, confidence FROM facts WHERE predicate = 'answers' "
            f"AND repo IN ({marks}) AND symbol_key IS NOT NULL", repos):
        sterms = set(subject.split())
        if not sterms:
            continue
        overlap = len(qterms & sterms) / len(qterms | sterms)
        if overlap >= 0.5:
            out[key] = max(out.get(key, 0.0), 0.10 * conf * overlap)
    return out


def search(store: Store, query: str, mode: str = "hybrid", repo: str | None = None,
           language: str | None = None, k: int = 10, min_sim: float = MIN_SIMILARITY,
           use_feedback: bool = True, include_wiki: bool = True, embedder=None,
           now: float | None = None) -> list[Result]:
    if mode not in ("hybrid", "vector", "text"):
        raise ValueError("mode must be hybrid, vector or text")
    repo_ids = _repo_filter(store, repo)
    if not repo_ids or not query.strip():
        return []
    wiki_ok = include_wiki and not language

    scores: dict[tuple[str, int], float] = {}
    sim_of: dict[tuple[str, int], float] = {}
    vrank: dict[tuple[str, int], int] = {}
    trank: dict[tuple[str, int], int] = {}
    defs: set[tuple[str, int]] = set()

    if mode in ("vector", "hybrid"):
        embedder = embedder or get_embedder(_repo_embedder(store, repo_ids))
        qvec = embedder.embed([query])[0]
        vec = [(("symbol", i), s) for i, s in vector_candidates(store, qvec, "symbols", repo_ids, language,
                                                               min_sim=min_sim)]
        if wiki_ok:
            vec += [(("wiki", i), s) for i, s in vector_candidates(store, qvec, "wiki_pages", repo_ids, None,
                                                                  min_sim=min_sim)]
            vec.sort(key=lambda x: -x[1])
        for rank, (doc, sim) in enumerate(vec, 1):
            sim_of[doc] = sim
            vrank[doc] = rank
            scores[doc] = scores.get(doc, 0.0) + (sim if mode == "vector" else 1.0 / (RRF_K + rank))

    if mode in ("text", "hybrid"):
        sym_txt = text_candidates(store, query, repo_ids, language)
        wiki_txt = text_candidates(store, query, repo_ids, None, wiki=True) if wiki_ok else []
        if mode == "hybrid":
            # UNION fusion, but a document found only by keywords must not be semantically unrelated.
            ssim = similarities(store, qvec, "symbols", repo_ids, language, sym_txt)
            wsim = similarities(store, qvec, "wiki_pages", repo_ids, None, wiki_txt)
            sym_txt = [i for i in sym_txt if ("symbol", i) in sim_of or ssim.get(i, 0.0) >= TEXT_ONLY_FLOOR]
            wiki_txt = [i for i in wiki_txt if ("wiki", i) in sim_of or wsim.get(i, 0.0) >= TEXT_ONLY_FLOOR]
        txt = [("symbol", i) for i in sym_txt]
        if wiki_ok:
            # Interleave wiki pages by their own BM25 order.
            wiki = [("wiki", i) for i in wiki_txt]
            merged = []
            for i in range(max(len(txt), len(wiki))):
                if i < len(wiki):
                    merged.append(wiki[i])
                if i < len(txt):
                    merged.append(txt[i])
            txt = merged if wiki else txt
        for rank, doc in enumerate(txt, 1):
            trank[doc] = rank
            scores[doc] = scores.get(doc, 0.0) + 1.0 / (RRF_K + rank)
        for rank, sid in enumerate(definition_candidates(store, query, repo_ids, language), 1):
            doc = ("symbol", sid)
            defs.add(doc)
            scores[doc] = scores.get(doc, 0.0) + 2.0 / (RRF_K + rank)
        if wiki_ok:
            for rank, pid in enumerate(wiki_name_candidates(store, query, repo_ids), 1):
                doc = ("wiki", pid)
                defs.add(doc)
                scores[doc] = scores.get(doc, 0.0) + 2.0 / (RRF_K + rank)

    if not scores:
        return []
    results = _materialise(store, scores, sim_of, vrank, trank, defs)
    if not _TEST_QUERY.search(query):
        for r in results:
            if is_test_path(r.path):
                r.score *= TEST_PENALTY
    if use_feedback:
        repo_names = sorted({r.repo for r in results})
        fb = feedback_weights(store, [r.key for r in results if r.doc == "symbol"], now)
        facts = fact_boosts(store, query, repo_names)
        for r in results:
            b = boost_from_weight(fb.get(r.key, 0.0)) + facts.get(r.key, 0.0)
            r.boost = min(MAX_BOOST, b)
            r.score *= 1.0 + r.boost
    results.sort(key=lambda r: (-r.score, r.key))
    return results[:k]


def _repo_embedder(store: Store, repo_ids) -> str | None:
    marks = ",".join("?" * len(repo_ids))
    row = store.q1(f"SELECT embedder FROM repos WHERE id IN ({marks}) AND embedder IS NOT NULL", repo_ids)
    return row[0] if row else None


def _snippet(source: str, n: int = 3) -> str:
    lines = [l for l in source.splitlines() if l.strip()]
    return "\n".join(lines[:n])


def _materialise(store, scores, sim_of, vrank, trank, defs) -> list[Result]:
    out = []
    sym_ids = [i for d, i in scores if d == "symbol"]
    wiki_ids = [i for d, i in scores if d == "wiki"]
    for i in range(0, len(sym_ids), 500):
        chunk = sym_ids[i:i + 500]
        marks = ",".join("?" * len(chunk))
        for r in store.q(
                f"SELECT s.*, f.path, rp.name AS repo_name FROM symbols s JOIN files f ON f.id = s.file_id "
                f"JOIN repos rp ON rp.id = s.repo_id WHERE s.id IN ({marks})", chunk):
            doc = ("symbol", r["id"])
            out.append(Result("symbol", r["id"], r["key"], r["repo_name"], r["path"], r["name"],
                              r["qualname"], r["kind"], r["language"], r["start_line"], r["end_line"],
                              scores[doc], sim_of.get(doc), trank.get(doc), vrank.get(doc), doc in defs,
                              signature=r["signature"], snippet=_snippet(r["source"])))
    if wiki_ids:
        marks = ",".join("?" * len(wiki_ids))
        for r in store.q(f"SELECT p.*, rp.name AS repo_name FROM wiki_pages p JOIN repos rp "
                         f"ON rp.id = p.repo_id WHERE p.id IN ({marks})", wiki_ids):
            doc = ("wiki", r["id"])
            out.append(Result("wiki", r["id"], r["slug"], r["repo_name"], f"wiki/{r['slug']}", r["title"],
                              r["slug"], "wiki", "markdown", 1, r["body"].count("\n") + 1, scores[doc],
                              sim_of.get(doc), trank.get(doc), vrank.get(doc),
                              snippet=_snippet(r["body"], 4)))
    return out
