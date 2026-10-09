"""SQLite storage: repositories, files, symbols, graph edges, wiki, memory, feedback.

The embedded backend mirrors the Oracle schema in `oracle/schema.sql` table
for table, so the same ingestion code feeds either. Text search uses FTS5;
vectors are float32 BLOBs scanned with numpy (exact cosine, which is what an
HNSW index at TARGET ACCURACY 95 approximates).
"""

from __future__ import annotations

import json
import os
import sqlite3
import time
from pathlib import Path

SCHEMA_VERSION = 1

SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);

CREATE TABLE IF NOT EXISTS repos (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    path        TEXT NOT NULL,
    commit_sha  TEXT,
    embedder    TEXT,
    indexed_at  REAL,
    created_at  REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id          INTEGER PRIMARY KEY,
    repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    language    TEXT NOT NULL,
    content_sha TEXT NOT NULL,
    size        INTEGER NOT NULL,
    lines       INTEGER NOT NULL,
    parser      TEXT NOT NULL,
    module_doc  TEXT NOT NULL DEFAULT '',
    commit_sha  TEXT,
    indexed_at  REAL NOT NULL,
    UNIQUE (repo_id, path)
);

CREATE TABLE IF NOT EXISTS symbols (
    id           INTEGER PRIMARY KEY,
    repo_id      INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    file_id      INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    key          TEXT NOT NULL,           -- stable identity: path::qualname[#n]
    name         TEXT NOT NULL,
    qualname     TEXT NOT NULL,
    kind         TEXT NOT NULL,
    language     TEXT NOT NULL,
    start_line   INTEGER NOT NULL,
    end_line     INTEGER NOT NULL,
    signature    TEXT NOT NULL DEFAULT '',
    doc          TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    commit_sha   TEXT,
    embedding    BLOB,
    UNIQUE (repo_id, key)
);
CREATE INDEX IF NOT EXISTS symbols_name ON symbols(repo_id, name);
CREATE INDEX IF NOT EXISTS symbols_file ON symbols(file_id);

CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
    name, qualname, words, doc, source,
    tokenize = "unicode61 tokenchars '_'"
);

