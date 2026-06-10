"""The default store: one SQLite database, every table keyed by project.

Vectors are float32 blobs ranked by cosine in Python, which is fine at
personal-wiki scale. Full-text search uses FTS5 when SQLite has it and a plain
token-overlap scan otherwise.
"""

from __future__ import annotations

import json
import sqlite3
import struct
from datetime import datetime, timezone

import numpy as np

from ..domain import (
    Contradiction,
    EmbeddingDimMismatch,
    Frontmatter,
    Hit,
    Operation,
    WikiPage,
)
from ..fusion import reciprocal_rank_fusion
from ..text import tokenize


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _pack(vec: list[float]) -> bytes:
    return struct.pack(f"<{len(vec)}f", *vec)


def _unpack(blob: bytes) -> np.ndarray:
    return np.frombuffer(blob, dtype="<f4")


class SqliteStore:
    def __init__(self, path: str = ":memory:") -> None:
        self.path = path
        # Lets the one connection (and its :memory: db) be used from other threads.
        self._db = sqlite3.connect(path, check_same_thread=False)
        self._db.row_factory = sqlite3.Row
        self._db.execute("PRAGMA journal_mode=WAL")
        self._fts = self._detect_fts5()
        self._create_schema()

    def _detect_fts5(self) -> bool:
        try:
            self._db.execute("CREATE VIRTUAL TABLE _fts_probe USING fts5(x)")
            self._db.execute("DROP TABLE _fts_probe")
            return True
        except sqlite3.OperationalError:
            return False

    def _create_schema(self) -> None:
        c = self._db
        c.executescript(
            """
            CREATE TABLE IF NOT EXISTS projects (
                name TEXT PRIMARY KEY,
                created_at TEXT
            );
            CREATE TABLE IF NOT EXISTS pages (
                project TEXT, page_id TEXT, type TEXT, title TEXT,
                frontmatter TEXT, body TEXT, content_sha256 TEXT,
                PRIMARY KEY (project, page_id)
            );
            CREATE TABLE IF NOT EXISTS embeddings (
                project TEXT, page_id TEXT, dim INTEGER, vec BLOB,
                PRIMARY KEY (project, page_id)
            );
            CREATE TABLE IF NOT EXISTS oplog (
                project TEXT, seq INTEGER, ts TEXT, op TEXT,
                page_id TEXT, details TEXT,
                PRIMARY KEY (project, seq)
            );
            CREATE TABLE IF NOT EXISTS sources (
                project TEXT, id TEXT, title TEXT, sha TEXT, path TEXT,
                ingested_at TEXT,
                PRIMARY KEY (project, id)
            );
            CREATE TABLE IF NOT EXISTS contradictions (
                project TEXT, id TEXT, entity TEXT,
                page_a TEXT, claim_a TEXT, page_b TEXT, claim_b TEXT,
                detected_at TEXT,
                PRIMARY KEY (project, id)
            );
            """
        )
        if self._fts:
            c.execute(
                "CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts "
                "USING fts5(page_id UNINDEXED, project UNINDEXED, title, body)"
            )
        c.commit()

    def init_project(self, project: str) -> None:
        self._db.execute(
            "INSERT OR IGNORE INTO projects(name, created_at) VALUES (?, ?)",
            (project, _now()),
        )
        self._db.commit()

    def list_projects(self) -> list[str]:
        rows = self._db.execute("SELECT name FROM projects ORDER BY name").fetchall()
        return [r["name"] for r in rows]

    def delete_project(self, project: str) -> None:
        for tbl in ("pages", "embeddings", "oplog", "sources", "contradictions"):
            self._db.execute(f"DELETE FROM {tbl} WHERE project = ?", (project,))
        if self._fts:
            self._db.execute("DELETE FROM pages_fts WHERE project = ?", (project,))
        self._db.execute("DELETE FROM projects WHERE name = ?", (project,))
        self._db.commit()

    def upsert_page(self, project: str, page: WikiPage, embedding: list[float]) -> None:
        fm = page.frontmatter
        self._db.execute(
            "INSERT OR REPLACE INTO pages"
            "(project, page_id, type, title, frontmatter, body, content_sha256)"
            " VALUES (?,?,?,?,?,?,?)",
            (
                project, fm.id, fm.type, fm.title,
                json.dumps(fm.to_dict()), page.body, fm.content_sha256,
            ),
        )
        self._db.execute(
            "INSERT OR REPLACE INTO embeddings(project, page_id, dim, vec) VALUES (?,?,?,?)",
            (project, fm.id, len(embedding), _pack(embedding)),
        )
        if self._fts:
            self._db.execute(
                "DELETE FROM pages_fts WHERE project = ? AND page_id = ?", (project, fm.id)
            )
            self._db.execute(
                "INSERT INTO pages_fts(page_id, project, title, body) VALUES (?,?,?,?)",
                (fm.id, project, fm.title, page.body),
            )
        self._db.commit()

    def _row_to_page(self, row: sqlite3.Row) -> WikiPage:
        fm = Frontmatter.from_dict(json.loads(row["frontmatter"]))
        return WikiPage(frontmatter=fm, body=row["body"])

    def get_page(self, project: str, page_id: str) -> WikiPage | None:
        row = self._db.execute(
            "SELECT * FROM pages WHERE project = ? AND page_id = ?", (project, page_id)
        ).fetchone()
        return self._row_to_page(row) if row else None

    def all_pages(self, project: str) -> list[WikiPage]:
        rows = self._db.execute(
            "SELECT * FROM pages WHERE project = ? ORDER BY page_id", (project,)
        ).fetchall()
        return [self._row_to_page(r) for r in rows]

    def delete_page(self, project: str, page_id: str) -> None:
        self._db.execute(
            "DELETE FROM pages WHERE project = ? AND page_id = ?", (project, page_id)
        )
        self._db.execute(
            "DELETE FROM embeddings WHERE project = ? AND page_id = ?", (project, page_id)
        )
        if self._fts:
            self._db.execute(
                "DELETE FROM pages_fts WHERE project = ? AND page_id = ?", (project, page_id)
            )
        self._db.commit()

    def vector_search(self, project: str, query_vec: list[float], k: int) -> list[Hit]:
        rows = self._db.execute(
            "SELECT page_id, dim, vec FROM embeddings WHERE project = ?", (project,)
        ).fetchall()
        if not rows:
            return []
        q = np.asarray(query_vec, dtype="<f4")
        qn = float(np.linalg.norm(q))
        hits: list[Hit] = []
        for r in rows:
            if r["dim"] != len(query_vec):
                raise EmbeddingDimMismatch(
                    f"query dim {len(query_vec)} != stored dim {r['dim']} "
                    f"for page {r['page_id']!r}"
                )
            v = _unpack(r["vec"])
            denom = qn * float(np.linalg.norm(v))
            score = float(np.dot(q, v) / denom) if denom > 0 else 0.0
            hits.append(Hit(page_id=r["page_id"], score=score, via="vector"))
        hits.sort(key=lambda h: (-h.score, h.page_id))
        return hits[:k]

    def fulltext_search(self, project: str, query: str, k: int) -> list[Hit]:
        toks = [t for t in tokenize(query) if len(t) > 1]
        if not toks:
            return []
        if self._fts:
            match = " OR ".join(f'"{t}"' for t in toks)
            try:
                rows = self._db.execute(
                    "SELECT page_id, bm25(pages_fts) AS rank FROM pages_fts "
                    "WHERE pages_fts MATCH ? AND project = ? "
                    "ORDER BY rank LIMIT ?",
                    (match, project, k),
                ).fetchall()
            except sqlite3.OperationalError:
                return []
            return [Hit(page_id=r["page_id"], score=-float(r["rank"]), via="fulltext") for r in rows]
        # No FTS5: rank by how many query tokens each page contains.
        rows = self._db.execute(
            "SELECT page_id, title, body FROM pages WHERE project = ?", (project,)
        ).fetchall()
        scored: list[Hit] = []
        qset = set(toks)
        for r in rows:
            doc = set(tokenize(f"{r['title']} {r['body']}"))
            overlap = len(qset & doc)
            if overlap:
                scored.append(Hit(page_id=r["page_id"], score=float(overlap), via="fulltext"))
        scored.sort(key=lambda h: (-h.score, h.page_id))
        return scored[:k]

    def hybrid_search(self, project: str, query: str, query_vec: list[float], k: int) -> list[Hit]:
        pool = max(k * 3, 10)
        vec_hits = self.vector_search(project, query_vec, pool)
        ft_hits = self.fulltext_search(project, query, pool)
        return reciprocal_rank_fusion([vec_hits, ft_hits], top=k)

    def append_log(self, project: str, op: Operation) -> Operation:
        row = self._db.execute(
            "SELECT COALESCE(MAX(seq), 0) AS m FROM oplog WHERE project = ?", (project,)
        ).fetchone()
        op.seq = int(row["m"]) + 1
        self._db.execute(
            "INSERT INTO oplog(project, seq, ts, op, page_id, details) VALUES (?,?,?,?,?,?)",
            (project, op.seq, op.ts, op.op, op.page_id, json.dumps(op.details)),
        )
        self._db.commit()
        return op

    def read_log(self, project: str, since: datetime | None = None) -> list[Operation]:
        rows = self._db.execute(
            "SELECT seq, ts, op, page_id, details FROM oplog WHERE project = ? ORDER BY seq",
            (project,),
        ).fetchall()
        ops = [
            Operation(
                op=r["op"], page_id=r["page_id"], details=json.loads(r["details"]),
                ts=r["ts"], seq=r["seq"],
            )
            for r in rows
        ]
        if since is not None:
            cutoff = since.isoformat()
            ops = [o for o in ops if o.ts >= cutoff]
        return ops

    def get_source_by_sha(self, project: str, sha: str) -> str | None:
        row = self._db.execute(
            "SELECT id FROM sources WHERE project = ? AND sha = ?", (project, sha)
        ).fetchone()
        return row["id"] if row else None

    def add_source(self, project: str, source_id: str, title: str, sha: str, path: str | None) -> None:
        self._db.execute(
            "INSERT OR REPLACE INTO sources(project, id, title, sha, path, ingested_at)"
            " VALUES (?,?,?,?,?,?)",
            (project, source_id, title, sha, path, _now()),
        )
        self._db.commit()

    def add_contradiction(self, project: str, c: Contradiction) -> None:
        self._db.execute(
            "INSERT OR REPLACE INTO contradictions"
            "(project, id, entity, page_a, claim_a, page_b, claim_b, detected_at)"
            " VALUES (?,?,?,?,?,?,?,?)",
            (project, c.id, c.entity, c.page_a, c.claim_a, c.page_b, c.claim_b, c.detected_at),
        )
        self._db.commit()

    def list_contradictions(self, project: str) -> list[Contradiction]:
        rows = self._db.execute(
            "SELECT * FROM contradictions WHERE project = ? ORDER BY id", (project,)
        ).fetchall()
        return [
            Contradiction(
                id=r["id"], entity=r["entity"], page_a=r["page_a"], claim_a=r["claim_a"],
                page_b=r["page_b"], claim_b=r["claim_b"], detected_at=r["detected_at"],
            )
            for r in rows
        ]

    def close(self) -> None:
        self._db.close()