-- Relationships extracted at ingest (the property graph's edge tables).
CREATE TABLE IF NOT EXISTS calls (
    src_id   INTEGER NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    dst_name TEXT NOT NULL,
    repo_id  INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    PRIMARY KEY (src_id, dst_name)
);
CREATE INDEX IF NOT EXISTS calls_dst ON calls(repo_id, dst_name);

CREATE TABLE IF NOT EXISTS imports (
    file_id  INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    repo_id  INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    module   TEXT NOT NULL,
    PRIMARY KEY (file_id, module)
);

CREATE TABLE IF NOT EXISTS wiki_pages (
    id          INTEGER PRIMARY KEY,
    repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    module_path TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    commit_sha  TEXT,
    built_at    REAL NOT NULL,
    embedding   BLOB,
    UNIQUE (repo_id, slug)
);
CREATE VIRTUAL TABLE IF NOT EXISTS wiki_fts USING fts5(
    title, body, tokenize = "unicode61 tokenchars '_'"
);

-- Citations are rows, not prose: each points at a symbol by foreign key and
-- remembers the symbol's content hash at build time, so staleness is a join.
CREATE TABLE IF NOT EXISTS wiki_citations (
    page_id      INTEGER NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    symbol_id    INTEGER REFERENCES symbols(id) ON DELETE SET NULL,
    symbol_key   TEXT NOT NULL,
    role         TEXT NOT NULL,           -- key | entry | dependency
    content_hash TEXT NOT NULL,
    PRIMARY KEY (page_id, symbol_key, role)
);

-- Episodic memory: every tool invocation.
CREATE TABLE IF NOT EXISTS runs (
    id         INTEGER PRIMARY KEY,
    ts         REAL NOT NULL,
    tool       TEXT NOT NULL,
    repo       TEXT,
    query      TEXT,
    args       TEXT NOT NULL DEFAULT '{}',
    results    TEXT NOT NULL DEFAULT '[]',   -- symbol keys returned, in rank order
    result_count INTEGER NOT NULL DEFAULT 0,
    duration_ms REAL
);
CREATE INDEX IF NOT EXISTS runs_repo ON runs(repo, ts);

-- Semantic memory: durable facts distilled from runs.
CREATE TABLE IF NOT EXISTS facts (
    id          INTEGER PRIMARY KEY,
    repo        TEXT NOT NULL,
    subject     TEXT NOT NULL,
    predicate   TEXT NOT NULL,
    assertion   TEXT NOT NULL,
    confidence  REAL NOT NULL,
    source_runs TEXT NOT NULL DEFAULT '[]',
    symbol_key  TEXT,
    created_at  REAL NOT NULL,
    updated_at  REAL NOT NULL,
    UNIQUE (repo, subject, predicate, symbol_key)
);

-- Usage feedback (a result was cited / opened / used).
CREATE TABLE IF NOT EXISTS feedback (
    id          INTEGER PRIMARY KEY,
    ts          REAL NOT NULL,
    repo        TEXT NOT NULL,
    symbol_key  TEXT NOT NULL,
    kind        TEXT NOT NULL,
    run_id      INTEGER
);
CREATE INDEX IF NOT EXISTS feedback_key ON feedback(repo, symbol_key);
"""


def default_db_path() -> Path:
    home = os.environ.get("CCCTX_HOME") or os.path.join(Path.home(), ".ccctx")
    return Path(home) / "ccctx.db"


class Store:
    def __init__(self, path: str | Path | None = None):
        self.path = Path(path) if path else default_db_path()
        if str(self.path) != ":memory:":
            self.path.parent.mkdir(parents=True, exist_ok=True)
        self.db = sqlite3.connect(str(self.path))
        self.db.row_factory = sqlite3.Row
        self.db.execute("PRAGMA foreign_keys = ON")
        self.db.execute("PRAGMA journal_mode = WAL")
        self.db.execute("PRAGMA synchronous = NORMAL")
        self.db.executescript(SCHEMA)
        self.db.execute("INSERT OR IGNORE INTO meta VALUES ('schema_version', ?)", (str(SCHEMA_VERSION),))
        self.db.commit()

    def close(self):
        self.db.close()

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()

    def add_repo(self, name: str, path: str) -> int:
        cur = self.db.execute(
            "INSERT INTO repos (name, path, created_at) VALUES (?, ?, ?)", (name, path, time.time()))
        self.db.commit()
        return cur.lastrowid

    def remove_repo(self, name: str) -> bool:
        repo = self.repo(name)
        if not repo:
            return False
        ids = [r[0] for r in self.db.execute("SELECT id FROM symbols WHERE repo_id = ?", (repo["id"],))]
        self._fts_delete(ids)
        pages = [r[0] for r in self.db.execute("SELECT id FROM wiki_pages WHERE repo_id = ?", (repo["id"],))]
        self.db.executemany("DELETE FROM wiki_fts WHERE rowid = ?", [(p,) for p in pages])
        self.db.execute("DELETE FROM repos WHERE id = ?", (repo["id"],))
        self.db.commit()
        return True

    def repo(self, name: str):
        return self.db.execute("SELECT * FROM repos WHERE name = ?", (name,)).fetchone()

    def repo_by_id(self, repo_id: int):
        return self.db.execute("SELECT * FROM repos WHERE id = ?", (repo_id,)).fetchone()

    def repos(self) -> list[dict]:
        rows = self.db.execute("""
            SELECT r.*,
                   (SELECT COUNT(*) FROM files f WHERE f.repo_id = r.id) AS file_count,
                   (SELECT COUNT(*) FROM symbols s WHERE s.repo_id = r.id) AS symbol_count
            FROM repos r ORDER BY r.name""").fetchall()
        return [dict(r) for r in rows]

    # The FTS tables are contentless and keyed by rowid = symbol id.

    def _fts_delete(self, ids):
        self.db.executemany("DELETE FROM symbols_fts WHERE rowid = ?", [(i,) for i in ids])

    def fts_put(self, symbol_id: int, name: str, qualname: str, words: str, doc: str, source: str):
        self.db.execute("DELETE FROM symbols_fts WHERE rowid = ?", (symbol_id,))
        self.db.execute(
            "INSERT INTO symbols_fts (rowid, name, qualname, words, doc, source) VALUES (?, ?, ?, ?, ?, ?)",
            (symbol_id, name, qualname, words, doc, source))

    def q(self, sql: str, params=()) -> list[sqlite3.Row]:
        return self.db.execute(sql, params).fetchall()

    def q1(self, sql: str, params=()):
        return self.db.execute(sql, params).fetchone()

    def commit(self):
        self.db.commit()


def dumps(obj) -> str:
    return json.dumps(obj, separators=(",", ":"))
